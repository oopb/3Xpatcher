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

func TestSupportedProtocolsExcludeSnellAndXrayProtocols(t *testing.T) {
	for _, p := range []Protocol{"snell", "vless", "vmess", "trojan", "shadowsocks", "hysteria2"} {
		if IsSupportedProtocol(p) {
			t.Fatalf("%s must not be exposed by V1 supplemental core", p)
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
		t.Fatal("Snell must not be generated in V1")
	}
}

func TestRejectInvalidTUICUUID(t *testing.T) {
	_, err := BuildConfig([]InboundRecord{{Enable: true, Listen: "::", Port: 443, Tag: "t", Protocol: ProtocolTUIC, Settings: raw(TUICSettings{Users: []TUICUser{{UUID: "bad", Password: "p"}}, TLS: tls()})}})
	if err == nil {
		t.Fatal("expected invalid UUID error")
	}
}
