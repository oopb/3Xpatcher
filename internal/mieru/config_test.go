package mieru

import (
	"bytes"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func pint(v int) *int { return &v }

func TestBuildMieruTCPConfig(t *testing.T) {
	b, err := BuildServerConfig(Record{
		ID: 1, Port: 5000,
		Users:    []User{{Name: "alice@example.com", Password: "secret"}},
		Settings: Settings{Transport: "TCP", MTU: 1400, AllowPrivateIP: true, UserHintIsMandatory: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, needle := range []string{`"port": 5000`, `"protocol": "TCP"`, `"name": "alice@example.com"`, `"hashedPassword"`, `"allowPrivateIP": true`, `"userHintIsMandatory": true`} {
		if !strings.Contains(s, needle) {
			t.Fatalf("missing %s:\n%s", needle, s)
		}
	}
	if strings.Contains(s, `"password": "secret"`) {
		t.Fatalf("plaintext password leaked into runtime config: %s", s)
	}
}

func TestBuildMieruPrivilegedPort(t *testing.T) {
	b, err := BuildServerConfig(Record{ID: 7, Port: 443, Users: []User{{Name: "u", Password: "p"}}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"port": 443`) {
		t.Fatalf("privileged port missing: %s", b)
	}
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
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, needle := range []string{`"portRange": "20000-20010"`, `"protocol": "UDP"`, `"trafficPattern"`, `"maxSleepMs": 10`, `"maxMiddlePaddingLen": 64`, `"LOW_ENTROPY_MODE_32"`} {
		if !strings.Contains(s, needle) {
			t.Fatalf("missing %s:\n%s", needle, s)
		}
	}
}

func TestBuildMieruCompleteServerConfig(t *testing.T) {
	b, err := BuildServerConfig(Record{
		ID: 8, Port: 41000,
		Users: []User{{Name: "u@example.com", Password: "p"}},
		Settings: Settings{
			Transport:    "TCP",
			PortRangeEnd: 41002,
			AdditionalPortBindings: []PortBinding{
				{Port: 42000, Transport: "UDP"},
				{Port: 43000, PortRangeEnd: 43005, Transport: "TCP"},
			},
			DNSDualStack: "PREFER_IPv6",
			DNSHosts: []DNSHost{
				{Domain: "internal.example", IP: "10.0.0.8"},
				{Domain: "v6.example", IP: "2001:db8::8"},
			},
			EgressProxies: []EgressProxy{
				{Name: "local-socks", Host: "127.0.0.1", Port: 1080, Username: "alice", Password: "secret"},
			},
			EgressRules: []EgressRule{
				{DomainNames: []string{"example.com"}, Action: "PROXY", ProxyNames: []string{"local-socks"}},
				{IPRanges: []string{"10.0.0.0/8"}, Action: "REJECT"},
				{IPRanges: []string{"*"}, DomainNames: []string{"*"}, Action: "DIRECT"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(b, &root); err != nil {
		t.Fatal(err)
	}
	bindings, _ := root["portBindings"].([]any)
	if len(bindings) != 3 {
		t.Fatalf("expected 3 port bindings, got %d: %s", len(bindings), b)
	}
	dns, _ := root["dns"].(map[string]any)
	if dns["dualStack"] != "PREFER_IPv6" {
		t.Fatalf("dns dualStack missing: %s", b)
	}
	egress, _ := root["egress"].(map[string]any)
	if len(egress) == 0 {
		t.Fatalf("egress missing: %s", b)
	}
	s := string(b)
	for _, needle := range []string{`"portRange": "41000-41002"`, `"port": 42000`, `"portRange": "43000-43005"`, `"dualStack": "PREFER_IPv6"`, `"internal.example": "10.0.0.8"`, `"protocol": "SOCKS5_PROXY_PROTOCOL"`, `"socks5Authentication"`, `"action": "PROXY"`, `"action": "REJECT"`} {
		if !strings.Contains(s, needle) {
			t.Fatalf("missing %s:\n%s", needle, s)
		}
	}
}

func TestMieruDefaultsPreserveOfficialImplicitTrafficPattern(t *testing.T) {
	b, err := BuildServerConfig(Record{ID: 3, Port: 5001, Users: []User{{Name: "u", Password: "p"}}})
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if strings.Contains(s, `"trafficPattern"`) {
		t.Fatalf("trafficPattern must be omitted unless enabled: %s", s)
	}
	if !strings.Contains(s, `"protocol": "TCP"`) || !strings.Contains(s, `"mtu": 1400`) {
		t.Fatalf("defaults missing: %s", s)
	}
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

func TestOfficialMitaAcceptsRenderedConfig(t *testing.T) {
	bin := os.Getenv("MIERU_OFFICIAL_BINARY")
	if bin == "" {
		t.Skip("MIERU_OFFICIAL_BINARY is not set")
	}
	port := freeTCPPort(t)
	port2 := freeTCPPort(t)
	for port2 == port {
		port2 = freeTCPPort(t)
	}
	cfg, err := BuildServerConfig(Record{
		ID: 9, Port: port,
		Users: []User{{Name: "ci@example.com", Password: "ci-password"}},
		Settings: Settings{
			Transport:              "TCP",
			AdditionalPortBindings: []PortBinding{{Port: port2, Transport: "TCP"}},
			MTU:                    1400,
			DNSDualStack:           "ONLY_IPv4",
			DNSHosts:               []DNSHost{{Domain: "ci.invalid", IP: "127.0.0.1"}},
			EgressProxies:          []EgressProxy{{Name: "ci-socks", Host: "127.0.0.1", Port: 9}},
			EgressRules:            []EgressRule{{DomainNames: []string{"never.invalid"}, Action: "PROXY", ProxyNames: []string{"ci-socks"}}, {IPRanges: []string{"*"}, DomainNames: []string{"*"}, Action: "DIRECT"}},
			TrafficPatternEnabled:  true,
			TrafficSeed:            7,
			PaddingMaxMiddleLen:    pint(32),
			PaddingMaxEndLen:       pint(64),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	dir, err := os.MkdirTemp("", "3xpatcher-mieru-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	cfgPath := filepath.Join(dir, "server.json")
	uds := filepath.Join(dir, "mita.sock")
	if err := os.WriteFile(cfgPath, cfg, 0o640); err != nil {
		t.Fatal(err)
	}

	var credential *syscall.Credential
	if os.Geteuid() == 0 {
		if u, lookupErr := user.Lookup("mita"); lookupErr == nil {
			uid, uidErr := strconv.Atoi(u.Uid)
			gid, gidErr := strconv.Atoi(u.Gid)
			if uidErr != nil || gidErr != nil {
				t.Fatalf("invalid mita uid/gid: %v %v", uidErr, gidErr)
			}
			if err := os.Chown(dir, uid, gid); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(dir, 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.Chown(cfgPath, 0, gid); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(cfgPath, 0o640); err != nil {
				t.Fatal(err)
			}
			credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}
		}
	}

	env := append(os.Environ(), "MITA_CONFIG_JSON_FILE="+cfgPath, "MITA_UDS_PATH="+uds, "MITA_LOG_NO_TIMESTAMP=true")
	cmd := exec.Command(bin, "run")
	cmd.Env = env
	if credential != nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{Credential: credential}
	}
	var log bytes.Buffer
	cmd.Stdout = &log
	cmd.Stderr = &log
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		select {
		case <-done:
		case <-time.After(time.Second):
		}
	}()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-done:
			t.Fatalf("official mita exited before RUNNING: %v\n%s", err, log.String())
		default:
		}
		status := exec.Command(bin, "status")
		status.Env = env
		out, _ := status.CombinedOutput()
		if strings.Contains(string(out), `mita server status is "RUNNING"`) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("official mita did not accept rendered config:\n%s", log.String())
}

func TestRejectInvalidMieruRange(t *testing.T) {
	_, err := BuildServerConfig(Record{ID: 4, Port: 5000, Users: []User{{Name: "u", Password: "p"}}, Settings: Settings{PortRangeEnd: 4999}})
	if err == nil {
		t.Fatal("expected range error")
	}
}

func TestRejectIncompleteQuota(t *testing.T) {
	_, err := BuildServerConfig(Record{ID: 5, Port: 5000, Users: []User{{Name: "u", Password: "p"}}, Settings: Settings{QuotaDays: 1}})
	if err == nil {
		t.Fatal("expected quota error")
	}
}

func TestRejectUnknownEgressProxy(t *testing.T) {
	_, err := BuildServerConfig(Record{
		ID: 6, Port: 5000,
		Users:    []User{{Name: "u", Password: "p"}},
		Settings: Settings{EgressRules: []EgressRule{{DomainNames: []string{"*"}, Action: "PROXY", ProxyNames: []string{"missing"}}}},
	})
	if err == nil {
		t.Fatal("expected unknown egress proxy error")
	}
}
