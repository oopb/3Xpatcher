package sub

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func TestBuildShadowTLSShareLinksCompatibility(t *testing.T) {
	settings := map[string]any{
		"innerMethod":    "2022-blake3-aes-128-gcm",
		"innerPassword":  "MDEyMzQ1Njc4OWFiY2RlZg==",
		"handshakeServer": "captive.apple.com",
	}
	got := buildShadowTLSShareLinks(settings, "shadow-user-password", "203.0.113.10", 443, "stls-user")
	if strings.Contains(got, "shadowtls://") {
		t.Fatalf("legacy custom shadowtls URI leaked into export: %s", got)
	}
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected SIP002 and Shadowrocket variants, got %d: %q", len(lines), got)
	}

	first, err := url.Parse(lines[0])
	if err != nil {
		t.Fatalf("parse SIP002 link: %v", err)
	}
	if first.Scheme != "ss" || first.Hostname() != "203.0.113.10" || first.Port() != "443" {
		t.Fatalf("unexpected SIP002 endpoint: %s", lines[0])
	}
	plugin := first.Query().Get("plugin")
	for _, want := range []string{"shadow-tls", "host=captive.apple.com", "password=shadow-user-password", "version=3"} {
		if !strings.Contains(plugin, want) {
			t.Fatalf("SIP002 plugin is missing %q: %q", want, plugin)
		}
	}

	qIndex := strings.Index(lines[1], "?")
	if qIndex < 0 {
		t.Fatalf("Shadowrocket link has no query: %s", lines[1])
	}
	query, err := url.ParseQuery(strings.SplitN(strings.SplitN(lines[1][qIndex+1:], "#", 2)[0], "#", 2)[0])
	if err != nil {
		t.Fatalf("parse Shadowrocket query: %v", err)
	}
	rawDescriptor := query.Get("shadow-tls")
	decoded, err := base64.StdEncoding.DecodeString(rawDescriptor)
	if err != nil {
		t.Fatalf("decode Shadowrocket descriptor: %v", err)
	}
	var descriptor map[string]any
	if err := json.Unmarshal(decoded, &descriptor); err != nil {
		t.Fatalf("unmarshal Shadowrocket descriptor: %v", err)
	}
	if descriptor["version"] != "3" || descriptor["password"] != "shadow-user-password" || descriptor["host"] != "captive.apple.com" {
		t.Fatalf("unexpected Shadowrocket descriptor: %#v", descriptor)
	}
}
