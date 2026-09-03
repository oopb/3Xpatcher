package sub

import (
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func (s *SubClashService) buildSingboxProxy(subReq *SubService, inbound *model.Inbound, client model.Client, stream map[string]any, ep map[string]any) map[string]any {
	settings := subReq.linkSettings(inbound)
	name := subReq.endpointRemark(inbound, client.Email, ep, "")
	proxy := map[string]any{"name": name, "server": inbound.Listen, "port": inbound.Port, "udp": true}
	tls, _ := stream["tlsSettings"].(map[string]any)
	applyTLS := func() {
		if tls != nil {
			if sni, _ := tls["serverName"].(string); sni != "" { proxy["sni"] = sni }
			if alpn := anyStringSlice(tls["alpn"]); len(alpn) > 0 { proxy["alpn"] = alpn }
		}
		if mode, _ := settings["tlsMode"].(string); mode == "self_signed_sni" {
			if sni, _ := settings["camouflageSNI"].(string); strings.TrimSpace(sni) != "" { proxy["sni"] = strings.TrimSpace(sni) }
			proxy["skip-cert-verify"] = true
		}
		if ep != nil {
			if sni, ok := externalProxySNI(ep); ok { proxy["sni"] = sni }
			if insecure, _ := ep["allowInsecure"].(bool); insecure { proxy["skip-cert-verify"] = true }
		}
	}

	switch inbound.Protocol {
	case model.TUIC:
		if client.ID == "" || client.Password == "" { return nil }
		proxy["type"] = "tuic"; proxy["uuid"] = client.ID; proxy["password"] = client.Password
		if v, _ := settings["congestionControl"].(string); v != "" { proxy["congestion-controller"] = v }
		if v, _ := settings["zeroRTTHandshake"].(bool); v { proxy["reduce-rtt"] = true }
		applyTLS(); return proxy
	case model.AnyTLS:
		if client.Password == "" { return nil }
		proxy["type"] = "anytls"; proxy["password"] = client.Password; applyTLS(); return proxy
	case model.ShadowTLS:
		innerMethod, _ := settings["innerMethod"].(string); innerPassword, _ := settings["innerPassword"].(string); handshake, _ := settings["handshakeServer"].(string)
		if client.Password == "" || innerMethod == "" || innerPassword == "" { return nil }
		proxy["type"] = "ss"; proxy["cipher"] = innerMethod; proxy["password"] = innerPassword; proxy["plugin"] = "shadow-tls"
		pluginOpts := map[string]any{"version": 3, "password": client.Password}; if handshake != "" { pluginOpts["host"] = handshake }; proxy["plugin-opts"] = pluginOpts; return proxy
	case model.Naive:
		return nil
	default:
		return nil
	}
}
