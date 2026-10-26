package sub

import (
  "encoding/json"
  "testing"
)

func TestXhttpDownloadTLSCDN(t *testing.T) {
	x := map[string]any{
		"path": "/same-inbound",
		"mode": "auto",
		"downloadSettings": map[string]any{
			"address": "cdn.example.com", "port": float64(443),
			"security": "tls",
			"xhttpSettings": map[string]any{"host": ""},
			"tlsSettings": map[string]any{
				"serverName": "cdn.example.com", "fingerprint": "chrome",
				"alpn": []any{"h2"},
			},
		},
	}
	d := xhttpClientDownload(x)
	if d == nil || d["security"] != "tls" {
		t.Fatalf("missing TLS client download: %#v", d)
	}
	inner := d["xhttpSettings"].(map[string]any)
	if inner["path"] != "/same-inbound" || inner["host"] != "cdn.example.com" {
		t.Fatalf("wrong shared path/host: %#v", inner)
	}
	m := xhttpDownloadMihomo(d)
	if m["server"] != "cdn.example.com" || m["tls"] != true {
		t.Fatalf("wrong Mihomo downlink: %#v", m)
	}
	real := m["reality-opts"].(map[string]any)
	if real["public-key"] != "" {
		t.Fatalf("REALITY key must be reset on TLS downlink: %#v", real)
	}
}

func TestXhttpDownloadREALITY(t *testing.T) {
	x := map[string]any{
		"path": "/shared",
		"mode": "stream-up",
		"downloadSettings": map[string]any{
			"address": "203.0.113.20", "port": float64(443),
			"security": "reality",
			"xhttpSettings": map[string]any{"host": ""},
			"realitySettings": map[string]any{
				"serverName": "sni.example.com", "publicKey": "pub",
				"shortId": "a1b2", "fingerprint": "chrome",
			},
		},
	}
	d := xhttpClientDownload(x)
	if d == nil || d["security"] != "reality" {
		t.Fatalf("missing REALITY downlink: %#v", d)
	}
	if d["xhttpSettings"].(map[string]any)["path"] != "/shared" {
		t.Fatal("split XHTTP must use the same path")
	}
	m := xhttpDownloadMihomo(d)
	if m["servername"] != "sni.example.com" {
		t.Fatalf("downlink SNI lost: %#v", m)
	}
	if m["reality-opts"].(map[string]any)["short-id"] != "a1b2" {
		t.Fatalf("downlink short ID lost: %#v", m)
	}
}

func TestXhttpDownloadFailClosed(t *testing.T) {
	for _, in := range []map[string]any{
		{},
		{"downloadSettings": map[string]any{}},
		{"downloadSettings": map[string]any{"address": "", "port": float64(443), "security": "tls"}},
		{"downloadSettings": map[string]any{"address": "cdn", "port": float64(443), "security": "tls"}},
		{"downloadSettings": map[string]any{"address": "vps", "port": float64(443), "security": "reality"}},
	} {
		if got := xhttpClientDownload(in); got != nil {
			t.Fatalf("invalid download must be skipped: %#v", got)
		}
	}
}


// Guard the *real* subscription entry points, not just the conversion helper.
func TestXhttpDownloadSubscriptionEntrypoints(t *testing.T) {
	x := map[string]any{
		"host": "cdn-up.example.com",
		"path": "/same",
		"mode": "auto",
		"downloadSettings": map[string]any{
			"address": "203.0.113.5", "port": float64(443),
			"security": "reality",
			"xhttpSettings": map[string]any{"host": ""},
			"realitySettings": map[string]any{
				"serverName": "target.example.com", "publicKey": "some-key",
				"shortId": "ab12", "fingerprint": "chrome",
			},
		},
	}
	raw := buildXhttpExtra(x)
	down, ok := raw["downloadSettings"].(map[string]any)
	if !ok || down["address"] != "203.0.113.5" {
		t.Fatalf("share link extra must contain valid Xray stream: %#v", raw)
	}
	opts := buildXhttpClashOpts(x)
	cd, ok := opts["download-settings"].(map[string]any)
	if !ok || cd["server"] != "203.0.113.5" || cd["servername"] != "target.example.com" {
		t.Fatalf("Mihomo export missing flattened download settings: %#v", opts)
	}

	wire, err := json.Marshal(map[string]any{
		"network": "xhttp", "security": "none", "xhttpSettings": x,
	})
	if err != nil { t.Fatal(err) }
	service := &SubJsonService{}
	export := service.streamData(string(wire), "")
	xout, _ := export["xhttpSettings"].(map[string]any)
	if xout == nil {
		t.Fatalf("Xray JSON export lost xhttpSettings: %#v", export)
	}
	if _, exists := xout["downloadSettings"]; exists {
		t.Fatal("Xray outbound must nest downloadSettings under extra, not transport root")
	}
	inner, _ := xout["extra"].(map[string]any)
	if inner == nil || inner["downloadSettings"] == nil {
		t.Fatalf("Xray JSON export did not nest extra.downloadSettings: %#v", xout)
	}
}

func TestXhttpDownloadDisabledBackwardsCompatibility(t *testing.T) {
	x := map[string]any{"path":"/existing", "mode":"auto"}
	if _, ok := buildXhttpExtra(x)["downloadSettings"]; ok {
		t.Fatal("disabled XHTTP split must not produce a downloadSettings link option")
	}
	if _, ok := buildXhttpClashOpts(x)["download-settings"]; ok {
		t.Fatal("disabled XHTTP split must not produce a Mihomo download-settings")
	}
}
