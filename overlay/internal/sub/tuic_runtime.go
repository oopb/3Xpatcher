package sub

import "strings"

func nativeTUICServer(settings map[string]any) map[string]any {
	server, _ := settings["server"].(map[string]any)
	return server
}

func applyNativeTUICLinkSettings(settings map[string]any, params map[string]string) bool {
	server := nativeTUICServer(settings)
	if server == nil {
		return false
	}
	if sni, _ := server["sni"].(string); strings.TrimSpace(sni) != "" {
		params["sni"] = strings.TrimSpace(sni)
	}
	if cc, _ := server["congestion_control"].(string); strings.TrimSpace(cc) != "" {
		params["congestion_control"] = strings.TrimSpace(cc)
	}
	if zero, _ := server["zero_rtt_handshake"].(bool); zero {
		params["zero_rtt_handshake"] = "1"
	}
	if raw, ok := server["alpn"].([]any); ok {
		values := make([]string, 0, len(raw))
		for _, item := range raw {
			if value, ok := item.(string); ok && strings.TrimSpace(value) != "" {
				values = append(values, strings.TrimSpace(value))
			}
		}
		if len(values) > 0 {
			params["alpn"] = strings.Join(values, ",")
		}
	}
	return true
}

func applyNativeTUICClashSettings(settings map[string]any, proxy map[string]any) bool {
	server := nativeTUICServer(settings)
	if server == nil {
		return false
	}
	if sni, _ := server["sni"].(string); strings.TrimSpace(sni) != "" {
		proxy["sni"] = strings.TrimSpace(sni)
	}
	if cc, _ := server["congestion_control"].(string); strings.TrimSpace(cc) != "" {
		proxy["congestion-controller"] = strings.TrimSpace(cc)
	}
	if zero, _ := server["zero_rtt_handshake"].(bool); zero {
		proxy["reduce-rtt"] = true
	}
	if raw, ok := server["alpn"].([]any); ok {
		values := make([]string, 0, len(raw))
		for _, item := range raw {
			if value, ok := item.(string); ok && strings.TrimSpace(value) != "" {
				values = append(values, strings.TrimSpace(value))
			}
		}
		if len(values) > 0 {
			proxy["alpn"] = values
		}
	}
	if cert, _ := server["certificate"].(string); strings.Contains(cert, "/x-ui-singbox/certs/") {
		proxy["skip-cert-verify"] = true
	}
	return true
}
