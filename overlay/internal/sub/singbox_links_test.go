package sub

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestShadowrocketShadowTLSUsesDescriptorNotSIP003(t *testing.T) {
	inbound := &model.Inbound{Protocol: model.ShadowTLS, Port: 443}
	client := model.Client{Email: "stls@example.com", Password: "shadow-user-password"}
	settings := map[string]any{
		"innerMethod":    "2022-blake3-aes-128-gcm",
		"innerPassword":  "MDEyMzQ1Njc4OWFiY2RlZg==",
		"handshakeServer": "captive.apple.com",
	}
	got := (&SubService{clientUserAgent: "Shadowrocket/2.2.86"}).buildSingboxEndpointLink(
		inbound, client, settings, map[string]any{}, nil, "203.0.113.10", 443,
	)
	if strings.Contains(got, "plugin=") || !strings.Contains(got, "shadow-tls=") {
		t.Fatalf("Shadowrocket must receive descriptor ShadowTLS, not SIP003: %s", got)
	}
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse Shadowrocket link: %v", err)
	}
	rawDescriptor := parsed.Query().Get("shadow-tls")
	decoded, err := base64.StdEncoding.DecodeString(rawDescriptor)
	if err != nil {
		t.Fatalf("decode descriptor: %v", err)
	}
	var descriptor map[string]any
	if err := json.Unmarshal(decoded, &descriptor); err != nil {
		t.Fatalf("unmarshal descriptor: %v", err)
	}
	if descriptor["version"] != "3" || descriptor["password"] != client.Password || descriptor["host"] != "captive.apple.com" || descriptor["address"] != "203.0.113.10" || descriptor["port"] != "443" {
		t.Fatalf("unexpected Shadowrocket descriptor: %#v", descriptor)
	}
}

func TestGenericShadowTLSKeepsSIP003Only(t *testing.T) {
	settings := map[string]any{
		"innerMethod":    "2022-blake3-aes-128-gcm",
		"innerPassword":  "MDEyMzQ1Njc4OWFiY2RlZg==",
		"handshakeServer": "captive.apple.com",
	}
	got := buildShadowTLSSIP003Link(settings, "shadow-user-password", "203.0.113.10", 443, "stls-user")
	if strings.Contains(got, "shadow-tls=") || !strings.Contains(got, "plugin=") {
		t.Fatalf("generic ShadowTLS must remain the explicit SIP003 form: %s", got)
	}
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse SIP003 link: %v", err)
	}
	plugin := parsed.Query().Get("plugin")
	for _, want := range []string{"shadow-tls", "host=captive.apple.com", "password=shadow-user-password", "version=3"} {
		if !strings.Contains(plugin, want) {
			t.Fatalf("SIP003 plugin missing %q: %q", want, plugin)
		}
	}
}

func TestShadowrocketNaiveMirrorsSuiHTTP2Link(t *testing.T) {
	inbound := &model.Inbound{Protocol: model.Naive, Port: 443}
	client := model.Client{Email: "alice@example.com", Password: "secret"}
	settings := map[string]any{"tcpFastOpen": true, "network": "tcp"}
	stream := map[string]any{
		"security": "tls",
		"tlsSettings": map[string]any{
			"serverName":      "naive.example.com",
			"certificateMode": "self_signed_sni",
			"alpn":            []any{"h2"},
		},
	}

	shadowrocket := (&SubService{clientUserAgent: "Shadowrocket/2.2.86"}).buildSingboxEndpointLink(
		inbound, client, settings, stream, nil, "203.0.113.20", 443,
	)
	parsed, err := url.Parse(shadowrocket)
	if err != nil {
		t.Fatalf("parse Shadowrocket Naive link: %v", err)
	}
	if parsed.Scheme != "http2" {
		t.Fatalf("S-UI-compatible Shadowrocket Naive link must be http2://: %s", shadowrocket)
	}
	authority, err := base64.StdEncoding.DecodeString(parsed.Host)
	if err != nil {
		t.Fatalf("decode http2 authority: %v (%s)", err, parsed.Host)
	}
	if string(authority) != "alice@example.com:secret@203.0.113.20:443" {
		t.Fatalf("unexpected http2 authority: %q", authority)
	}
	q := parsed.Query()
	checks := map[string]string{
		"padding":  "1",
		"peer":     "naive.example.com",
		"alpn":     "h2",
		"insecure": "1",
		"tfo":      "1",
	}
	for key, want := range checks {
		if got := q.Get(key); got != want {
			t.Fatalf("Naive param %s: got %q want %q; link=%s", key, got, want, shadowrocket)
		}
	}
	if q.Get("sni") != "" {
		t.Fatalf("S-UI Naive compatibility link must use peer, not sni: %s", shadowrocket)
	}
}

func TestGenericNaiveUsesNetworkSpecificNativeScheme(t *testing.T) {
	inbound := &model.Inbound{Protocol: model.Naive, Port: 443}
	client := model.Client{Email: "alice@example.com", Password: "secret"}
	stream := map[string]any{"security": "tls", "tlsSettings": map[string]any{"serverName": "naive.example.com"}}

	tcp := (&SubService{}).buildSingboxEndpointLink(inbound, client, map[string]any{"network": "tcp"}, stream, nil, "203.0.113.20", 443)
	if !strings.HasPrefix(tcp, "naive+https://") || strings.Contains(tcp, "\n") {
		t.Fatalf("TCP Naive must export one naive+https link: %q", tcp)
	}
	udp := (&SubService{}).buildSingboxEndpointLink(inbound, client, map[string]any{"network": "udp"}, stream, nil, "203.0.113.20", 443)
	if !strings.HasPrefix(udp, "naive+quic://") || strings.Contains(udp, "\n") {
		t.Fatalf("UDP Naive must export one naive+quic link: %q", udp)
	}
}

func TestTUICClashRestoresPreMieruShapeAndSuiH3(t *testing.T) {
	inbound := &model.Inbound{
		Protocol: model.TUIC,
		Listen:   "203.0.113.30",
		Port:     10443,
		Settings: `{"congestionControl":"bbr","zeroRTTHandshake":true,"heartbeat":"10s","maxConcurrentStreams":20,"disablePathMTUDiscovery":true}`,
	}
	client := model.Client{
		Email:    "tuic@example.com",
		ID:       "00000000-0000-4000-8000-000000000001",
		Password: "tuic-password",
	}
	stream := map[string]any{
		"security": "tls",
		"tlsSettings": map[string]any{
			"serverName":      "tuic.example.com",
			"certificateMode": "self_signed_sni",
			"alpn":            []any{"custom-server-value"},
		},
	}

	proxy := (&SubClashService{}).buildSingboxProxy(&SubService{}, inbound, client, stream, nil)
	if proxy == nil {
		t.Fatal("TUIC proxy unexpectedly omitted")
	}
	checks := map[string]any{
		"type":                  "tuic",
		"uuid":                  client.ID,
		"password":              client.Password,
		"congestion-controller": "bbr",
		"reduce-rtt":            true,
		"sni":                   "tuic.example.com",
		"skip-cert-verify":      true,
	}
	for key, want := range checks {
		if got := proxy[key]; got != want {
			t.Fatalf("TUIC field %s: got %#v want %#v; proxy=%#v", key, got, want, proxy)
		}
	}
	for _, forbidden := range []string{"heartbeat-interval", "udp-relay-mode", "max-open-streams", "disable-mtu-discovery", "disable-sni"} {
		if _, exists := proxy[forbidden]; exists {
			t.Fatalf("server-side/experimental TUIC field %q leaked into Clash profile: %#v", forbidden, proxy)
		}
	}
	alpn, ok := proxy["alpn"].([]string)
	if !ok || len(alpn) != 1 || alpn[0] != "h3" {
		t.Fatalf("TUIC Clash ALPN must match S-UI h3 output: %#v", proxy["alpn"])
	}
}
