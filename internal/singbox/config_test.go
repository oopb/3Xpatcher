package singbox

import (
	"encoding/json"
	"strings"
	"testing"
)

func tls() TLSSettings {
	return TLSSettings{Enabled: true, CertificatePath: "/cert.pem", KeyPath: "/key.pem"}
}
func raw(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
func boolPtr(v bool) *bool      { return &v }

func realityTLS() TLSSettings {
	return TLSSettings{
		Enabled:    true,
		ServerName: "www.microsoft.com",
		Reality: &RealitySettings{
			Enabled:           true,
			HandshakeServer:   "www.microsoft.com",
			HandshakePort:     443,
			PrivateKey:        "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
			ShortIDs:          []string{"0123456789abcdef"},
			MaxTimeDifference: "60s",
		},
	}
}

func TestSupportedProtocolsIncludeSnellAndExcludeXrayProtocols(t *testing.T) {
	if !IsSupportedProtocol(ProtocolSnell) {
		t.Fatal("snell must be exposed by supplemental sing-box core")
	}
	for _, p := range []Protocol{"vless", "vmess", "trojan", "shadowsocks", "hysteria2"} {
		if IsSupportedProtocol(p) {
			t.Fatalf("%s must not be exposed by supplemental core", p)
		}
	}
}

func TestBuildFourProtocolConfig(t *testing.T) {
	records := []InboundRecord{
		{Enable: true, Remark: "t", Listen: "::", Port: 443, Tag: "tuic-443", Protocol: ProtocolTUIC, Settings: raw(TUICSettings{Users: []TUICUser{{UUID: "550e8400-e29b-41d4-a716-446655440000", Password: "p"}}, TLS: tls()})},
		{Enable: true, Remark: "a", Listen: "::", Port: 8443, Tag: "anytls-8443", Protocol: ProtocolAnyTLS, Settings: raw(AnyTLSSettings{Users: []PasswordUser{{Name: "a", Password: "p"}}, TLS: tls()})},
		{Enable: true, Remark: "n", Listen: "::", Port: 9443, Tag: "naive-9443", Protocol: ProtocolNaive, Settings: raw(NaiveSettings{Users: []NaiveUser{{Username: "u", Password: "p"}}, TLS: tls()})},
		{Enable: true, Remark: "s", Listen: "::", Port: 10443, Tag: "st-10443", Protocol: ProtocolShadowTLS, Settings: raw(ShadowTLSSettings{Users: []PasswordUser{{Name: "u", Password: "p"}}, HandshakeServer: "www.microsoft.com", InnerPassword: "AAAAAAAAAAAAAAAAAAAAAA=="})},
	}
	b, err := BuildConfig(records)
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, needle := range []string{`"type": "tuic"`, `"type": "anytls"`, `"type": "naive"`, `"type": "shadowtls"`, `"tag": "st-10443-inner"`, `"type": "shadowsocks"`} {
		if !strings.Contains(text, needle) {
			t.Fatalf("missing %s in config:\n%s", needle, text)
		}
	}
	if strings.Contains(text, `"type": "snell"`) {
		t.Fatal("Snell must not be generated unless an Snell record is present")
	}
}

func TestBuildAnyTLSReality(t *testing.T) {
	s := AnyTLSSettings{
		Users: []PasswordUser{{Name: "a", Password: "p"}},
		TLS:   realityTLS(),
	}
	b, err := BuildConfig([]InboundRecord{{Enable: true, Remark: "r", Listen: "::", Port: 443, Tag: "anytls-reality", Protocol: ProtocolAnyTLS, Settings: raw(s)}})
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, needle := range []string{
		`"type": "anytls"`,
		`"server_name": "www.microsoft.com"`,
		`"reality": {`,
		`"server": "www.microsoft.com"`,
		`"server_port": 443`,
		`"private_key": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"`,
		`"0123456789abcdef"`,
		`"max_time_difference": "60s"`,
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("missing Reality field %s:\n%s", needle, text)
		}
	}
	if strings.Contains(text, `"certificate_path"`) || strings.Contains(text, `"key_path"`) {
		t.Fatalf("Reality must not require a certificate: %s", text)
	}
}

func TestRejectRealityOnQUICAndNaive(t *testing.T) {
	_, err := BuildConfig([]InboundRecord{{
		Enable: true, Listen: "::", Port: 443, Tag: "tuic-r", Protocol: ProtocolTUIC,
		Settings: raw(TUICSettings{Users: []TUICUser{{UUID: "550e8400-e29b-41d4-a716-446655440000", Password: "p"}}, TLS: realityTLS()}),
	}})
	if err == nil || !strings.Contains(err.Error(), "TUIC/QUIC") {
		t.Fatalf("expected TUIC Reality rejection, got %v", err)
	}

	_, err = BuildConfig([]InboundRecord{{
		Enable: true, Listen: "::", Port: 443, Tag: "naive-r", Protocol: ProtocolNaive,
		Settings: raw(NaiveSettings{Network: "tcp", Users: []NaiveUser{{Username: "u", Password: "p"}}, TLS: realityTLS()}),
	}})
	if err == nil || !strings.Contains(err.Error(), "Naive outbound") {
		t.Fatalf("expected Naive Reality rejection, got %v", err)
	}
}

func TestAdvancedTUICAndListenFields(t *testing.T) {
	s := TUICSettings{
		Users: []TUICUser{{UUID: "550e8400-e29b-41d4-a716-446655440000", Password: "p"}},
		TLS: TLSSettings{Enabled: true, CertificatePath: "/cert.pem", KeyPath: "/key.pem", MinVersion: "1.2", MaxVersion: "1.3", CurvePreferences: []string{"X25519"}},
		ListenSettings: ListenSettings{BindInterface: "eth0", TCPFastOpen: true, UDPFragment: boolPtr(true), UDPTimeout: "5m"},
		QUICSettings:   QUICSettings{IdleTimeout: "30s", KeepAlivePeriod: "10s", InitialPacketSize: 1350, DisablePathMTUDiscovery: true},
	}
	b, err := BuildConfig([]InboundRecord{{Enable: true, Listen: "::", Port: 443, Tag: "t", Protocol: ProtocolTUIC, Settings: raw(s)}})
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, needle := range []string{`"bind_interface": "eth0"`, `"tcp_fast_open": true`, `"udp_fragment": true`, `"idle_timeout": "30s"`, `"initial_packet_size": 1350`, `"disable_path_mtu_discovery": true`, `"min_version": "1.2"`, `"curve_preferences"`} {
		if !strings.Contains(text, needle) {
			t.Fatalf("missing advanced field %s:\n%s", needle, text)
		}
	}
}

func TestExplicitDisableUDPFragment(t *testing.T) {
	s := TUICSettings{Users: []TUICUser{{UUID: "550e8400-e29b-41d4-a716-446655440000", Password: "p"}}, TLS: tls(), ListenSettings: ListenSettings{UDPFragment: boolPtr(false)}}
	b, err := BuildConfig([]InboundRecord{{Enable: true, Listen: "::", Port: 443, Tag: "t", Protocol: ProtocolTUIC, Settings: raw(s)}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"udp_fragment": false`) {
		t.Fatalf("explicit false was lost: %s", b)
	}
}

func TestShadowTLSHandshakeForSNI(t *testing.T) {
	s := ShadowTLSSettings{Users: []PasswordUser{{Name: "u", Password: "p"}}, HandshakeServer: "www.cloudflare.com", HandshakeForServerNameJSON: `{"example.com":{"server":"example.com","server_port":443}}`, InnerPassword: "AAAAAAAAAAAAAAAAAAAAAA=="}
	b, err := BuildConfig([]InboundRecord{{Enable: true, Listen: "::", Port: 443, Tag: "st", Protocol: ProtocolShadowTLS, Settings: raw(s)}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"handshake_for_server_name"`) {
		t.Fatalf("missing handshake_for_server_name: %s", b)
	}
}

func TestRejectInvalidTUICUUID(t *testing.T) {
	_, err := BuildConfig([]InboundRecord{{Enable: true, Listen: "::", Port: 443, Tag: "t", Protocol: ProtocolTUIC, Settings: raw(TUICSettings{Users: []TUICUser{{UUID: "bad", Password: "p"}}, TLS: tls()})}})
	if err == nil {
		t.Fatal("expected invalid UUID error")
	}
}
