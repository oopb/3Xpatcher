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

func TestXhttpDownloadInheritsServerHostAndShadowrocketURI(t *testing.T) {
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
  params:=map[string]string{}
  shadowrocketXHTTPDownloadParams(x,params)
  var parsed map[string]any
  if err:=json.Unmarshal([]byte(params["obfsParam"]),&parsed);err!=nil {t.Fatal(err)}
  opts:=parsed[""].(map[string]any)
  for key,want:=range map[string]string{
    "downloadTargetHost":"203.0.113.27",
    "downloadTargetPort":"9443",
    "downloadServerName":"target.example.com",
    "downloadHTTPHost":"cdn.example.com",
    // Native Shadowrocket shares the top-level XHTTP path; no nested downloadPath.
    "xPaddingBytes":"100-1000",
  } {
    if opts[key]!=want {t.Fatalf("%s got %v want %q",key,opts[key],want)}
  }
  if parsed["Host"]!="cdn.example.com" {t.Fatalf("uplink Host missing: %#v",parsed)}
  if strings.Contains(params["obfsParam"],"pub-key") {t.Fatal("do not encode unverified Shadowrocket reality key name")}
}


func TestXhttpDownloadShadowrocketNativeRealityKeysPreserveXrayURI(t *testing.T) {
  // Regression from a Shadowrocket-exported TLS uplink / REALITY download.
  // Keep Xray's standard VLESS authority, encryption query, and
  // extra.downloadSettings; only supply the downlink-specific pbk/sid.
  x:=map[string]any{
    "host":"cdn03.ijj.kdns.fr","path":"/fe98239xhnwe7743xfdw",
    "mode":"auto","xPaddingBytes":"100-1000",
    "downloadSettings":map[string]any{
      "address":"23.246.167.78","port":float64(443),"security":"reality",
      "xhttpSettings":map[string]any{"host":"cdn03.ijj.kdns.fr"},
      "realitySettings":map[string]any{
        "serverName":"aws.amazon.com",
        "publicKey":"7_B0g4hZQ9Kxw6TicXrAIvHxvek3vbfZQmUpko1RBls",
        "shortId":"b926c80a0e3759","fingerprint":"chrome",
      },
    },
  }
  params:=map[string]string{
    "type":"xhttp","security":"tls","encryption":"mlkem768x25519plus.native.0rtt.SAMPLE",
    "sni":"cdn03.ijj.kdns.fr","flow":"xtls-rprx-vision",
    "path":"/fe98239xhnwe7743xfdw",
  }
  extra:=buildXhttpExtra(x)
  b,err:=json.Marshal(extra);if err!=nil {t.Fatal(err)}
  params["extra"]=string(b)
  shadowrocketXHTTPDownloadParams(x,params)
  shadowrocketXHTTPDownloadRealityQueryParams(x,params)
  const key="7_B0g4hZQ9Kxw6TicXrAIvHxvek3vbfZQmUpko1RBls"
  if params["pbk"]!=key || params["sid"]!="b926c80a0e3759" {
    t.Fatalf("Shadowrocket downlink REALITY credentials missing: %#v",params)
  }
  for k,want:=range map[string]string{
    "security":"tls","encryption":"mlkem768x25519plus.native.0rtt.SAMPLE",
    "flow":"xtls-rprx-vision","type":"xhttp","sni":"cdn03.ijj.kdns.fr",
  } {
    if params[k]!=want {t.Fatalf("modified standard Xray URI %s: got %q want %q",k,params[k],want)}
  }
  var sr map[string]any
  if err:=json.Unmarshal([]byte(params["obfsParam"]),&sr);err!=nil {t.Fatal(err)}
  nested,ok:=sr[""].(map[string]any)
  if !ok {t.Fatalf("non-native Shadowrocket shape: %#v",sr)}
  for k,want:=range map[string]string{
    "downloadServerName":"aws.amazon.com","downloadTargetHost":"23.246.167.78",
    "downloadTargetPort":"443","downloadHTTPHost":"cdn03.ijj.kdns.fr",
    "xPaddingBytes":"100-1000",
  } {
    if nested[k]!=want {t.Fatalf("obfsParam %s got %v, want %q",k,nested[k],want)}
  }
  if _,ok:=nested["downloadPath"];ok {t.Fatalf("redundant non-native downloadPath: %#v",nested)}
  if sr["Host"]!="cdn03.ijj.kdns.fr" {t.Fatalf("missing SR parent Host: %#v",sr)}
  var canonical map[string]any
  if err:=json.Unmarshal([]byte(params["extra"]),&canonical);err!=nil {t.Fatal(err)}
  down,ok:=canonical["downloadSettings"].(map[string]any)
  if !ok || down["security"]!="reality" || down["address"]!="23.246.167.78" {
    t.Fatalf("standard Xray downlink damaged: %#v",canonical)
  }
  reality:=down["realitySettings"].(map[string]any)
  if reality["publicKey"]!=key || reality["shortId"]!="b926c80a0e3759" {
    t.Fatalf("standard Xray REALITY credentials damaged: %#v",reality)
  }
  dx:=down["xhttpSettings"].(map[string]any)
  if dx["path"]!="/fe98239xhnwe7743xfdw" {
    t.Fatalf("standard Xray downlink path missing: %#v",dx)
  }
}

func TestXhttpDownloadShadowrocketKeyIsolation(t *testing.T) {
 x:=map[string]any{"path":"/same","downloadSettings":map[string]any{
   "address":"203.0.113.9","port":float64(10443),"security":"reality",
   "realitySettings":map[string]any{"serverName":"www.example.com","publicKey":"DOWN_KEY","shortId":"DOWN_ID"},
 }}
 for _,tc:=range []struct{name string;params map[string]string;pbk,sid string}{
  {"TLS upstream",map[string]string{"security":"tls"},"DOWN_KEY","DOWN_ID"},
  {"REALITY upstream",map[string]string{"security":"reality","pbk":"UP_KEY","sid":"UP_ID"},"UP_KEY","UP_ID"},
  {"plain upstream",map[string]string{"security":"none"},"",""},
 } {
   t.Run(tc.name,func(t *testing.T){
    shadowrocketXHTTPDownloadRealityQueryParams(x,tc.params)
    if tc.params["pbk"]!=tc.pbk || tc.params["sid"]!=tc.sid {
     t.Fatalf("uplink security/query polluted: %#v",tc.params)
    }
   })
 }
 tls:=map[string]any{"path":"/same","downloadSettings":map[string]any{
    "address":"cdn.example.org","port":float64(2083),"security":"tls",
    "tlsSettings":map[string]any{"serverName":"cdn.example.org"},
 }}
 params:=map[string]string{"security":"tls","encryption":"none"}
 shadowrocketXHTTPDownloadRealityQueryParams(tls,params)
 shadowrocketXHTTPDownloadParams(tls,params)
 if _,has:=params["pbk"];has {t.Fatal("TLS/TLS must not inject REALITY pbk")}
 if _,has:=params["sid"];has {t.Fatal("TLS/TLS must not inject REALITY sid")}
 var obfs map[string]any
 if err:=json.Unmarshal([]byte(params["obfsParam"]),&obfs);err!=nil {t.Fatal(err)}
 if _,has:=obfs[""].(map[string]any)["downloadPath"];has {t.Fatal("redundant path in TLS/TLS obfs")}
}
