package sub

import "testing"

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
