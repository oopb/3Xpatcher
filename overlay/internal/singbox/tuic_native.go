package singbox

import (
	"errors"
	"fmt"
	"strings"
)

// installTUICRuntimeTLS translates the native 3x-ui TUIC server block into the
// sing-box TUIC settings shape. This lets the Runtime selector reuse exactly
// one TUIC form/data model. Legacy 3Xpatcher TUIC rows without settings.server
// keep using the old streamSettings TLS adapter for backward compatibility.
func installTUICRuntimeTLS(settings map[string]any, rawStream string) error {
	server, _ := settings["server"].(map[string]any)
	if server == nil {
		return installTLSFromStream(settings, rawStream)
	}

	copyString := func(src, dst string) {
		if v, ok := server[src].(string); ok && strings.TrimSpace(v) != "" {
			settings[dst] = strings.TrimSpace(v)
		}
	}
	copyBool := func(src, dst string) {
		if v, ok := server[src].(bool); ok {
			settings[dst] = v
		}
	}

	copyString("congestion_control", "congestionControl")
	copyBool("zero_rtt_handshake", "zeroRTTHandshake")

	if seconds := intNumber(server["authentication_timeout"], 0); seconds > 0 {
		settings["authTimeout"] = fmt.Sprintf("%ds", seconds)
	}
	if seconds := intNumber(server["max_idle_time"], 0); seconds > 0 {
		settings["idleTimeout"] = fmt.Sprintf("%ds", seconds)
	}

	cert, _ := server["certificate"].(string)
	key, _ := server["private_key"].(string)
	cert = strings.TrimSpace(cert)
	key = strings.TrimSpace(key)
	if cert == "" || key == "" {
		return errors.New("TUIC certificate and private key are required")
	}

	tls := map[string]any{
		"enabled":         true,
		"certificatePath": cert,
		"keyPath":         key,
	}
	if sni, _ := server["sni"].(string); strings.TrimSpace(sni) != "" {
		tls["serverName"] = strings.TrimSpace(sni)
	}
	if alpn := stringSlice(server["alpn"]); len(alpn) > 0 {
		tls["alpn"] = alpn
	}
	settings["tls"] = tls
	return nil
}
