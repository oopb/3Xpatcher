package model

import (
	"encoding/json"
	"strings"
)

type TUICRuntime string

const (
	TUICRuntimeNative  TUICRuntime = "native"
	TUICRuntimeSingbox TUICRuntime = "singbox"
)

// TUICRuntimeFromSettings selects which implementation owns a TUIC inbound.
// New rows explicitly persist settings.runtime. For compatibility, legacy
// 3Xpatcher camelCase TUIC settings are inferred as sing-box while the native
// 3x-ui nested server shape is inferred as native.
func TUICRuntimeFromSettings(raw string) TUICRuntime {
	var settings map[string]any
	if strings.TrimSpace(raw) != "" && json.Unmarshal([]byte(raw), &settings) == nil {
		if value, _ := settings["runtime"].(string); value == string(TUICRuntimeSingbox) {
			return TUICRuntimeSingbox
		} else if value == string(TUICRuntimeNative) {
			return TUICRuntimeNative
		}
		if _, ok := settings["server"]; ok {
			return TUICRuntimeNative
		}
		for _, key := range []string{"congestionControl", "authTimeout", "heartbeat", "idleTimeout", "keepAlivePeriod"} {
			if _, ok := settings[key]; ok {
				return TUICRuntimeSingbox
			}
		if _, ok := settings["congestion_control"]; ok {
			return TUICRuntimeNative
		}
	}
	return TUICRuntimeNative
}

func IsSingboxOwnedInbound(ib *Inbound) bool {
	if ib == nil {
		return false
	}
	if ib.Protocol == TUIC {
		return TUICRuntimeFromSettings(ib.Settings) == TUICRuntimeSingbox
	}
	return IsSingboxProtocol(ib.Protocol)
}
