package sub

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func (s *SubService) genSingboxLink(inbound *model.Inbound, email string) string {
	if !model.IsSingboxProtocol(inbound.Protocol) { return "" }
	client, ok := s.clientForLink(inbound, email); if !ok { return "" }
	settings := s.linkSettings(inbound)
	stream := unmarshalStreamSettings(inbound.StreamSettings)
	external, _ := stream["externalProxy"].([]any)
	if len(external) == 0 {
		return s.buildSingboxEndpointLink(inbound, client, settings, stream, nil, s.resolveInboundAddress(inbound), inbound.Port)
	}
	links := make([]string, 0, len(external))
	for _, raw := range external {
		ep, ok := raw.(map[string]any); if !ok { continue }
		dest, _ := ep["dest"].(string); port, ok := ep["port"].(float64); if dest == "" || !ok { continue }
		if link := s.buildSingboxEndpointLink(inbound, client, settings, stream, ep, dest, int(port)); link != "" { links = append(links, link) }
	}
	return strings.Join(links, "\n")
}

func (s *SubService) buildSingboxEndpointLink(inbound *model.Inbound, client model.Client, settings, stream, ep map[string]any, host string, port int) string {
	params := map[string]string{}
	transport := ""; if inbound.Protocol == model.TUIC { transport = "quic" }
	remark := s.endpointRemark(inbound, client.Email, ep, transport)
	applySingboxTLSLinkParams(settings, stream, ep, params)

	switch inbound.Protocol {
	case model.TUIC:
		if client.ID == "" || client.Password == "" { return "" }
		if v, _ := settings["congestionControl"].(string); v != "" { params["congestion_control"] = v }
		if v, _ := settings["zeroRTTHandshake"].(bool); v { params["zero_rtt_handshake"] = "1" }
		if v, _ := settings["heartbeat"].(string); v != "" { params["heartbeat"] = v }
		base := fmt.Sprintf("tuic://%s:%s@%s", encodeUserinfo(client.ID), encodeUserinfo(client.Password), joinHostPort(host, port))
		return buildLinkWithParams(base, params, remark)
	case model.AnyTLS:
		if client.Password == "" { return "" }
		return buildLinkWithParams(fmt.Sprintf("anytls://%s@%s", encodeUserinfo(client.Password), joinHostPort(host, port)), params, remark)
	case model.ShadowTLS:
		if client.Password == "" { return "" }
		params["version"] = "3"
		if v, _ := settings["handshakeServer"].(string); v != "" { params["sni"] = v; params["handshake"] = v + ":" + strconv.Itoa(intNumber(settings["handshakePort"], 443)) }
		if v, _ := settings["innerMethod"].(string); v != "" { params["inner_method"] = v }
		if v, _ := settings["innerPassword"].(string); v != "" { params["inner_password"] = v }
		return buildLinkWithParams(fmt.Sprintf("shadowtls://%s@%s", encodeUserinfo(client.Password), joinHostPort(host, port)), params, remark)
	case model.Naive:
		if client.Password == "" { return "" }
		return buildLinkWithParams(fmt.Sprintf("naive+https://%s:%s@%s", encodeUserinfo(client.Email), encodeUserinfo(client.Password), joinHostPort(host, port)), params, remark)
	default:
		return ""
	}
}

func applySingboxTLSLinkParams(settings, stream, ep map[string]any, params map[string]string) {
	tls, _ := stream["tlsSettings"].(map[string]any)
	selfSigned := false
	if tls != nil {
		if sni, _ := tls["serverName"].(string); sni != "" { params["sni"] = sni }
		if alpn := anyStringSlice(tls["alpn"]); len(alpn) > 0 { params["alpn"] = strings.Join(alpn, ",") }
		if mode, _ := tls["certificateMode"].(string); mode == "self_signed_sni" { selfSigned = true }
	}
	if !selfSigned {
		if mode, _ := settings["tlsMode"].(string); mode == "self_signed_sni" { // 0.6 compatibility
			selfSigned = true
			if _, exists := params["sni"]; !exists {
				if sni, _ := settings["camouflageSNI"].(string); strings.TrimSpace(sni) != "" { params["sni"] = strings.TrimSpace(sni) }
			}
		}
	}
	if selfSigned { params["insecure"] = "1" }
	if ep != nil {
		if sni, ok := externalProxySNI(ep); ok { params["sni"] = sni }
		if insecure, _ := ep["allowInsecure"].(bool); insecure { params["insecure"] = "1" }
	}
}

func anyStringSlice(v any) []string {
	arr, _ := v.([]any); out := make([]string, 0, len(arr))
	for _, item := range arr { if s, ok := item.(string); ok && s != "" { out = append(out, s) } }
	return out
}

func intNumber(v any, fallback int) int {
	switch n := v.(type) { case float64: if n > 0 { return int(n) }; case int: if n > 0 { return n } }
	return fallback
}
