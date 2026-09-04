package sub

import (
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func isSupplementalSelfSignedTLS(settings, tls map[string]any) bool {
	if tls != nil {
		if mode, _ := tls["certificateMode"].(string); mode == "self_signed_sni" {
			return true
		}
		// Real persisted 3Xpatcher self-signed state can outlive the UI-only
		// certificateMode marker on upgraded/historical rows. The generated
		// certificate metadata is authoritative: these fields are written only
		// by the supplemental SNI certificate flow and identify a certificate
		// that no public CA can validate on the client.
		certPath, _ := tls["selfSignedCertificatePath"].(string)
		keyPath, _ := tls["selfSignedKeyPath"].(string)
		if strings.TrimSpace(certPath) != "" && strings.TrimSpace(keyPath) != "" {
			return true
		}
	}
	if mode, _ := settings["tlsMode"].(string); mode == "self_signed_sni" { // 0.6 compatibility
		return true
	}
	return false
}

func (s *SubClashService) buildSingboxProxy(subReq *SubService, inbound *model.Inbound, client model.Client, stream map[string]any, ep map[string]any) map[string]any {
	settings := subReq.linkSettings(inbound)
	name := subReq.endpointRemark(inbound, client.Email, ep, "")
	proxy := map[string]any{"name": name, "server": inbound.Listen, "port": inbound.Port, "udp": true}
	tls, _ := stream["tlsSettings"].(map[string]any)
	applyTLS := func() {
		selfSigned := isSupplementalSelfSignedTLS(settings, tls)
		if tls != nil {
			if sni, _ := tls["serverName"].(string); sni != "" {
				proxy["sni"] = sni
			}
			if alpn := anyStringSlice(tls["alpn"]); len(alpn) > 0 {
				proxy["alpn"] = alpn
			}
		}
		if selfSigned {
			if _, exists := proxy["sni"]; !exists {
				if sni, _ := settings["camouflageSNI"].(string); strings.TrimSpace(sni) != "" {
					proxy["sni"] = strings.TrimSpace(sni)
				}
			}
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
		// Mihomo enables UDP for TUIC by protocol default. Match a known-good
		// Clash Verge TUIC profile instead of carrying the generic udp flag, and
		// make the TUIC-v5 relay mode explicit rather than relying on client
		// defaults that have changed across Clash.Meta/Mihomo generations.
		delete(proxy, "udp")
		proxy["udp-relay-mode"] = "native"
		if v, _ := settings["congestionControl"].(string); v != "" {
			proxy["congestion-controller"] = v
		}
		if v, _ := settings["zeroRTTHandshake"].(bool); v {
			proxy["reduce-rtt"] = true
		}
		// Preserve the inbound's configured TLS values exactly. In particular,
		// do not collapse a working ordered ALPN list such as h3,h2,http/1.1 to
		// an invented h3-only value. Generated self-signed certificates always
		// require skip-cert-verify on the Mihomo client, even when an upgraded
		// row no longer carries the certificateMode marker.
		applyTLS()
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
