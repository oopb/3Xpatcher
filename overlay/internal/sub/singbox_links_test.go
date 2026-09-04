package sub

import (
	"net/url"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestBuildShadowTLSShareLinksCompatibility(t *testing.T) {
	settings := map[string]any{
		"innerMethod":    "2022-blake3-aes-128-gcm",
		"innerPassword":  "MDEyMzQ1Njc4OWFiY2RlZg==",
		"handshakeServer": "captive.apple.com",
	}
	got := buildShadowTLSShareLinks(settings, "shadow-user-password", "203.0.113.10", 443, "stls-user")
	if strings.Contains(got, "shadowtls://") || strings.Contains(got, "shadow-tls=") {
		t.Fatalf("legacy ShadowTLS variant leaked into export: %s", got)
	}
	if strings.Contains(got, "\n") {
		t.Fatalf("ShadowTLS must export exactly one working SIP003 node: %q", got)
	}

	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse SIP002 link: %v", err)
	}
	if parsed.Scheme != "ss" || parsed.Hostname() != "203.0.113.10" || parsed.Port() != "443" {
		t.Fatalf("unexpected SIP002 endpoint: %s", got)
	}
	plugin := parsed.Query().Get("plugin")
	for _, want := range []string{"shadow-tls", "host=captive.apple.com", "password=shadow-user-password", "version=3"} {
		if !strings.Contains(plugin, want) {
			t.Fatalf("SIP002 plugin is missing %q: %q", want, plugin)
		}
	}
}

func TestNaiveRawLinkUsesShadowrocketNativeHTTPS(t *testing.T) {
	inbound := &model.Inbound{Protocol: model.Naive, Port: 443}
	client := model.Client{Email: "alice@example.com", Password: "secret"}
	settings := map[string]any{}
	stream := map[string]any{
		"security": "tls",
		"tlsSettings": map[string]any{
			"serverName":      "naive.example.com",
			"certificateMode": "self_signed_sni",
		},
	}

	standard := (&SubService{}).buildSingboxEndpointLink(inbound, client, settings, stream, nil, "203.0.113.20", 443)
	if !strings.HasPrefix(standard, "naive+https://") {
		t.Fatalf("generic raw link must stay portable naive+https: %s", standard)
	}

	shadowrocket := (&SubService{clientUserAgent: "Shadowrocket/2.2.86"}).buildSingboxEndpointLink(inbound, client, settings, stream, nil, "203.0.113.20", 443)
	if !strings.HasPrefix(shadowrocket, "https://") || strings.HasPrefix(shadowrocket, "naive+https://") {
		t.Fatalf("Shadowrocket raw subscription must use native HTTPS NaiveProxy form: %s", shadowrocket)
	}
	if !strings.Contains(shadowrocket, "sni=naive.example.com") || !strings.Contains(shadowrocket, "insecure=1") {
		t.Fatalf("Shadowrocket Naive link lost TLS parameters: %s", shadowrocket)
	}
}

func TestBuildTUICMihomoCompatibilityFields(t *testing.T) {
	inbound := &model.Inbound{
		Protocol: model.TUIC,
		Listen:   "203.0.113.30",
		Port:     10443,
		Settings: `{"congestionControl":"bbr","heartbeat":"10s","maxConcurrentStreams":20,"disablePathMTUDiscovery":true}`,
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
			"alpn":            []any{"h3"},
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
		"heartbeat-interval":    10000,
		"udp-relay-mode":        "native",
		"max-open-streams":      20,
		"disable-mtu-discovery": true,
		"sni":                   "tuic.example.com",
		"skip-cert-verify":      true,
	}
	for key, want := range checks {
		if got := proxy[key]; got != want {
			t.Fatalf("TUIC field %s: got %#v want %#v; proxy=%#v", key, got, want, proxy)
		}
	}
	if _, disabled := proxy["disable-sni"]; disabled {
		t.Fatalf("disable-sni must not be set when explicit SNI exists: %#v", proxy)
	}
}

func TestBuildTUICDisablesImplicitSNIWhenUnset(t *testing.T) {
	inbound := &model.Inbound{
		Protocol: model.TUIC,
		Listen:   "203.0.113.31",
		Port:     10443,
		Settings: `{}`,
	}
	client := model.Client{ID: "00000000-0000-4000-8000-000000000002", Password: "pw"}
	proxy := (&SubClashService{}).buildSingboxProxy(&SubService{}, inbound, client, map[string]any{"security": "tls"}, nil)
	if proxy == nil || proxy["disable-sni"] != true {
		t.Fatalf("TUIC without SNI must disable implicit Mihomo SNI: %#v", proxy)
	}
}
