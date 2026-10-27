package sub

import (
  "encoding/json"
  "strings"
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
	if _, ok := m["reality-opts"]; ok {
		t.Fatalf("TLS upstream and TLS download must not introduce REALITY options: %#v", m)
	}
	split := buildXhttpClashOpts(x, "tls")
	got := split["download-settings"].(map[string]any)
	if _, ok := got["reality-opts"]; ok {
		t.Fatalf("full TLS/TLS subscription output must have no REALITY options: %#v", got)
	}
	// TLS downlink with a REALITY uplink is a different transport
	// transition: explicitly clear the inherited outer REALITY public key.
	reset := xhttpDownloadMihomo(d, "reality")
	real := reset["reality-opts"].(map[string]any)
	if real["public-key"] != "" {
		t.Fatalf("REALITY-to-TLS must explicitly reset inherited key: %#v", real)
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


func TestXhttpManagedRealityDoesNotLeakPrivateKey(t *testing.T) {
    x := map[string]any{
        "path":"/common",
        "downloadSettings":map[string]any{
            "address":"198.51.100.10",
            "port":float64(9443),
            "security":"reality",
            "realitySettings":map[string]any{
                "serverName":"www.example.com",
                "publicKey":"client-public",
                "shortId":"ab12",
            },
            "managedListener":map[string]any{
                "enabled":true,
                "realitySettings":map[string]any{
                    "privateKey":"SERVER_PRIVATE_NEVER_SUBSCRIBE",
                    "publicKey":"client-public",
                    "serverNames":[]any{"www.example.com"},
                    "target":"www.example.com:443",
                },
            },
        },
    }
    d:=xhttpClientDownload(x)
    if d==nil {t.Fatal("Xray client REALITY parameters absent")}
    b,err:=json.Marshal(d)
    if err!=nil {t.Fatal(err)}
    if strings.Contains(string(b),"SERVER_PRIVATE_NEVER_SUBSCRIBE") || strings.Contains(string(b),"managedListener") {
       t.Fatalf("server-side REALITY data leaked into Xray JSON: %s",string(b))
    }
    m:=xhttpDownloadMihomo(d)
    b,err=json.Marshal(m)
    if err!=nil {t.Fatal(err)}
    if strings.Contains(string(b),"SERVER_PRIVATE_NEVER_SUBSCRIBE") {
       t.Fatal("server-side REALITY private key leaked into Mihomo YAML")
    }
}

func TestMihomoRealityDownloadDoesNotInheritCDNHostOrALPN(t *testing.T) {
  xhttp := map[string]any{
    "path": "/same-xhttp",
    "mode": "auto",
    "downloadSettings": map[string]any{
      "address": "198.51.100.20",
      "port": float64(9443),
      "security": "reality",
      "realitySettings": map[string]any{
        "serverName": "www.example.com",
        "publicKey": "EXAMPLE_PUBLIC_KEY",
        "shortId": "ab12",
        "fingerprint": "chrome",
      },
      "xhttpSettings": map[string]any{"host": ""},
    },
  }
  opts:=buildXhttpClashOpts(xhttp)
  down,ok:=opts["download-settings"].(map[string]any)
  if !ok {t.Fatalf("no downlink output: %#v",opts)}
  host,hasHost:=down["host"]
  if !hasHost || host!="" {t.Fatalf("empty Reality Host must be explicit to disable CDN Host inheritance: %#v",down)}
  alpn,ok:=down["alpn"].([]string)
  if !ok || len(alpn)!=1 || alpn[0]!="h2" {
    t.Fatalf("Reality downlink must override uplink ALPN with h2: %#v",down)
  }
  if down["servername"]!="www.example.com" || down["port"]!=float64(9443) {
    t.Fatalf("Reality port/SNI mismatch: %#v",down)
  }
}

func TestXhttpDownloadInheritsServerHostAndPreservesRealitySNI(t *testing.T) {
  x:=map[string]any{
    "host":"cdn.example.com","path":"/shared","xPaddingBytes":"100-1000",
    "downloadSettings":map[string]any{
      "address":"203.0.113.27","port":float64(9443),"security":"reality",
      "xhttpSettings":map[string]any{"host":""},
      "realitySettings":map[string]any{
        "serverName":"target.example.com",
        "fingerprint":"chrome","publicKey":"pub-key","shortId":"abcd",
      },
    },
  }
  down:=xhttpClientDownload(x)
  inner:=down["xhttpSettings"].(map[string]any)
  if inner["host"]!="cdn.example.com" { t.Fatalf("Host must match shared XHTTP inbound: %#v",inner) }
  if inner["path"]!="/shared" { t.Fatalf("wrong shared path: %#v",inner) }
  proxy:=xhttpDownloadMihomo(down)
  if proxy["host"]!="cdn.example.com" || proxy["servername"]!="target.example.com" {
    t.Fatalf("HTTP Host and REALITY SNI must stay independent: %#v",proxy)
  }
}

func TestXhttpDownloadStandardExtraPreservesRealityFields(t *testing.T) {
  x:=map[string]any{
    "host":"cdn.example.com", "path":"/single-session", "mode":"auto",
    "downloadSettings":map[string]any{
      "address":"203.0.113.25", "port":float64(443), "security":"reality",
      "realitySettings":map[string]any{
        "serverName":"www.example.com", "publicKey":"CLIENT_PUBLIC_KEY",
        "shortId":"aabbccdd", "fingerprint":"chrome",
      },
      "xhttpSettings":map[string]any{"host":"cdn.example.com"},
    },
  }
  // Only the documented Xray extra.downloadSettings is produced; the
  // Downlink REALITY credentials stay in the Xray-native nested object.
  extra:=buildXhttpExtra(x)
  down,ok:=extra["downloadSettings"].(map[string]any)
  if !ok {t.Fatalf("missing Xray downloadSettings: %#v",extra)}
  if down["address"]!="203.0.113.25" || down["security"]!="reality" {
    t.Fatalf("wrong Xray downlink: %#v",down)
  }
  real:=down["realitySettings"].(map[string]any)
  if real["publicKey"]!="CLIENT_PUBLIC_KEY" || real["shortId"]!="aabbccdd" {
    t.Fatalf("downlink credentials lost: %#v",real)
  }
  transport:=down["xhttpSettings"].(map[string]any)
  if transport["path"]!="/single-session" || transport["host"]!="cdn.example.com" {
    t.Fatalf("XHTTP downlink transport corrupted: %#v",transport)
  }
}
