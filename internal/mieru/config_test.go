package mieru

import (
	"strings"
	"testing"
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

func TestRejectInvalidMieruRange(t *testing.T) {
	_, err := BuildServerConfig(Record{ID: 4, Port: 5000, Users: []User{{Name: "u", Password: "p"}}, Settings: Settings{PortRangeEnd: 4999}})
	if err == nil { t.Fatal("expected range error") }
}

func TestRejectIncompleteQuota(t *testing.T) {
	_, err := BuildServerConfig(Record{ID: 5, Port: 5000, Users: []User{{Name: "u", Password: "p"}}, Settings: Settings{QuotaDays: 1}})
	if err == nil { t.Fatal("expected quota error") }
}
