package sub

import (
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func (s *SubService) genMieruLink(inbound *model.Inbound, email string) string {
	if inbound.Protocol != model.Mieru { return "" }
	client, ok := s.clientForLink(inbound, email)
	if !ok || strings.TrimSpace(client.Password) == "" { return "" }
	settings := s.linkSettings(inbound)
	stream := unmarshalStreamSettings(inbound.StreamSettings)
	external, _ := stream["externalProxy"].([]any)
	if len(external) == 0 {
		return s.buildMieruSimpleLink(inbound, client, settings, nil, s.resolveInboundAddress(inbound), inbound.Port)
	}
	links := make([]string, 0, len(external))
	for _, raw := range external {
		ep, ok := raw.(map[string]any); if !ok { continue }
		dest, _ := ep["dest"].(string); port := intNumber(ep["port"], 0)
		if strings.TrimSpace(dest) == "" || port <= 0 { continue }
		if link := s.buildMieruSimpleLink(inbound, client, settings, ep, dest, port); link != "" { links = append(links, link) }
	}
	return strings.Join(links, "\n")
}

func (s *SubService) buildMieruSimpleLink(inbound *model.Inbound, client model.Client, settings map[string]any, ep map[string]any, host string, port int) string {
	transport, _ := settings["transport"].(string); transport = strings.ToUpper(strings.TrimSpace(transport)); if transport == "" { transport = "TCP" }
	mtu := intNumber(settings["mtu"], 1400)
	multiplexing, _ := settings["clientMultiplexing"].(string); if multiplexing == "" { multiplexing = "MULTIPLEXING_LOW" }
	handshake, _ := settings["clientHandshakeMode"].(string); if handshake == "" { handshake = "HANDSHAKE_STANDARD" }
	profile := strings.TrimSpace(inbound.Remark); if profile == "" { profile = "default" }
	q := url.Values{}
	q.Set("profile", profile); q.Set("mtu", strconv.Itoa(mtu)); q.Set("multiplexing", multiplexing); q.Set("handshake-mode", handshake)
	if traffic, _ := settings["clientTrafficPattern"].(string); strings.TrimSpace(traffic) != "" { q.Set("traffic-pattern", strings.TrimSpace(traffic)) }
	if ep == nil {
		if end := intNumber(settings["portRangeEnd"], 0); end > port { q.Add("port", strconv.Itoa(port)+"-"+strconv.Itoa(end)) } else { q.Add("port", strconv.Itoa(port)) }
	} else { q.Add("port", strconv.Itoa(port)) }
	q.Add("protocol", transport)
	urlHost := host
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil && strings.Contains(ip.String(), ":") { urlHost = "[" + ip.String() + "]" }
	u := url.URL{Scheme: "mierus", User: url.UserPassword(client.Email, client.Password), Host: urlHost, RawQuery: q.Encode()}
	u.Fragment = s.endpointRemark(inbound, client.Email, ep, strings.ToLower(transport))
	return u.String()
}
