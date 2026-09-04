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
	)
	// Mirror the user's known-good Clash Verge TUIC profile. The ordering is
	// part of the TLS contract and must survive 3Xpatcher generation unchanged.
	alpn := []string{"h3", "h2", "http/1.1"}

	settingsBytes, err := json.Marshal(singbox.TUICSettings{
		Users: []singbox.TUICUser{{Name: "e2e", UUID: uuid, Password: password}},
		CongestionControl: "bbr",
		TLS: singbox.TLSSettings{
			Enabled:         true,
			ServerName:      "localhost",
			ALPN:            alpn,
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
		Settings: `{"congestionControl":"bbr","zeroRTTHandshake":false}`,
	}
	client := model.Client{ID: uuid, Password: password, Email: "e2e", Enable: true}
	stream := map[string]any{
		"security": "tls",
		"tlsSettings": map[string]any{
			"serverName":      "localhost",
			"alpn":            []any{"h3", "h2", "http/1.1"},
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
	if proxy["udp-relay-mode"] != "native" {
		t.Fatalf("generated TUIC proxy must explicitly use native UDP relay: %#v", proxy)
	}
	if _, exists := proxy["udp"]; exists {
		t.Fatalf("generated TUIC proxy should rely on Mihomo's TUIC UDP default, not generic udp: %#v", proxy)
	}
	generatedALPN, ok := proxy["alpn"].([]string)
	if !ok || len(generatedALPN) != len(alpn) {
		t.Fatalf("generated TUIC proxy did not preserve ALPN list: %#v", proxy["alpn"])
	}
	for i := range alpn {
		if generatedALPN[i] != alpn[i] {
			t.Fatalf("generated TUIC proxy changed ALPN order: got %#v want %#v", generatedALPN, alpn)
		}
	}
	for _, forbidden := range []string{"heartbeat-interval", "max-open-streams", "disable-mtu-discovery", "disable-sni"} {
		if _, exists := proxy[forbidden]; exists {
			t.Fatalf("generated TUIC proxy regressed unrelated field %q: %#v", forbidden, proxy)
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
	// dial is ready. Retry only this startup window; once established, require
	// consecutive TCP-over-TUIC requests to succeed.
	deadline := time.Now().Add(8 * time.Second)
	var lastErr error
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
		if readErr == nil && resp.StatusCode == http.StatusOK && string(body) == "tuic-e2e-ok" {
			established = true
			break
		}
		if readErr != nil {
			lastErr = readErr
		} else {
			lastErr = fmt.Errorf("status=%s body=%q", resp.Status, body)
		}
		time.Sleep(200 * time.Millisecond)
	}
	if !established {
		t.Fatalf("TUIC TCP path never established: %v\n--- generated Mihomo YAML ---\n%s\n--- sing-box ---\n%s\n--- mihomo ---\n%s", lastErr, mihomoYAML, singboxLogs.String(), mihomoLogs.String())
	}

	for i := 0; i < 2; i++ {
		resp, reqErr := httpClient.Get(target.URL)
		if reqErr != nil {
			t.Fatalf("established TUIC TCP request %d failed: %v\n--- generated Mihomo YAML ---\n%s\n--- sing-box ---\n%s\n--- mihomo ---\n%s", i+1, reqErr, mihomoYAML, singboxLogs.String(), mihomoLogs.String())
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil || resp.StatusCode != http.StatusOK || string(body) != "tuic-e2e-ok" {
			t.Fatalf("established TUIC TCP request %d bad response: status=%s body=%q readErr=%v\n--- sing-box ---\n%s\n--- mihomo ---\n%s", i+1, resp.Status, body, readErr, singboxLogs.String(), mihomoLogs.String())
		}
	}

	// Exercise the field that motivated V11.6: SOCKS5 UDP ASSOCIATE enters
	// Mihomo's mixed-port, is relayed using TUIC udp-relay-mode=native, exits
	// sing-box, reaches a real UDP echo socket, and returns through the tunnel.
	udpTarget := startUDPEchoServer(t)
	for i := 0; i < 2; i++ {
		payload := []byte(fmt.Sprintf("tuic-udp-e2e-%d", i+1))
		if err := socks5UDPExchange(mixedPort, udpTarget, payload); err != nil {
			t.Fatalf("TUIC native UDP relay %d failed: %v\n--- generated Mihomo YAML ---\n%s\n--- sing-box ---\n%s\n--- mihomo ---\n%s", i+1, err, mihomoYAML, singboxLogs.String(), mihomoLogs.String())
		}
	}
}

func startUDPEchoServer(t *testing.T) *net.UDPAddr {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	go func() {
		buf := make([]byte, 64*1024)
		for {
			n, addr, readErr := conn.ReadFromUDP(buf)
			if readErr != nil {
				return
			}
			_, _ = conn.WriteToUDP(buf[:n], addr)
		}
	}()
	return conn.LocalAddr().(*net.UDPAddr)
}

func socks5UDPExchange(mixedPort int, target *net.UDPAddr, payload []byte) error {
	control, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", mixedPort), 2*time.Second)
	if err != nil {
		return fmt.Errorf("SOCKS5 control dial: %w", err)
	}
	defer control.Close()
	_ = control.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := control.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		return fmt.Errorf("SOCKS5 greeting write: %w", err)
	}
	greeting := make([]byte, 2)
	if _, err := io.ReadFull(control, greeting); err != nil {
		return fmt.Errorf("SOCKS5 greeting read: %w", err)
	}
	if !bytes.Equal(greeting, []byte{0x05, 0x00}) {
		return fmt.Errorf("SOCKS5 greeting response: %v", greeting)
	}
	// UDP ASSOCIATE with 0.0.0.0:0 means the client asks Mihomo to allocate a
	// UDP relay endpoint while this TCP control connection remains open.
	if _, err := control.Write([]byte{0x05, 0x03, 0x00, 0x01, 0, 0, 0, 0, 0, 0}); err != nil {
		return fmt.Errorf("SOCKS5 UDP associate write: %w", err)
	}
	relay, err := readSocks5ReplyAddr(control)
	if err != nil {
		return err
	}
	if relay.IP == nil || relay.IP.IsUnspecified() {
		relay.IP = net.ParseIP("127.0.0.1")
	}
	udpConn, err := net.DialUDP("udp", nil, relay)
	if err != nil {
		return fmt.Errorf("SOCKS5 UDP relay dial %s: %w", relay, err)
	}
	defer udpConn.Close()
	_ = udpConn.SetDeadline(time.Now().Add(5 * time.Second))

	targetIP := target.IP.To4()
	if targetIP == nil {
		return fmt.Errorf("test target is not IPv4: %s", target)
	}
	packet := make([]byte, 0, 10+len(payload))
	packet = append(packet, 0x00, 0x00, 0x00, 0x01)
	packet = append(packet, targetIP...)
	packet = append(packet, byte(target.Port>>8), byte(target.Port))
	packet = append(packet, payload...)
	if _, err := udpConn.Write(packet); err != nil {
		return fmt.Errorf("SOCKS5 UDP write: %w", err)
	}
	buf := make([]byte, 64*1024)
	n, err := udpConn.Read(buf)
	if err != nil {
		return fmt.Errorf("SOCKS5 UDP read: %w", err)
	}
	got, err := socks5UDPPayload(buf[:n])
	if err != nil {
		return err
	}
	if !bytes.Equal(got, payload) {
		return fmt.Errorf("UDP echo mismatch: got %q want %q", got, payload)
	}
	return nil
}

func readSocks5ReplyAddr(r io.Reader) (*net.UDPAddr, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, fmt.Errorf("SOCKS5 UDP associate reply header: %w", err)
	}
	if header[0] != 0x05 || header[1] != 0x00 {
		return nil, fmt.Errorf("SOCKS5 UDP associate rejected: %v", header)
	}
	var ip net.IP
	switch header[3] {
	case 0x01:
		raw := make([]byte, 4)
		if _, err := io.ReadFull(r, raw); err != nil {
			return nil, err
		}
		ip = net.IP(raw)
	case 0x04:
		raw := make([]byte, 16)
		if _, err := io.ReadFull(r, raw); err != nil {
			return nil, err
		}
		ip = net.IP(raw)
	case 0x03:
		length := []byte{0}
		if _, err := io.ReadFull(r, length); err != nil {
			return nil, err
		}
		host := make([]byte, int(length[0]))
		if _, err := io.ReadFull(r, host); err != nil {
			return nil, err
		}
		resolved, err := net.ResolveIPAddr("ip", string(host))
		if err != nil {
			return nil, err
		}
		ip = resolved.IP
	default:
		return nil, fmt.Errorf("SOCKS5 unsupported address type: %d", header[3])
	}
	portBytes := make([]byte, 2)
	if _, err := io.ReadFull(r, portBytes); err != nil {
		return nil, err
	}
	port := int(portBytes[0])<<8 | int(portBytes[1])
	return &net.UDPAddr{IP: ip, Port: port}, nil
}

func socks5UDPPayload(packet []byte) ([]byte, error) {
	if len(packet) < 4 || packet[0] != 0 || packet[1] != 0 || packet[2] != 0 {
		return nil, fmt.Errorf("invalid SOCKS5 UDP header: %v", packet)
	}
	offset := 4
	switch packet[3] {
	case 0x01:
		offset += 4
	case 0x04:
		offset += 16
	case 0x03:
		if len(packet) <= offset {
			return nil, fmt.Errorf("truncated SOCKS5 UDP domain header")
		}
		offset += 1 + int(packet[offset])
	default:
		return nil, fmt.Errorf("unsupported SOCKS5 UDP address type: %d", packet[3])
	}
	offset += 2
	if len(packet) < offset {
		return nil, fmt.Errorf("truncated SOCKS5 UDP packet")
	}
	return packet[offset:], nil
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
