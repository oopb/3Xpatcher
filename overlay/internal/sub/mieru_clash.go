package sub

import (
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func (s *SubClashService) buildMieruProxy(subReq *SubService, inbound *model.Inbound, client model.Client, ep map[string]any) map[string]any {
	if inbound.Protocol != model.Mieru || strings.TrimSpace(client.Password) == "" { return nil }
	settings := subReq.linkSettings(inbound)
	transport, _ := settings["transport"].(string); transport = strings.ToUpper(strings.TrimSpace(transport)); if transport == "" { transport = "TCP" }
	server := subReq.resolveInboundAddress(inbound); port := inbound.Port
	if ep != nil {
		if dest, _ := ep["dest"].(string); strings.TrimSpace(dest) != "" { server = dest }
		if p := intNumber(ep["port"], 0); p > 0 { port = p }
	}
	proxy := map[string]any{
		"name": subReq.endpointRemark(inbound, client.Email, ep, strings.ToLower(transport)),
		"type": "mieru", "server": server, "transport": transport, "udp": true,
		"username": client.Email, "password": client.Password,
	}
	if ep == nil {
		if end := intNumber(settings["portRangeEnd"], 0); end > port { proxy["port-range"] = strings.Join([]string{itoa(port), itoa(end)}, "-") } else { proxy["port"] = port }
	} else { proxy["port"] = port }
	if v, _ := settings["clientMultiplexing"].(string); strings.TrimSpace(v) != "" { proxy["multiplexing"] = v } else { proxy["multiplexing"] = "MULTIPLEXING_LOW" }
	if v, _ := settings["clientHandshakeMode"].(string); strings.TrimSpace(v) != "" { proxy["handshake-mode"] = v } else { proxy["handshake-mode"] = "HANDSHAKE_STANDARD" }
	if v, _ := settings["clientTrafficPattern"].(string); strings.TrimSpace(v) != "" { proxy["traffic-pattern"] = strings.TrimSpace(v) }
	return proxy
}

func itoa(v int) string {
	const digits = "0123456789"
	if v == 0 { return "0" }
	buf := [20]byte{}; i := len(buf)
	for v > 0 { i--; buf[i] = digits[v%10]; v /= 10 }
	return string(buf[i:])
}
