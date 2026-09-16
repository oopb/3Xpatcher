package singbox

import (
	"errors"
	"fmt"
	"strings"
)

// installTUICRuntimeTLS translates the native 3x-ui TUIC server shape into the
// canonical settings consumed by the sing-box renderer. Legacy 3Xpatcher TUIC
// rows keep using streamSettings and remain backward compatible.
func installTUICRuntimeTLS(settings map[string]any, rawStream string) error {
	server, _ := settings["server"].(map[string]any)
	if server == nil {
		return installTLSFromStream(settings, rawStream)
	}

	if v, _ := server["congestion_control"].(string); strings.TrimSpace(v) != "" {
		settings["congestionControl"] = v
	}
	if v, ok := server["zero_rtt_handshake"].(bool); ok {
		settings["zeroRTTHandshake"] = v
	}
	if seconds := intFromAny(server["authentication_timeout"]); seconds > 0 {
		settings["authTimeout"] = fmt.Sprintf("%ds", seconds)
	}

	cert, _ := server["certificate"].(string)
	key, _ := server["private_key"].(string)
	if strings.TrimSpace(cert) == "" || strings.TrimSpace(key) == "" {
		return errors.New("TUIC certificate and private key paths are required")
	}

	tls := map[string]any{
		"enabled":         true,
		"certificatePath": cert,
		"keyPath":         key,
	}
	if sni, _ := server["sni"].(string); strings.TrimSpace(sni) != "" {
		tls["serverName"] = sni
	}
	if alpn, ok := server["alpn"].([]any); ok && len(alpn) > 0 {
		values := make([]string, 0, len(alpn))
		for _, item := range alpn {
			if value, ok := item.(string); ok && strings.TrimSpace(value) != "" {
				values = append(values, value)
			}
		}
		if len(values) > 0 {
			tls["alpn"] = values
		}
	} else if alpn, ok := server["alpn"].([]string); ok && len(alpn) > 0 {
		tls["alpn"] = alpn
	}
	settings["tls"] = tls
	return nil
}

func intFromAny(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	default:
		return 0
	}
}
