package sub

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	yaml "github.com/goccy/go-yaml"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/singbox"
)

func TestTUICMihomoE2E(t *testing.T) {
	singboxBinary := os.Getenv("SINGBOX_E2E_BINARY")
	mihomoBinary := os.Getenv("MIHOMO_E2E_BINARY")
	if singboxBinary == "" || mihomoBinary == "" {
		t.Skip("SINGBOX_E2E_BINARY and MIHOMO_E2E_BINARY are required for the real TUIC E2E test")
	}

	tmp := t.TempDir()
	certPath, keyPath := writeTUICE2ECertificate(t, tmp)
	tuicPort := freeUDPPort(t)
	mixedPort := freeTCPPort(t)

	const (
		uuid     = "11111111-1111-4111-8111-111111111111"
		password = "3xpatcher-tuic-e2e-password"
		name     = "TUIC-E2E"
		// Deliberately not h3. V11.4 accidentally forced h3 in the Clash
		// generator, so an h3-only fixture could never detect that regression.
		// The real contract is that client ALPN follows the server TLS setting.
		alpn = "3xpatcher-tuic-e2e"
	)

	settingsBytes, err := json.Marshal(singbox.TUICSettings{
		Users: []singbox.TUICUser{{Name: "e2e", UUID: uuid, Password: password}},
		CongestionControl: "cubic",
		TLS: singbox.TLSSettings{
			Enabled:         true,
			ServerName:      "localhost",
			ALPN:            []string{alpn},
			CertificatePath: certPath,
			KeyPath:         keyPath,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	serverConfig, err := singbox.BuildConfig([]singbox.InboundRecord{{
		ID:       1,
		Remark:   name,
		Enable:   true,
		Listen:   "127.0.0.1",
		Port:     tuicPort,
		Protocol: singbox.ProtocolTUIC,
		Tag:      "tuic-e2e",
		Settings: settingsBytes,
	}})
	if err != nil {
		t.Fatalf("BuildConfig TUIC server: %v", err)
	}
	serverConfigPath := filepath.Join(tmp, "sing-box.json")
	if err := os.WriteFile(serverConfigPath, serverConfig, 0o600); err != nil {
		t.Fatal(err)
	}

	singboxCmd, singboxLogs := startE2EProcess(t, singboxBinary, "run", "-c", serverConfigPath)
	waitE2EProcessAlive(t, "sing-box", singboxCmd, singboxLogs, 700*time.Millisecond)

	inbound := &model.Inbound{
		Id:       1,
		Remark:   name,
		Enable:   true,
		Listen:   "127.0.0.1",
		Port:     tuicPort,
		Protocol: model.TUIC,
		Settings: `{"congestionControl":"cubic","zeroRTTHandshake":false}`,
	}
	client := model.Client{ID: uuid, Password: password, Email: "e2e", Enable: true}
	stream := map[string]any{
		"security": "tls",
		"tlsSettings": map[string]any{
			"serverName":      "localhost",
			"alpn":            []any{alpn},
			"certificateMode": "self_signed_sni",
		},
	}
	ep := map[string]any{"remarkFinal": true, "remark": name}
	proxy := (&SubClashService{}).buildSingboxProxy(&SubService{}, inbound, client, stream, ep)
	if proxy == nil {
		t.Fatal("TUIC Clash generator returned nil")
	}
	if proxy["type"] != "tuic" || proxy["uuid"] != uuid || proxy["password"] != password {
		t.Fatalf("unexpected generated TUIC proxy: %#v", proxy)
	}
	generatedALPN, ok := proxy["alpn"].([]string)
	if !ok || len(generatedALPN) != 1 || generatedALPN[0] != alpn {
		t.Fatalf("generated TUIC proxy did not preserve server ALPN %q: %#v", alpn, proxy["alpn"])
	}
	for _, forbidden := range []string{"heartbeat-interval", "udp-relay-mode", "max-open-streams", "disable-mtu-discovery", "disable-sni"} {
		if _, exists := proxy[forbidden]; exists {
			t.Fatalf("generated TUIC proxy regressed forbidden field %q: %#v", forbidden, proxy)
		}
	}

	mihomoConfig := map[string]any{
		"mixed-port": mixedPort,
		"allow-lan":  false,
		"mode":       "rule",
		"log-level":  "debug",
		"ipv6":       false,
		"proxies":    []map[string]any{proxy},
		"rules":      []string{"MATCH," + name},
	}
	mihomoYAML, err := yaml.Marshal(mihomoConfig)
	if err != nil {
		t.Fatalf("marshal Mihomo config: %v", err)
	}
	mihomoConfigPath := filepath.Join(tmp, "mihomo.yaml")
	if err := os.WriteFile(mihomoConfigPath, mihomoYAML, 0o600); err != nil {
		t.Fatal(err)
	}

	mihomoHome := filepath.Join(tmp, "mihomo-home")
	if err := os.MkdirAll(mihomoHome, 0o700); err != nil {
		t.Fatal(err)
	}
	mihomoCmd, mihomoLogs := startE2EProcess(t, mihomoBinary, "-d", mihomoHome, "-f", mihomoConfigPath)
	waitTCPReady(t, mihomoCmd, mihomoLogs, fmt.Sprintf("127.0.0.1:%d", mixedPort), 10*time.Second)

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "tuic-e2e-ok")
	}))
	defer target.Close()

	proxyURL, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", mixedPort))
	if err != nil {
		t.Fatal(err)
	}
	httpClient := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
		Timeout:   3 * time.Second,
	}

	// Mihomo can accept on mixed-port a few milliseconds before the first QUIC
	// dial is ready. Retry only the startup window; once established, require
	// consecutive requests to succeed so a permanently broken TUIC path cannot
	// be hidden by the retry loop.
	deadline := time.Now().Add(8 * time.Second)
	var lastErr error
	var lastStatus, lastBody string
	established := false
	for time.Now().Before(deadline) {
		resp, reqErr := httpClient.Get(target.URL)
		if reqErr != nil {
			lastErr = reqErr
			time.Sleep(200 * time.Millisecond)
			continue
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		lastStatus = resp.Status
		lastBody = string(body)
		if readErr == nil && resp.StatusCode == http.StatusOK && lastBody == "tuic-e2e-ok" {
			established = true
			break
		}
		if readErr != nil {
			lastErr = readErr
		} else {
			lastErr = fmt.Errorf("status=%s body=%q", lastStatus, lastBody)
		}
		time.Sleep(200 * time.Millisecond)
	}
	if !established {
		t.Fatalf("TUIC never established before deadline: %v\n--- generated Mihomo YAML ---\n%s\n--- sing-box ---\n%s\n--- mihomo ---\n%s", lastErr, mihomoYAML, singboxLogs.String(), mihomoLogs.String())
	}

	for i := 0; i < 2; i++ {
		resp, reqErr := httpClient.Get(target.URL)
		if reqErr != nil {
			t.Fatalf("established TUIC request %d failed: %v\n--- generated Mihomo YAML ---\n%s\n--- sing-box ---\n%s\n--- mihomo ---\n%s", i+1, reqErr, mihomoYAML, singboxLogs.String(), mihomoLogs.String())
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil || resp.StatusCode != http.StatusOK || string(body) != "tuic-e2e-ok" {
			t.Fatalf("established TUIC request %d bad response: status=%s body=%q readErr=%v\n--- sing-box ---\n%s\n--- mihomo ---\n%s", i+1, resp.Status, body, readErr, singboxLogs.String(), mihomoLogs.String())
		}
	}
}

func writeTUICE2ECertificate(t *testing.T, dir string) (string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "localhost"},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPath := filepath.Join(dir, "tuic.crt")
	keyPath := filepath.Join(dir, "tuic.key")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	return certPath, keyPath
}

func freeUDPPort(t *testing.T) int {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	port := conn.LocalAddr().(*net.UDPAddr).Port
	_ = conn.Close()
	return port
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

func startE2EProcess(t *testing.T, binary string, args ...string) (*exec.Cmd, *bytes.Buffer) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	logs := &bytes.Buffer{}
	cmd.Stdout = logs
	cmd.Stderr = logs
	if err := cmd.Start(); err != nil {
		t.Fatalf("start %s: %v", binary, err)
	}
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
	})
	return cmd, logs
}

func waitE2EProcessAlive(t *testing.T, name string, cmd *exec.Cmd, logs *bytes.Buffer, delay time.Duration) {
	t.Helper()
	time.Sleep(delay)
	if cmd.Process == nil || cmd.Process.Signal(syscall.Signal(0)) != nil {
		t.Fatalf("%s exited during startup\n%s", name, logs.String())
	}
}

func waitTCPReady(t *testing.T, cmd *exec.Cmd, logs *bytes.Buffer, address string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cmd.Process == nil || cmd.Process.Signal(syscall.Signal(0)) != nil {
			t.Fatalf("process exited before %s became ready\n%s", address, logs.String())
		}
		conn, err := net.DialTimeout("tcp", address, 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s\n%s", address, logs.String())
}
