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
		selfSigned := false
		if tls != nil {
			if sni, _ := tls["serverName"].(string); sni != "" {
				proxy["sni"] = sni
			}
			if alpn := anyStringSlice(tls["alpn"]); len(alpn) > 0 {
				proxy["alpn"] = alpn
			}
			if mode, _ := tls["certificateMode"].(string); mode == "self_signed_sni" {
				selfSigned = true
			}
		}
		if !selfSigned {
			if mode, _ := settings["tlsMode"].(string); mode == "self_signed_sni" { // 0.6 compatibility
				selfSigned = true
				if _, exists := proxy["sni"]; !exists {
					if sni, _ := settings["camouflageSNI"].(string); strings.TrimSpace(sni) != "" {
						proxy["sni"] = strings.TrimSpace(sni)
					}
				}
			}
		}
		if selfSigned {
			proxy["skip-cert-verify"] = true
		}
		if ep != nil {
			if sni, ok := externalProxySNI(ep); ok {
				proxy["sni"] = sni
			}
			if insecure, _ := ep["allowInsecure"].(bool); insecure {
				proxy["skip-cert-verify"] = true
			}
		}
	}

	switch inbound.Protocol {
	case model.TUIC:
		if client.ID == "" || client.Password == "" {
			return nil
		}
		proxy["type"] = "tuic"
		proxy["uuid"] = client.ID
		proxy["password"] = client.Password
		if v, _ := settings["congestionControl"].(string); v != "" {
			proxy["congestion-controller"] = v
		}
		if v, _ := settings["zeroRTTHandshake"].(bool); v {
			proxy["reduce-rtt"] = true
		}
		// Keep the exact pre-Mieru TUIC field shape (including udp:true and the
		// inbound's SNI/ALPN). Real Clash Verge A/B testing isolated the actual
		// regression to certificate verification: these supplemental TUIC
		// profiles must skip verification for their camouflage/self-signed TLS.
		applyTLS()
		proxy["skip-cert-verify"] = true
		return proxy
	case model.AnyTLS:
		if client.Password == "" {
			return nil
		}
		if security, _ := stream["security"].(string); security == "reality" {
			return nil
		}
		proxy["type"] = "anytls"
		proxy["password"] = client.Password
		applyTLS()
		return proxy
	case model.ShadowTLS:
		innerMethod, _ := settings["innerMethod"].(string)
		innerPassword, _ := settings["innerPassword"].(string)
		handshake, _ := settings["handshakeServer"].(string)
		if client.Password == "" || innerMethod == "" || innerPassword == "" {
			return nil
		}
		proxy["type"] = "ss"
		proxy["cipher"] = innerMethod
		proxy["password"] = innerPassword
		proxy["plugin"] = "shadow-tls"
		proxy["client-fingerprint"] = "chrome"
		pluginOpts := map[string]any{"version": 3, "password": client.Password}
		if handshake != "" {
			pluginOpts["host"] = handshake
		}
		proxy["plugin-opts"] = pluginOpts
		return proxy
	case model.Naive:
		return nil
	default:
		return nil
	}
}
