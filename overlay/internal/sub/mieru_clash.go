package sub

import (
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func (s *SubClashService) buildMieruProxy(subReq *SubService, inbound *model.Inbound, client model.Client, ep map[string]any) map[string]any {
	if inbound.Protocol != model.Mieru || strings.TrimSpace(client.Password) == "" {
		return nil
	}
	settings := subReq.linkSettings(inbound)
	binding := mieruShareBindings(inbound, settings)[0]
	if ep != nil {
		if p := intNumber(ep["port"], 0); p > 0 {
			binding.Port = p
			binding.RangeEnd = 0
		}
		binding.Transport = normalizedMieruTransport(settings["transport"])
	}
	return s.buildMieruProxyForBinding(subReq, inbound, client, ep, settings, binding, false)
}

func (s *SubClashService) buildMieruProxies(subReq *SubService, inbound *model.Inbound, client model.Client) []map[string]any {
	if inbound.Protocol != model.Mieru || strings.TrimSpace(client.Password) == "" {
		return nil
	}
	settings := subReq.linkSettings(inbound)
	bindings := mieruShareBindings(inbound, settings)
	out := make([]map[string]any, 0, len(bindings))
	for _, binding := range bindings {
		if proxy := s.buildMieruProxyForBinding(subReq, inbound, client, nil, settings, binding, len(bindings) > 1); proxy != nil {
			out = append(out, proxy)
		}
	}
	return out
}

func (s *SubClashService) buildMieruProxyForBinding(
	subReq *SubService,
	inbound *model.Inbound,
	client model.Client,
	ep map[string]any,
	settings map[string]any,
	binding mieruShareBinding,
	labelBinding bool,
) map[string]any {
	server := subReq.resolveInboundAddress(inbound)
	if ep != nil {
		if dest, _ := ep["dest"].(string); strings.TrimSpace(dest) != "" {
			server = dest
		}
	}
	name := subReq.endpointRemark(inbound, client.Email, ep, strings.ToLower(binding.Transport))
	if labelBinding {
		spec := itoa(binding.Port)
		if binding.RangeEnd > binding.Port {
			spec += "-" + itoa(binding.RangeEnd)
		}
		name += " [" + binding.Transport + " " + spec + "]"
	}
	proxy := map[string]any{
		"name":           name,
		"type":           "mieru",
		"server":         server,
		"transport":      binding.Transport,
		"udp":            true,
		"username":       client.Email,
		"password":       client.Password,
		"multiplexing":   "MULTIPLEXING_LOW",
		"handshake-mode": "HANDSHAKE_STANDARD",
	}
	if binding.RangeEnd > binding.Port {
		proxy["port-range"] = itoa(binding.Port) + "-" + itoa(binding.RangeEnd)
	} else {
		proxy["port"] = binding.Port
	}
	if v, _ := settings["clientMultiplexing"].(string); strings.TrimSpace(v) != "" {
		proxy["multiplexing"] = v
	}
	if v, _ := settings["clientHandshakeMode"].(string); strings.TrimSpace(v) != "" {
		proxy["handshake-mode"] = v
	}
	if v, _ := settings["clientTrafficPattern"].(string); strings.TrimSpace(v) != "" {
		proxy["traffic-pattern"] = strings.TrimSpace(v)
	}
	return proxy
}

func itoa(v int) string {
	const digits = "0123456789"
	if v == 0 {
		return "0"
	}
	buf := [20]byte{}
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = digits[v%10]
		v /= 10
	}
	return string(buf[i:])
}
