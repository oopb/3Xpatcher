package model

import (
	"encoding/json"
	"strings"
)

// Supplemental protocols are stored in the same inbounds table and share the
// panel's ClientRecord/ClientInbound identity model. 3x-ui v3.8+ defines TUIC
// natively; 3Xpatcher keeps that native data shape and adds a per-inbound
// runtime selector so TUIC can be owned either by upstream tuic-server or by
// supplemental sing-box. Mieru remains isolated in official mita instances.
const (
	AnyTLS    Protocol = "anytls"
	ShadowTLS Protocol = "shadowtls"
	Naive     Protocol = "naive"
	Mieru     Protocol = "mieru"
)

type TUICRuntime string

const (
	TUICRuntimeNative  TUICRuntime = "native"
	TUICRuntimeSingbox TUICRuntime = "singbox"
)

// TUICRuntimeFromSettings is deliberately backward compatible:
//   - explicit settings.runtime always wins;
//   - upstream v3.8 TUIC rows contain settings.server and stay Native;
//   - legacy 3Xpatcher TUIC rows used the old camelCase sing-box shape and
//     automatically stay on sing-box after upgrade;
//   - an empty/unknown shape falls back to Native, matching upstream behavior.
func TUICRuntimeFromSettings(raw string) TUICRuntime {
	var settings map[string]any
	if strings.TrimSpace(raw) == "" || json.Unmarshal([]byte(raw), &settings) != nil || settings == nil {
		return TUICRuntimeNative
	}
	if runtime, _ := settings["runtime"].(string); runtime != "" {
		switch strings.ToLower(strings.TrimSpace(runtime)) {
		case string(TUICRuntimeSingbox):
			return TUICRuntimeSingbox
		case string(TUICRuntimeNative):
			return TUICRuntimeNative
		}
	}
	if _, ok := settings["server"]; ok {
		return TUICRuntimeNative
	}
	for _, key := range []string{
		"congestionControl", "authTimeout", "zeroRTTHandshake", "heartbeat",
		"idleTimeout", "keepAlivePeriod", "streamReceiveWindow",
		"connectionReceiveWindow", "maxConcurrentStreams", "initialPacketSize",
	} {
		if _, ok := settings[key]; ok {
			return TUICRuntimeSingbox
		}
	}
	return TUICRuntimeNative
}

func IsSingboxProtocol(p Protocol) bool {
	switch p {
	case TUIC, AnyTLS, ShadowTLS, Naive:
		return true
	default:
		return false
	}
}

func IsSingboxOwnedInbound(inbound *Inbound) bool {
	if inbound == nil {
		return false
	}
	if inbound.Protocol == TUIC {
		return TUICRuntimeFromSettings(inbound.Settings) == TUICRuntimeSingbox
	}
	return IsSingboxProtocol(inbound.Protocol)
}

func IsMieruProtocol(p Protocol) bool { return p == Mieru }
func IsSupplementalProtocol(p Protocol) bool { return IsSingboxProtocol(p) || IsMieruProtocol(p) }

func IsSingboxTLSProtocol(p Protocol) bool { return p == TUIC || p == AnyTLS || p == Naive }
