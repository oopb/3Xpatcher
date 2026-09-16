package mieru

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const mieruE2EUser = "mita"

func TestMieruMihomoE2E(t *testing.T) {
	mita := os.Getenv("MIERU_E2E_BINARY")
	mihomo := os.Getenv("MIHOMO_E2E_BINARY")
	if mita == "" || mihomo == "" {
		t.Skip("MIERU_E2E_BINARY and MIHOMO_E2E_BINARY are required")
	}

	const username = "mieru-e2e@example.com"
	const password = "3xpatcher-mieru-e2e-secret"
	serverPort := freeTCPPort(t)
	mixedPort := freeTCPPort(t)

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("3xpatcher-mieru-e2e-ok"))
	}))
	defer target.Close()

	cfg, err := BuildServerConfig(Record{
		ID:     991,
		Remark: "mieru-e2e",
		Port:   serverPort,
		Settings: Settings{
			Transport:       "TCP",
			MTU:             1400,
			LoggingLevel:    "INFO",
			AllowPrivateIP:  true,
			AllowLoopbackIP: true,
		},
		Users: []User{{Name: username, Password: password}},
	})
	if err != nil {
		t.Fatalf("BuildServerConfig: %v", err)
	}

	dir := t.TempDir()
	// Go's TempDir is 0700 by default. The production service runs as the
	// dedicated mita account, so make the test root traversable while keeping
	// the config itself read-only to non-owner users.
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	serverConfig := filepath.Join(dir, "mita.json")
	if err := os.WriteFile(serverConfig, cfg, 0o644); err != nil {
		t.Fatal(err)
	}

	runtimeDir := filepath.Join(dir, "run")
	if err := os.MkdirAll(runtimeDir, 0o775); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("sudo", "chown", mieruE2EUser+":"+mieruE2EUser, runtimeDir).CombinedOutput(); err != nil {
		t.Fatalf("prepare Mieru runtime dir: %v: %s", err, strings.TrimSpace(string(out)))
	}
	uds := filepath.Join(runtimeDir, "mita.sock")

	mitaLog := new(bytes.Buffer)
	mitaCmd := mitaExec(mita, serverConfig, uds, "run")
	mitaCmd.Stdout = mitaLog
	mitaCmd.Stderr = mitaLog
	if err := mitaCmd.Start(); err != nil {
		t.Fatalf("start mita: %v", err)
	}
	defer func() {
		_, _ = mitaCommand(mita, serverConfig, uds, "stop")
		_ = mitaCmd.Wait()
	}()

	deadline := time.Now().Add(15 * time.Second)
	for {
		out, statusErr := mitaCommand(mita, serverConfig, uds, "status")
		if statusErr == nil && strings.Contains(out, `mita server status is "RUNNING"`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("mita never became RUNNING: %v\nstatus=%s\nlog=%s", statusErr, out, mitaLog.String())
		}
		time.Sleep(250 * time.Millisecond)
	}

	resetMieruStatsState()
	beforeRaw, err := mitaCommand(mita, serverConfig, uds, "get", "metrics")
	if err != nil {
		t.Fatalf("initial mita metrics: %v\n%s", err, beforeRaw)
	}
	before, err := parseMetricsJSON([]byte(beforeRaw))
	if err != nil {
		t.Fatalf("parse initial mita metrics: %v\n%s", err, beforeRaw)
	}
	if delta := mieruMetricDelta(991, before); len(delta) != 0 {
		t.Fatalf("initial metrics must establish a zero delta baseline, got %#v", delta)
	}

	mihomoConfig := filepath.Join(dir, "mihomo.yaml")
	configText := fmt.Sprintf(`mixed-port: %d
allow-lan: false
mode: rule
log-level: debug
proxies:
  - name: mieru-e2e
    type: mieru
    server: 127.0.0.1
    port: %d
    transport: TCP
    username: %q
    password: %q
    multiplexing: MULTIPLEXING_LOW
    handshake-mode: HANDSHAKE_STANDARD
rules:
  - MATCH,mieru-e2e
`, mixedPort, serverPort, username, password)
	if err := os.WriteFile(mihomoConfig, []byte(configText), 0o600); err != nil {
		t.Fatal(err)
	}
	mihomoHome := filepath.Join(dir, "mihomo")
	if err := os.MkdirAll(mihomoHome, 0o700); err != nil {
		t.Fatal(err)
	}

	mihomoLog := new(bytes.Buffer)
	mihomoCmd := exec.Command(mihomo, "-d", mihomoHome, "-f", mihomoConfig)
	mihomoCmd.Stdout = mihomoLog
	mihomoCmd.Stderr = mihomoLog
	if err := mihomoCmd.Start(); err != nil {
		t.Fatalf("start Mihomo: %v", err)
	}
	defer stopProcess(mihomoCmd)
	waitTCP(t, fmt.Sprintf("127.0.0.1:%d", mixedPort), 15*time.Second, mihomoLog)

	proxy := fmt.Sprintf("http://127.0.0.1:%d", mixedPort)
	var response string
	for i := 0; i < 8; i++ {
		out, curlErr := exec.Command("curl", "-fsS", "--max-time", "8", "--proxy", proxy, target.URL).CombinedOutput()
		if curlErr == nil && strings.Contains(string(out), "3xpatcher-mieru-e2e-ok") {
			response = string(out)
			break
		}
		if i == 7 {
			t.Fatalf("real Mieru request through Mihomo failed: %v\nresponse=%s\nmita=%s\nmihomo=%s", curlErr, string(out), mitaLog.String(), mihomoLog.String())
		}
		time.Sleep(500 * time.Millisecond)
	}
	if !strings.Contains(response, "3xpatcher-mieru-e2e-ok") {
		t.Fatalf("unexpected proxy response %q", response)
	}

	afterRaw, err := mitaCommand(mita, serverConfig, uds, "get", "metrics")
	if err != nil {
		t.Fatalf("mita metrics after traffic: %v\n%s", err, afterRaw)
	}
	after, err := parseMetricsJSON([]byte(afterRaw))
	if err != nil {
		t.Fatalf("parse mita metrics after traffic: %v\n%s", err, afterRaw)
	}
	delta := mieruMetricDelta(991, after)
	if delta["traffic\x00UploadBytes"] <= 0 && delta["traffic\x00DownloadBytes"] <= 0 {
		t.Fatalf("Mieru inbound traffic did not increase: delta=%#v metrics=%s", delta, afterRaw)
	}
	if delta["user - "+username+"\x00UploadBytes"] <= 0 && delta["user - "+username+"\x00DownloadBytes"] <= 0 {
		t.Fatalf("Mieru per-user traffic did not increase: delta=%#v metrics=%s", delta, afterRaw)
	}
}

func mitaExec(binary, config, uds string, args ...string) *exec.Cmd {
	envArgs := []string{
		"-u", mieruE2EUser, "--", "env",
		"MITA_CONFIG_JSON_FILE=" + config,
		"MITA_UDS_PATH=" + uds,
		"MITA_LOG_NO_TIMESTAMP=true",
		binary,
	}
	envArgs = append(envArgs, args...)
	return exec.Command("sudo", envArgs...)
}

func mitaCommand(binary, config, uds string, args ...string) (string, error) {
	out, err := mitaExec(binary, config, uds, args...).CombinedOutput()
	return string(out), err
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

func waitTCP(t *testing.T, addr string, timeout time.Duration, log *bytes.Buffer) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 250*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("%s did not start listening within %s\n%s", addr, timeout, log.String())
}

func stopProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()
}
