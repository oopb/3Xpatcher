package mieru

import (
	"bytes"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func pint(v int) *int { return &v }

func TestBuildMieruTCPConfig(t *testing.T) {
	b, err := BuildServerConfig(Record{
		ID: 1, Port: 5000,
		Users: []User{{Name: "alice@example.com", Password: "secret"}},
		Settings: Settings{Transport: "TCP", MTU: 1400, AllowPrivateIP: true, UserHintIsMandatory: true},
	})
	if err != nil { t.Fatal(err) }
	s := string(b)
	for _, needle := range []string{`"port": 5000`, `"protocol": "TCP"`, `"name": "alice@example.com"`, `"hashedPassword"`, `"allowPrivateIP": true`, `"userHintIsMandatory": true`} {
		if !strings.Contains(s, needle) { t.Fatalf("missing %s:\n%s", needle, s) }
	}
	if strings.Contains(s, `"password": "secret"`) { t.Fatalf("plaintext password leaked into runtime config: %s", s) }
}

func TestBuildMieruPrivilegedPort(t *testing.T) {
	b, err := BuildServerConfig(Record{ID: 7, Port: 443, Users: []User{{Name: "u", Password: "p"}}})
	if err != nil { t.Fatal(err) }
	if !strings.Contains(string(b), `"port": 443`) { t.Fatalf("privileged port missing: %s", b) }
}

func TestBuildMieruPortRangeAndTrafficPattern(t *testing.T) {
	b, err := BuildServerConfig(Record{
		ID: 2, Port: 20000,
		Users: []User{{Name: "u", Password: "p"}},
		Settings: Settings{
			Transport: "UDP", PortRangeEnd: 20010, TrafficPatternEnabled: true,
			TrafficSeed: 42, TrafficUnlockAll: true, TCPFragmentEnable: true, TCPFragmentMaxSleepMs: 10,
			NonceType: "NONCE_TYPE_PRINTABLE", NonceMinLen: 6, NonceMaxLen: 8,
			PaddingMaxMiddleLen: pint(64), PaddingMaxEndLen: pint(128),
			LowEntropyMode: "LOW_ENTROPY_MODE_32", LowEntropyMaskRotation: "LOW_ENTROPY_MASK_ROTATE_RIGHT_7",
		},
	})
	if err != nil { t.Fatal(err) }
	s := string(b)
	for _, needle := range []string{`"portRange": "20000-20010"`, `"protocol": "UDP"`, `"trafficPattern"`, `"maxSleepMs": 10`, `"maxMiddlePaddingLen": 64`, `"LOW_ENTROPY_MODE_32"`} {
		if !strings.Contains(s, needle) { t.Fatalf("missing %s:\n%s", needle, s) }
	}
}

func TestMieruDefaultsPreserveOfficialImplicitTrafficPattern(t *testing.T) {
	b, err := BuildServerConfig(Record{ID: 3, Port: 5001, Users: []User{{Name: "u", Password: "p"}}})
	if err != nil { t.Fatal(err) }
	s := string(b)
	if strings.Contains(s, `"trafficPattern"`) { t.Fatalf("trafficPattern must be omitted unless enabled: %s", s) }
	if !strings.Contains(s, `"protocol": "TCP"`) || !strings.Contains(s, `"mtu": 1400`) { t.Fatalf("defaults missing: %s", s) }
}

func TestOfficialMitaAcceptsRenderedConfig(t *testing.T) {
	bin := os.Getenv("MIERU_OFFICIAL_BINARY")
	if bin == "" { t.Skip("MIERU_OFFICIAL_BINARY is not set") }
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil { t.Fatal(err) }
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	cfg, err := BuildServerConfig(Record{
		ID: 9, Port: port,
		Users: []User{{Name: "ci@example.com", Password: "ci-password"}},
		Settings: Settings{Transport: "TCP", MTU: 1400, TrafficPatternEnabled: true, TrafficSeed: 7, PaddingMaxMiddleLen: pint(32), PaddingMaxEndLen: pint(64)},
	})
	if err != nil { t.Fatal(err) }
	dir := t.TempDir(); cfgPath := filepath.Join(dir, "server.json"); uds := filepath.Join(dir, "mita.sock")
	if err := os.WriteFile(cfgPath, cfg, 0o600); err != nil { t.Fatal(err) }
	env := append(os.Environ(), "MITA_CONFIG_JSON_FILE="+cfgPath, "MITA_UDS_PATH="+uds, "MITA_LOG_NO_TIMESTAMP=true")
	cmd := exec.Command(bin, "run"); cmd.Env = env
	var log bytes.Buffer; cmd.Stdout = &log; cmd.Stderr = &log
	if err := cmd.Start(); err != nil { t.Fatal(err) }
	done := make(chan error, 1); go func() { done <- cmd.Wait() }()
	defer func() { if cmd.Process != nil { _ = cmd.Process.Kill() }; select { case <-done: case <-time.After(time.Second): } }()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-done:
			t.Fatalf("official mita exited before RUNNING: %v\n%s", err, log.String())
		default:
		}
		status := exec.Command(bin, "status"); status.Env = env; out, _ := status.CombinedOutput()
		if strings.Contains(string(out), `mita server status is "RUNNING"`) { return }
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("official mita did not accept rendered config:\n%s", log.String())
}

func TestRejectInvalidMieruRange(t *testing.T) {
	_, err := BuildServerConfig(Record{ID: 4, Port: 5000, Users: []User{{Name: "u", Password: "p"}}, Settings: Settings{PortRangeEnd: 4999}})
	if err == nil { t.Fatal("expected range error") }
}
func TestRejectIncompleteQuota(t *testing.T) {
	_, err := BuildServerConfig(Record{ID: 5, Port: 5000, Users: []User{{Name: "u", Password: "p"}}, Settings: Settings{QuotaDays: 1}})
	if err == nil { t.Fatal("expected quota error") }
}
