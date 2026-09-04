package sub

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func (s *SubService) genSingboxLink(inbound *model.Inbound, email string) string {
	if !model.IsSingboxProtocol(inbound.Protocol) {
		return ""
	}
	client, ok := s.clientForLink(inbound, email)
	if !ok {
		return ""
	}
	settings := s.linkSettings(inbound)
	stream := unmarshalStreamSettings(inbound.StreamSettings)
	external, _ := stream["externalProxy"].([]any)
	if len(external) == 0 {
		return s.buildSingboxEndpointLink(inbound, client, settings, stream, nil, s.resolveInboundAddress(inbound), inbound.Port)
	}
	links := make([]string, 0, len(external))
	for _, raw := range external {
		ep, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		dest, _ := ep["dest"].(string)
		port, ok := ep["port"].(float64)
		if dest == "" || !ok {
			continue
		}
		if link := s.buildSingboxEndpointLink(inbound, client, settings, stream, ep, dest, int(port)); link != "" {
			links = append(links, link)
		}
	}
	return strings.Join(links, "\n")
}

func (s *SubService) buildSingboxEndpointLink(inbound *model.Inbound, client model.Client, settings, stream, ep map[string]any, host string, port int) string {
	params := map[string]string{}
	transport := ""
	if inbound.Protocol == model.TUIC {
		transport = "quic"
	}
	remark := s.endpointRemark(inbound, client.Email, ep, transport)
	applySingboxTLSLinkParams(settings, stream, ep, params)

	switch inbound.Protocol {
	case model.TUIC:
		if client.ID == "" || client.Password == "" {
			return ""
		}
		if v, _ := settings["congestionControl"].(string); v != "" {
			params["congestion_control"] = v
		}
		if v, _ := settings["zeroRTTHandshake"].(bool); v {
			params["zero_rtt_handshake"] = "1"
		}
		base := fmt.Sprintf("tuic://%s:%s@%s", encodeUserinfo(client.ID), encodeUserinfo(client.Password), joinHostPort(host, port))
		return buildLinkWithParams(base, params, remark)
	case model.AnyTLS:
		if client.Password == "" {
			return ""
		}
		return buildLinkWithParams(fmt.Sprintf("anytls://%s@%s", encodeUserinfo(client.Password), joinHostPort(host, port)), params, remark)
	case model.ShadowTLS:
		if client.Password == "" {
			return ""
		}
		if isShadowrocketUserAgent(s.clientUserAgent) {
			return buildShadowrocketShadowTLSLink(settings, client.Password, host, port, remark)
		}
		return buildShadowTLSSIP003Link(settings, client.Password, host, port, remark)
	case model.Naive:
		if client.Email == "" || client.Password == "" {
			return ""
		}
		applySuiNaiveLinkParams(settings, params)
		if isShadowrocketUserAgent(s.clientUserAgent) {
			return buildShadowrocketNaiveHTTP2Link(client.Email, client.Password, host, port, params, remark)
		}
		return buildSuiNaiveNativeLinks(settings, client.Email, client.Password, host, port, params, remark)
	default:
		return ""
	}
}

func isShadowrocketUserAgent(userAgent string) bool {
	return strings.Contains(strings.ToLower(userAgent), "shadowrocket")
}

// Shadowrocket's established SS+ShadowTLS representation stores the outer
// ShadowTLS endpoint/settings in a `shadow-tls=<base64 JSON>` query parameter.
// Sub-Store explicitly parses this form as Shadowrocket ShadowTLS. Do not feed
// Shadowrocket the SIP003 form: real-device testing showed it is imported as
// plain Shadowsocks (plugin=none) on the affected client version.
func buildShadowrocketShadowTLSLink(settings map[string]any, shadowPassword, host string, port int, remark string) string {
	method, _ := settings["innerMethod"].(string)
	if method == "" {
		method = "2022-blake3-aes-128-gcm"
	}
	innerPassword, _ := settings["innerPassword"].(string)
	handshake, _ := settings["handshakeServer"].(string)
	if innerPassword == "" || handshake == "" || shadowPassword == "" || host == "" || port < 1 || port > 65535 {
		return ""
	}
	endpoint := joinHostPort(host, port)
	legacyAuthority := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s@%s", method, innerPassword, endpoint)))
	descriptor, err := json.Marshal(map[string]any{
		"version":  "3",
		"password": shadowPassword,
		"host":     handshake,
		"address":  strings.Trim(host, "[]"),
		"port":     fmt.Sprintf("%d", port),
	})
	if err != nil {
		return ""
	}
	return buildLinkWithParams(
		"ss://"+legacyAuthority,
		map[string]string{"shadow-tls": base64.StdEncoding.EncodeToString(descriptor)},
		remark,
	)
}

// There is no universal standalone ShadowTLS share URI in sing-box/S-UI.
// Keep SIP003 only as the generic non-Shadowrocket representation for clients
// that explicitly implement the shadow-tls Shadowsocks plugin.
func buildShadowTLSSIP003Link(settings map[string]any, shadowPassword, host string, port int, remark string) string {
	method, _ := settings["innerMethod"].(string)
	if method == "" {
		method = "2022-blake3-aes-128-gcm"
	}
	innerPassword, _ := settings["innerPassword"].(string)
	handshake, _ := settings["handshakeServer"].(string)
	if innerPassword == "" || handshake == "" || shadowPassword == "" || host == "" || port < 1 || port > 65535 {
		return ""
	}
	endpoint := joinHostPort(host, port)
	userinfo := shadowsocksShareUserinfo(method, innerPassword)
	plugin := "shadow-tls;host=" + escapeSIP003Option(handshake) +
		";password=" + escapeSIP003Option(shadowPassword) + ";version=3"
	return buildLinkWithParams(
		fmt.Sprintf("ss://%s@%s/", userinfo, endpoint),
		map[string]string{"plugin": plugin},
		remark,
	)
}

func shadowsocksShareUserinfo(method, password string) string {
	if strings.HasPrefix(method, "2022") {
		return url.QueryEscape(method) + ":" + url.QueryEscape(password)
	}
	return base64.RawURLEncoding.EncodeToString([]byte(method + ":" + password))
}

func escapeSIP003Option(value string) string {
	return strings.NewReplacer(
		`\`, `\\`,
		`:`, `\:`,
		`;`, `\;`,
		`=`, `\=`,
	).Replace(value)
}

// S-UI's Naive link generator uses the same compatibility parameters for its
// http2:// and naive+https/naive+quic forms. In particular, SNI is named
// `peer`, padding is explicit, and tfo is always 0/1.
func applySuiNaiveLinkParams(settings map[string]any, params map[string]string) {
	if sni := params["sni"]; sni != "" {
		params["peer"] = sni
		delete(params, "sni")
	}
	params["padding"] = "1"
	if tfo, _ := settings["tcpFastOpen"].(bool); tfo {
		params["tfo"] = "1"
	} else {
		params["tfo"] = "0"
	}
}

func buildShadowrocketNaiveHTTP2Link(username, password, host string, port int, params map[string]string, remark string) string {
	if username == "" || password == "" || host == "" || port < 1 || port > 65535 {
		return ""
	}
	authority := fmt.Sprintf("%s:%s@%s", username, password, joinHostPort(host, port))
	encoded := base64.StdEncoding.EncodeToString([]byte(authority))
	return buildLinkWithParams("http2://"+encoded, params, remark)
}

func buildSuiNaiveNativeLinks(settings map[string]any, username, password, host string, port int, params map[string]string, remark string) string {
	network, _ := settings["network"].(string)
	schemes := []string{"naive+https", "naive+quic"}
	switch network {
	case "tcp":
		schemes = []string{"naive+https"}
	case "udp":
		schemes = []string{"naive+quic"}
	}
	links := make([]string, 0, len(schemes))
	for _, scheme := range schemes {
		base := fmt.Sprintf("%s://%s:%s@%s", scheme, encodeUserinfo(username), encodeUserinfo(password), joinHostPort(host, port))
		links = append(links, buildLinkWithParams(base, params, remark))
	}
	return strings.Join(links, "\n")
}

func applySingboxTLSLinkParams(settings, stream, ep map[string]any, params map[string]string) {
	if security, _ := stream["security"].(string); security == "reality" {
		applyNativeRealityLinkParams(stream, params)
	} else {
		tls, _ := stream["tlsSettings"].(map[string]any)
		selfSigned := isSupplementalSelfSignedTLS(settings, tls)
		if tls != nil {
			if sni, _ := tls["serverName"].(string); sni != "" {
				params["sni"] = sni
			}
			if alpn := anyStringSlice(tls["alpn"]); len(alpn) > 0 {
				params["alpn"] = strings.Join(alpn, ",")
			}
		}
		if selfSigned {
			if _, exists := params["sni"]; !exists {
				if sni, _ := settings["camouflageSNI"].(string); strings.TrimSpace(sni) != "" {
					params["sni"] = strings.TrimSpace(sni)
				}
			}
			params["insecure"] = "1"
		}
	}
	if ep != nil {
		if sni, ok := externalProxySNI(ep); ok {
			params["sni"] = sni
		}
		if insecure, _ := ep["allowInsecure"].(bool); insecure {
			params["insecure"] = "1"
		}
	}
}

func applyNativeRealityLinkParams(stream map[string]any, params map[string]string) {
	reality, _ := stream["realitySettings"].(map[string]any)
	if reality == nil {
		return
	}
	params["security"] = "reality"
	if names := anyStringSlice(reality["serverNames"]); len(names) > 0 {
		params["sni"] = names[0]
	}
	if ids := anyStringSlice(reality["shortIds"]); len(ids) > 0 {
		params["sid"] = ids[0]
	}
	client, _ := reality["settings"].(map[string]any)
	if client != nil {
		if publicKey, _ := client["publicKey"].(string); strings.TrimSpace(publicKey) != "" {
			params["pbk"] = strings.TrimSpace(publicKey)
		}
		if fingerprint, _ := client["fingerprint"].(string); strings.TrimSpace(fingerprint) != "" {
			params["fp"] = strings.TrimSpace(fingerprint)
		}
	}
}

func anyStringSlice(v any) []string {
	switch arr := v.(type) {
	case []any:
		out := make([]string, 0, len(arr))
		for _, item := range arr {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return arr
	default:
		return nil
	}
}

func intNumber(v any, fallback int) int {
	switch n := v.(type) {
	case float64:
		if n > 0 {
			return int(n)
		}
	case int:
		if n > 0 {
			return n
		}
	}
	return fallback
}
