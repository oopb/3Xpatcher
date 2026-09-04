package sub

import (
	"strings"
	"time"

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
		// Mihomo's TUIC client uses milliseconds for heartbeat-interval, while
		// the sing-box inbound UI stores Go duration strings such as "10s".
		if ms := mihomoDurationMilliseconds(settings["heartbeat"]); ms > 0 {
			proxy["heartbeat-interval"] = ms
		}
		// Keep Mihomo on TUIC v5's native datagram relay path explicitly. This
		// avoids client-version dependent defaults in Clash Verge.
		proxy["udp-relay-mode"] = "native"
		if streams := intNumber(settings["maxConcurrentStreams"], 0); streams > 0 {
			proxy["max-open-streams"] = streams
		}
		if disabled, _ := settings["disablePathMTUDiscovery"].(bool); disabled {
			proxy["disable-mtu-discovery"] = true
		}
		applyTLS()
		// When the panel intentionally has no SNI, tell Mihomo not to derive an
		// IP/domain SNI from `server`; sing-box TUIC accepts an empty SNI in this
		// configuration and this matches Mihomo's documented TUIC knob.
		if _, hasSNI := proxy["sni"]; !hasSNI {
			proxy["disable-sni"] = true
		}
		return proxy
	case model.AnyTLS:
		if client.Password == "" {
			return nil
		}
		// sing-box 1.14 supports AnyTLS+Reality end-to-end. Current Mihomo
		// explicitly does not support that combination, so omit it rather than
		// exporting a proxy that can never connect.
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
		// Mihomo models standalone ShadowTLS as a Shadowsocks proxy with the
		// built-in shadow-tls plugin. Its current documented example also uses
		// a client uTLS fingerprint; keeping it explicit avoids client-version
		// dependent defaults in Clash Verge / Mihomo.
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
		// Mihomo has no Naive proxy type. Do not manufacture an HTTPS proxy:
		// it would import successfully but speak a different protocol on wire.
		return nil
	default:
		return nil
	}
}

func mihomoDurationMilliseconds(value any) int {
	s, _ := value.(string)
	if strings.TrimSpace(s) == "" {
		return 0
	}
	d, err := time.ParseDuration(strings.TrimSpace(s))
	if err != nil || d <= 0 {
		return 0
	}
	ms := d / time.Millisecond
	if ms <= 0 {
		return 0
	}
	return int(ms)
}
