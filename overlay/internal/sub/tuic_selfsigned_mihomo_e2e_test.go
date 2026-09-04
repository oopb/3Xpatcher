package sub

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	yaml "github.com/goccy/go-yaml"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/singbox"
)

// TestTUICPersistedSelfSignedMihomoE2E reproduces the real upgraded-row shape
// that exposed V11.6's blind spot: the generated self-signed certificate
// metadata is persisted, but the UI-only certificateMode marker is absent.
// The generated Mihomo proxy must still disable public-CA verification, or the
// actual TLS handshake against this self-signed TUIC server will fail.
func TestTUICPersistedSelfSignedMihomoE2E(t *testing.T) {
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
		uuid     = "22222222-2222-4222-8222-222222222222"
		password = "3xpatcher-tuic-selfsigned-e2e"
		name     = "TUIC-SELF-SIGNED-E2E"
	)
	alpn := []string{"h3", "h2", "http/1.1"}

	settingsBytes, err := json.Marshal(singbox.TUICSettings{
		Users:             []singbox.TUICUser{{Name: "e2e", UUID: uuid, Password: password}},
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
		ID: 2, Remark: name, Enable: true, Listen: "127.0.0.1", Port: tuicPort,
		Protocol: singbox.ProtocolTUIC, Tag: "tuic-selfsigned-e2e", Settings: settingsBytes,
	}})
	if err != nil {
		t.Fatalf("BuildConfig TUIC self-signed server: %v", err)
	}
	serverConfigPath := filepath.Join(tmp, "sing-box.json")
	if err := os.WriteFile(serverConfigPath, serverConfig, 0o600); err != nil {
		t.Fatal(err)
	}
	singboxCmd, singboxLogs := startE2EProcess(t, singboxBinary, "run", "-c", serverConfigPath)
	waitE2EProcessAlive(t, "sing-box", singboxCmd, singboxLogs, 700*time.Millisecond)

	inbound := &model.Inbound{
		Id: 2, Remark: name, Enable: true, Listen: "127.0.0.1", Port: tuicPort,
		Protocol: model.TUIC, Settings: `{"congestionControl":"bbr","zeroRTTHandshake":false}`,
	}
	client := model.Client{ID: uuid, Password: password, Email: "e2e", Enable: true}
	stream := map[string]any{
		"security": "tls",
		"tlsSettings": map[string]any{
			"serverName":                "localhost",
			"alpn":                      []any{"h3", "h2", "http/1.1"},
			"selfSignedServerName":      "localhost",
			"selfSignedCertificatePath": certPath,
			"selfSignedKeyPath":         keyPath,
			// Deliberately NO certificateMode: this is the regression fixture.
		},
	}
	proxy := (&SubClashService{}).buildSingboxProxy(
		&SubService{}, inbound, client, stream,
		map[string]any{"remarkFinal": true, "remark": name},
	)
	if proxy == nil {
		t.Fatal("TUIC Clash generator returned nil")
	}
	if proxy["skip-cert-verify"] != true {
		t.Fatalf("persisted generated self-signed TLS must produce skip-cert-verify=true: %#v", proxy)
	}
	if proxy["udp-relay-mode"] != "native" {
		t.Fatalf("TUIC must preserve native UDP relay mode: %#v", proxy)
	}
	generatedALPN, ok := proxy["alpn"].([]string)
	if !ok || len(generatedALPN) != len(alpn) {
		t.Fatalf("generated TUIC ALPN mismatch: %#v", proxy["alpn"])
	}
	for i := range alpn {
		if generatedALPN[i] != alpn[i] {
			t.Fatalf("generated TUIC changed ALPN order: got %#v want %#v", generatedALPN, alpn)
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
		t.Fatal(err)
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
		_, _ = io.WriteString(w, "tuic-selfsigned-e2e-ok")
	}))
	defer target.Close()
	proxyURL, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", mixedPort))
	clientHTTP := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}, Timeout: 3 * time.Second}

	deadline := time.Now().Add(8 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, reqErr := clientHTTP.Get(target.URL)
		if reqErr == nil {
			body, readErr := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if readErr == nil && resp.StatusCode == http.StatusOK && string(body) == "tuic-selfsigned-e2e-ok" {
				return
			}
			lastErr = fmt.Errorf("status=%s body=%q readErr=%v", resp.Status, body, readErr)
		} else {
			lastErr = reqErr
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("persisted self-signed TUIC never established: %v\n--- generated Mihomo YAML ---\n%s\n--- sing-box ---\n%s\n--- mihomo ---\n%s", lastErr, mihomoYAML, singboxLogs.String(), mihomoLogs.String())
}
