package sub

// XHTTP split-download client rendering. The stored inbound's downloadSettings
// is panel metadata and MUST NOT be passed to the Xray listener. The split
// endpoints must both reach the same XHTTP inbound/path; these helpers only
// describe client connections, never deploy a CDN, reverse proxy, or fallback.
import (
	"encoding/json"
	"fmt"
	"strings"
)

// xhttpClientDownload returns the Xray-core client-side nested downloadSettings
// object, retaining only documented outbound fields. Accept either the XHTTP
// transport object itself or nil. Invalid/incomplete settings are NOT exported.
func xhttpClientDownload(xhttp map[string]any) map[string]any {
	if xhttp == nil {
		return nil
	}
	raw, ok := xhttp["downloadSettings"].(map[string]any)
	if !ok || len(raw) == 0 {
		return nil
	}
	address, _ := raw["address"].(string)
	address = strings.TrimSpace(address)
	port := 0
	switch p := raw["port"].(type) {
	case float64:
		port = int(p)
	case int:
		port = p
	}
	security, _ := raw["security"].(string)
	if address == "" || port < 1 || port > 65535 || (security != "tls" && security != "reality") {
		return nil
	}

	down := map[string]any{
		"address": address,
		"port": port,
		"network": "xhttp",
		"security": security,
	}
	xhttpDown := map[string]any{}
	if path, ok := xhttp["path"].(string); ok && path != "" {
		// Same listener/path: never let a stale client override drift from
		// the inbound after the administrator changes its path.
		xhttpDown["path"] = path
	}
	if mode, ok := xhttp["mode"].(string); ok && mode != "" {
		xhttpDown["mode"] = mode
	}
	rawXHTTP, _ := raw["xhttpSettings"].(map[string]any)
	host, _ := rawXHTTP["host"].(string)

	if security == "tls" {
		rawTLS, _ := raw["tlsSettings"].(map[string]any)
		sni, _ := rawTLS["serverName"].(string)
		if strings.TrimSpace(sni) == "" {
			return nil
		}
		fingerprint, _ := rawTLS["fingerprint"].(string)
		if fingerprint == "" {
			fingerprint = "chrome"
		}
		tls := map[string]any{
			"serverName": sni,
			"fingerprint": fingerprint,
		}
		if alpn, ok := rawTLS["alpn"].([]any); ok && len(alpn) > 0 {
			tls["alpn"] = alpn
		} else {
			tls["alpn"] = []string{"h2"}
		}
		down["tlsSettings"] = tls
		if host == "" {
			host = sni
		}
	} else {
		rawReality, _ := raw["realitySettings"].(map[string]any)
		sni, _ := rawReality["serverName"].(string)
		publicKey, _ := rawReality["publicKey"].(string)
		if strings.TrimSpace(sni) == "" || strings.TrimSpace(publicKey) == "" {
			return nil
		}
		fp, _ := rawReality["fingerprint"].(string)
		if fp == "" {
			fp = "chrome"
		}
		reality := map[string]any{
			"serverName": sni,
			"publicKey": publicKey,
			"fingerprint": fp,
		}
		if short, ok := rawReality["shortId"].(string); ok {
			reality["shortId"] = short
		}
		down["realitySettings"] = reality
	}
	// Both transports must reach the same XHTTP listener. When the downstream
	// Host is not specified, retain the upstream Host used by that listener.
	if host == "" { host, _ = xhttp["host"].(string) }
	xhttpDown["host"] = host
	down["xhttpSettings"] = xhttpDown
	return down
}

// Mihomo's download-settings inherits fields from its parent VLESS proxy.
// A REALITY-to-TLS transition must clear the inherited REALITY public key;
// TLS-to-TLS must not introduce any REALITY option at all.
func xhttpDownloadMihomo(download map[string]any, upstreamSecurity ...string) map[string]any {
	if download == nil {
		return nil
	}
	out := map[string]any{
		"server": download["address"],
		"port": download["port"],
		"tls": true,
	}
	xhttp, _ := download["xhttpSettings"].(map[string]any)
	if xhttp != nil {
		// Mihomo inherits omitted downlink fields from its uplink. An empty
		// Reality HTTP Host is intentional; omitting it leaks the CDN Host
		// into the Reality connection and breaks server-side XHTTP routing.
		if host, ok := xhttp["host"].(string); ok {
			out["host"] = host
		}
		if path, _ := xhttp["path"].(string); path != "" {
			out["path"] = path
		}
	}
	if download["security"] == "reality" {
		real, _ := download["realitySettings"].(map[string]any)
		// REALITY XHTTP supports H2, not HTTP/1.1 fallback inherited from
		// a CDN TLS upload's ALPN negotiation preferences.
		out["alpn"] = []string{"h2"}
		out["servername"] = real["serverName"]
		out["client-fingerprint"] = real["fingerprint"]
		out["reality-opts"] = map[string]any{
			"public-key": real["publicKey"],
			"short-id": real["shortId"],
		}
	} else {
		tls, _ := download["tlsSettings"].(map[string]any)
		out["servername"] = tls["serverName"]
		out["client-fingerprint"] = tls["fingerprint"]
		if alpn, ok := tls["alpn"]; ok {
			out["alpn"] = alpn
		}
		// Only a REALITY uplink contributes a public key that needs clearing.
		// Do not synthesize a REALITY option for an all-TLS XHTTP route.
		if len(upstreamSecurity) > 0 && upstreamSecurity[0] == "reality" {
			out["reality-opts"] = map[string]any{"public-key": ""}
		}
	}
	return out
}


// shadowrocketXHTTPDownloadParams adds Shadowrocket's separately exported
// xhttp obfsParam keys while retaining Xray's full extra.downloadSettings.
// Shadowrocket's native key names are established by exported node samples.
// Reality download credentials are still carried in the standard Xray extra;
// no undocumented Shadowrocket-specific credential names are fabricated.
func shadowrocketXHTTPDownloadParams(xhttp map[string]any, params map[string]string) {
	down := xhttpClientDownload(xhttp)
	if down == nil || params == nil { return }
	settings, _ := down["xhttpSettings"].(map[string]any)
	primaryHost, _ := xhttp["host"].(string)
	nested := map[string]any{
		"downloadTargetHost": down["address"],
		"downloadTargetPort": fmt.Sprint(down["port"]),
	}
// The two XHTTP legs deliberately share the top-level URI path. Native
	// Shadowrocket exports do not add downloadPath in obfsParam; keeping a
	// duplicate here can cause the app to choose the wrong path parser.
	if host, _ := settings["host"].(string); host != "" {
		nested["downloadHTTPHost"] = host
	}
	var serverName string
	if down["security"] == "reality" {
		reality, _ := down["realitySettings"].(map[string]any)
		serverName, _ = reality["serverName"].(string)
	} else {
		tls, _ := down["tlsSettings"].(map[string]any)
		serverName, _ = tls["serverName"].(string)
	}
	if serverName != "" { nested["downloadServerName"] = serverName }
	if padding, _ := xhttp["xPaddingBytes"].(string); padding != "" {
		nested["xPaddingBytes"] = padding
	}
	obj := map[string]any{"": nested}
	if primaryHost != "" { obj["Host"] = primaryHost }
	if raw, err := json.Marshal(obj); err == nil { params["obfsParam"] = string(raw) }
}

// shadowrocketXHTTPDownloadRealityQueryParams supplies the two fields a
// *native* Shadowrocket export carries at URI query level (pbk/sid) when a
// TLS uplink has a separate REALITY downlink. The canonical Xray
// extra.downloadSettings is retained unchanged. In particular, never
// overwrite pbk/sid for an upstream that itself uses REALITY, because
// those keys belong to the uplink in Xray/V2Ray's standard URI syntax.
//
// Call this only for VLESS after the upstream security options are emitted.
func shadowrocketXHTTPDownloadRealityQueryParams(xhttp map[string]any, params map[string]string) {
	if params == nil || params["security"] != "tls" {
		return
	}
	down := xhttpClientDownload(xhttp)
	if down == nil || down["security"] != "reality" {
		return
	}
	real, _ := down["realitySettings"].(map[string]any)
	key, _ := real["publicKey"].(string)
	if key == "" {
		return
	}
	if _, exists := params["pbk"]; !exists {
		params["pbk"] = key
	}
	if short, _ := real["shortId"].(string); short != "" {
		if _, exists := params["sid"]; !exists {
			params["sid"] = short
		}
	}
}
