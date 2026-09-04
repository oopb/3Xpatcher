package sub

import (
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func (s *SubService) genMieruLink(inbound *model.Inbound, email string) string {
	if inbound.Protocol != model.Mieru {
		return ""
	}
	client, ok := s.clientForLink(inbound, email)
	if !ok || strings.TrimSpace(client.Password) == "" {
		return ""
	}
	settings := s.linkSettings(inbound)
	stream := unmarshalStreamSettings(inbound.StreamSettings)
	external, _ := stream["externalProxy"].([]any)
	if len(external) == 0 {
		return s.buildMieruSimpleLink(inbound, client, settings, nil, s.resolveInboundAddress(inbound), inbound.Port)
	}
	links := make([]string, 0, len(external))
	for _, raw := range external {
		ep, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		dest, _ := ep["dest"].(string)
		port := intNumber(ep["port"], 0)
		if strings.TrimSpace(dest) == "" || port <= 0 {
			continue
		}
		if link := s.buildMieruSimpleLink(inbound, client, settings, ep, dest, port); link != "" {
			links = append(links, link)
		}
	}
	return strings.Join(links, "\n")
}

func (s *SubService) buildMieruSimpleLink(inbound *model.Inbound, client model.Client, settings map[string]any, ep map[string]any, host string, port int) string {
	mtu := intNumber(settings["mtu"], 1400)
	multiplexing, _ := settings["clientMultiplexing"].(string)
	if multiplexing == "" {
		multiplexing = "MULTIPLEXING_LOW"
	}
	handshake, _ := settings["clientHandshakeMode"].(string)
	if handshake == "" {
		handshake = "HANDSHAKE_STANDARD"
	}
	profile := strings.TrimSpace(inbound.Remark)
	if profile == "" {
		profile = "default"
	}
	q := url.Values{}
	q.Set("profile", profile)
	q.Set("mtu", strconv.Itoa(mtu))
	q.Set("multiplexing", multiplexing)
	q.Set("handshake-mode", handshake)
	if traffic, _ := settings["clientTrafficPattern"].(string); strings.TrimSpace(traffic) != "" {
		q.Set("traffic-pattern", strings.TrimSpace(traffic))
	}

	if ep == nil {
		for _, binding := range mieruShareBindings(inbound, settings) {
			if binding.RangeEnd > binding.Port {
				q.Add("port", strconv.Itoa(binding.Port)+"-"+strconv.Itoa(binding.RangeEnd))
			} else {
				q.Add("port", strconv.Itoa(binding.Port))
			}
			q.Add("protocol", binding.Transport)
		}
	} else {
		transport := normalizedMieruTransport(settings["transport"])
		q.Add("port", strconv.Itoa(port))
		q.Add("protocol", transport)
	}

	urlHost := host
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil && strings.Contains(ip.String(), ":") {
		urlHost = "[" + ip.String() + "]"
	}
	u := url.URL{Scheme: "mierus", User: url.UserPassword(client.Email, client.Password), Host: urlHost, RawQuery: q.Encode()}
	u.Fragment = s.endpointRemark(inbound, client.Email, ep, "mieru")
	return u.String()
}

type mieruShareBinding struct {
	Port      int
	RangeEnd  int
	Transport string
}

func normalizedMieruTransport(v any) string {
	transport, _ := v.(string)
	transport = strings.ToUpper(strings.TrimSpace(transport))
	if transport != "UDP" {
		return "TCP"
	}
	return "UDP"
}

func mieruShareBindings(inbound *model.Inbound, settings map[string]any) []mieruShareBinding {
	primary := mieruShareBinding{
		Port:      inbound.Port,
		RangeEnd:  intNumber(settings["portRangeEnd"], 0),
		Transport: normalizedMieruTransport(settings["transport"]),
	}
	out := []mieruShareBinding{primary}
	extra, _ := settings["additionalPortBindings"].([]any)
	for _, raw := range extra {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		port := intNumber(m["port"], 0)
		if port <= 0 {
			continue
		}
		out = append(out, mieruShareBinding{
			Port:      port,
			RangeEnd:  intNumber(m["portRangeEnd"], 0),
			Transport: normalizedMieruTransport(m["transport"]),
		})
	}
	return out
}
