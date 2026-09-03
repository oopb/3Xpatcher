package singbox

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// installTLSOrRealityFromStream reuses the native 3x-ui stream security model.
// For supplemental protocols that sing-box can actually run with Reality,
// security=reality is translated into sing-box InboundTLSOptions.Reality.
func installTLSOrRealityFromStream(settings map[string]any, raw string) error {
	var stream map[string]any
	if err := json.Unmarshal([]byte(raw), &stream); err != nil {
		return fmt.Errorf("invalid stream settings: %w", err)
	}
	if stream == nil {
		return errors.New("stream settings are missing")
	}
	security, _ := stream["security"].(string)
	if security != "reality" {
		return installTLSFromStream(settings, raw)
	}
	realityIn, _ := stream["realitySettings"].(map[string]any)
	if realityIn == nil {
		return errors.New("Reality settings are missing")
	}
	return installRealityFromNative3xui(settings, realityIn)
}

func installRealityFromNative3xui(settings map[string]any, realityIn map[string]any) error {
	target, _ := realityIn["target"].(string)
	host, port, err := splitRealityTarget(strings.TrimSpace(target))
	if err != nil {
		return err
	}
	serverNames := stringSlice(realityIn["serverNames"])
	if len(serverNames) == 0 || strings.TrimSpace(serverNames[0]) == "" {
		return errors.New("Reality SNI/serverNames is required")
	}
	serverName := strings.TrimSpace(serverNames[0])
	privateKey, _ := realityIn["privateKey"].(string)
	privateKey = strings.TrimSpace(privateKey)
	if privateKey == "" {
		return errors.New("Reality private key is required")
	}
	shortIDs := stringSlice(realityIn["shortIds"])

	realityOut := map[string]any{
		"enabled":         true,
		"handshakeServer": host,
		"handshakePort":   port,
		"privateKey":      privateKey,
	}
	if len(shortIDs) > 0 {
		realityOut["shortIds"] = shortIDs
	}
	if maxTimediff := integerValue(realityIn["maxTimediff"]); maxTimediff > 0 {
		realityOut["maxTimeDifference"] = strconv.FormatInt(maxTimediff, 10) + "ms"
	}

	settings["tls"] = map[string]any{
		"enabled":    true,
		"serverName": serverName,
		"reality":    realityOut,
	}
	return nil
}

func splitRealityTarget(target string) (string, int, error) {
	if target == "" {
		return "", 0, errors.New("Reality target is required")
	}
	if host, portText, err := net.SplitHostPort(target); err == nil {
		port, err := strconv.Atoi(portText)
		if err != nil || port < 1 || port > 65535 {
			return "", 0, errors.New("invalid Reality target port")
		}
		if strings.TrimSpace(host) == "" {
			return "", 0, errors.New("Reality target host is required")
		}
		return host, port, nil
	}
	// Native 3x-ui normally stores host:port. Accept a bare hostname too so
	// hand-edited API rows remain usable, matching Reality's conventional 443.
	if !strings.Contains(target, ":") {
		return target, 443, nil
	}
	return "", 0, fmt.Errorf("invalid Reality target %q; use host:port", target)
}

func integerValue(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case float32:
		return int64(n)
	case int:
		return int64(n)
	case int64:
		return n
	case json.Number:
		i, _ := n.Int64()
		return i
	default:
		return 0
	}
}
