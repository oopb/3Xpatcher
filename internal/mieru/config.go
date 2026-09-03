package mieru

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type User struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type Settings struct {
	Transport              string   `json:"transport,omitempty"`
	PortRangeEnd           int      `json:"portRangeEnd,omitempty"`
	MTU                    int      `json:"mtu,omitempty"`
	LoggingLevel           string   `json:"loggingLevel,omitempty"`
	AllowPrivateIP         bool     `json:"allowPrivateIP,omitempty"`
	AllowLoopbackIP        bool     `json:"allowLoopbackIP,omitempty"`
	QuotaDays              int      `json:"quotaDays,omitempty"`
	QuotaMegabytes         int      `json:"quotaMegabytes,omitempty"`
	MetricsLoggingInterval string   `json:"metricsLoggingInterval,omitempty"`
	UserHintIsMandatory    bool     `json:"userHintIsMandatory,omitempty"`
	TrafficPatternEnabled  bool     `json:"trafficPatternEnabled,omitempty"`
	TrafficSeed            int      `json:"trafficSeed,omitempty"`
	TrafficUnlockAll       bool     `json:"trafficUnlockAll,omitempty"`
	TCPFragmentEnable      bool     `json:"tcpFragmentEnable,omitempty"`
	TCPFragmentMaxSleepMs  int      `json:"tcpFragmentMaxSleepMs,omitempty"`
	NonceType              string   `json:"nonceType,omitempty"`
	NonceApplyToAllUDP     bool     `json:"nonceApplyToAllUDP,omitempty"`
	NonceMinLen            int      `json:"nonceMinLen,omitempty"`
	NonceMaxLen            int      `json:"nonceMaxLen,omitempty"`
	NonceCustomHexStrings  []string `json:"nonceCustomHexStrings,omitempty"`
	PaddingMaxMiddleLen    *int     `json:"paddingMaxMiddleLen,omitempty"`
	PaddingMaxEndLen       *int     `json:"paddingMaxEndLen,omitempty"`
	LowEntropyMode         string   `json:"lowEntropyMode,omitempty"`
	LowEntropyMaskRotation string   `json:"lowEntropyMaskRotation,omitempty"`
	ClientMultiplexing     string   `json:"clientMultiplexing,omitempty"`
	ClientHandshakeMode    string   `json:"clientHandshakeMode,omitempty"`
	ClientTrafficPattern   string   `json:"clientTrafficPattern,omitempty"`
}

type Record struct {
	ID       int
	Remark   string
	Port     int
	Settings Settings
	Users    []User
}

func BuildServerConfig(r Record) ([]byte, error) {
	if r.ID <= 0 { return nil, errors.New("inbound id is required") }
	if r.Port < 1025 || r.Port > 65535 { return nil, fmt.Errorf("Mieru port %d is outside 1025..65535", r.Port) }
	if len(r.Users) == 0 { return nil, errors.New("at least one Mieru user is required") }
	for _, u := range r.Users {
		if strings.TrimSpace(u.Name) == "" || strings.TrimSpace(u.Password) == "" { return nil, errors.New("Mieru user name and password are required") }
	}
	transport := strings.ToUpper(strings.TrimSpace(r.Settings.Transport))
	if transport == "" { transport = "TCP" }
	if transport != "TCP" && transport != "UDP" { return nil, fmt.Errorf("invalid Mieru transport %q", r.Settings.Transport) }
	binding := map[string]any{"protocol": transport}
	if r.Settings.PortRangeEnd > 0 {
		if r.Settings.PortRangeEnd < r.Port || r.Settings.PortRangeEnd > 65535 { return nil, fmt.Errorf("Mieru port range %d-%d is invalid", r.Port, r.Settings.PortRangeEnd) }
		binding["portRange"] = fmt.Sprintf("%d-%d", r.Port, r.Settings.PortRangeEnd)
	} else { binding["port"] = r.Port }
	users := make([]map[string]any, 0, len(r.Users))
	for _, u := range r.Users {
		digest := sha256.Sum256([]byte(u.Password + "\x00" + u.Name))
		m := map[string]any{"name": u.Name, "hashedPassword": hex.EncodeToString(digest[:])}
		if r.Settings.AllowPrivateIP { m["allowPrivateIP"] = true }
		if r.Settings.AllowLoopbackIP { m["allowLoopbackIP"] = true }
		if r.Settings.QuotaDays > 0 || r.Settings.QuotaMegabytes > 0 {
			if r.Settings.QuotaDays <= 0 || r.Settings.QuotaMegabytes <= 0 { return nil, errors.New("Mieru quota days and megabytes must be set together") }
			m["quotas"] = []map[string]any{{"days": r.Settings.QuotaDays, "megabytes": r.Settings.QuotaMegabytes}}
		}
		users = append(users, m)
	}
	mtu := r.Settings.MTU
	if mtu == 0 { mtu = 1400 }
	if mtu < 1280 || mtu > 65535 { return nil, fmt.Errorf("invalid Mieru MTU %d", mtu) }
	logging := strings.ToUpper(strings.TrimSpace(r.Settings.LoggingLevel))
	if logging == "" { logging = "INFO" }
	switch logging { case "FATAL", "ERROR", "WARN", "INFO", "DEBUG", "TRACE": default: return nil, fmt.Errorf("invalid Mieru logging level %q", logging) }
	root := map[string]any{"portBindings": []any{binding}, "users": users, "loggingLevel": logging, "mtu": mtu}
	advanced := map[string]any{}
	if strings.TrimSpace(r.Settings.MetricsLoggingInterval) != "" { advanced["metricsLoggingInterval"] = strings.TrimSpace(r.Settings.MetricsLoggingInterval) }
	if r.Settings.UserHintIsMandatory { advanced["userHintIsMandatory"] = true }
	if len(advanced) > 0 { root["advancedSettings"] = advanced }
	if r.Settings.TrafficPatternEnabled {
		traffic, err := buildTrafficPattern(r.Settings); if err != nil { return nil, err }; root["trafficPattern"] = traffic
	}
	return json.MarshalIndent(root, "", "  ")
}

func buildTrafficPattern(s Settings) (map[string]any, error) {
	m := map[string]any{}
	if s.TrafficSeed != 0 { m["seed"] = s.TrafficSeed }
	if s.TrafficUnlockAll { m["unlockAll"] = true }
	if s.TCPFragmentEnable || s.TCPFragmentMaxSleepMs != 0 {
		if s.TCPFragmentMaxSleepMs < 0 || s.TCPFragmentMaxSleepMs > 100 { return nil, errors.New("Mieru tcpFragment maxSleepMs must be 0..100") }
		t := map[string]any{"enable": s.TCPFragmentEnable}; if s.TCPFragmentMaxSleepMs > 0 { t["maxSleepMs"] = s.TCPFragmentMaxSleepMs }; m["tcpFragment"] = t
	}
	if strings.TrimSpace(s.NonceType) != "" {
		typeName := strings.ToUpper(strings.TrimSpace(s.NonceType))
		switch typeName { case "NONCE_TYPE_RANDOM", "NONCE_TYPE_PRINTABLE", "NONCE_TYPE_PRINTABLE_SUBSET", "NONCE_TYPE_FIXED": default: return nil, fmt.Errorf("invalid Mieru nonce type %q", typeName) }
		if s.NonceMinLen < 0 || s.NonceMinLen > 12 || s.NonceMaxLen < 0 || s.NonceMaxLen > 12 || (s.NonceMaxLen > 0 && s.NonceMinLen > s.NonceMaxLen) { return nil, errors.New("Mieru nonce lengths must satisfy 0 <= min <= max <= 12") }
		n := map[string]any{"type": typeName}; if s.NonceApplyToAllUDP { n["applyToAllUDPPacket"] = true }; if s.NonceMinLen > 0 { n["minLen"] = s.NonceMinLen }; if s.NonceMaxLen > 0 { n["maxLen"] = s.NonceMaxLen }
		if len(s.NonceCustomHexStrings) > 0 {
			for _, v := range s.NonceCustomHexStrings { b, err := hex.DecodeString(strings.TrimSpace(v)); if err != nil || len(b) > 12 { return nil, fmt.Errorf("invalid Mieru fixed nonce %q", v) } }
			n["customHexStrings"] = s.NonceCustomHexStrings
		}
		m["nonce"] = n
	}
	if s.PaddingMaxMiddleLen != nil || s.PaddingMaxEndLen != nil {
		p := map[string]any{}
		if s.PaddingMaxMiddleLen != nil { if *s.PaddingMaxMiddleLen < 0 || *s.PaddingMaxMiddleLen > 255 { return nil, errors.New("Mieru middle padding must be 0..255") }; p["maxMiddlePaddingLen"] = *s.PaddingMaxMiddleLen }
		if s.PaddingMaxEndLen != nil { if *s.PaddingMaxEndLen < 0 || *s.PaddingMaxEndLen > 255 { return nil, errors.New("Mieru end padding must be 0..255") }; p["maxEndPaddingLen"] = *s.PaddingMaxEndLen }
		m["padding"] = p
	}
	if strings.TrimSpace(s.LowEntropyMode) != "" && strings.ToUpper(strings.TrimSpace(s.LowEntropyMode)) != "LOW_ENTROPY_MODE_OFF" {
		mode := strings.ToUpper(strings.TrimSpace(s.LowEntropyMode)); switch mode { case "LOW_ENTROPY_MODE_32", "LOW_ENTROPY_MODE_40", "LOW_ENTROPY_MODE_48", "LOW_ENTROPY_MODE_56": default: return nil, fmt.Errorf("invalid Mieru low entropy mode %q", mode) }
		l := map[string]any{"mode": mode}; if rot := strings.ToUpper(strings.TrimSpace(s.LowEntropyMaskRotation)); rot != "" { if rot != "LOW_ENTROPY_MASK_NO_ROTATION" && !validRotation(rot) { return nil, fmt.Errorf("invalid Mieru low entropy mask rotation %q", rot) }; l["maskRotation"] = rot }; m["lowEntropy"] = l
	}
	return m, nil
}

func validRotation(v string) bool {
	for _, side := range []string{"RIGHT", "LEFT"} { for i := 1; i <= 15; i++ { if v == fmt.Sprintf("LOW_ENTROPY_MASK_ROTATE_%s_%d", side, i) { return true } } }; return false
}

func EffectiveClientMultiplexing(s Settings) string { v := strings.ToUpper(strings.TrimSpace(s.ClientMultiplexing)); if v == "" { return "MULTIPLEXING_LOW" }; return v }
func EffectiveClientHandshakeMode(s Settings) string { v := strings.ToUpper(strings.TrimSpace(s.ClientHandshakeMode)); if v == "" { return "HANDSHAKE_STANDARD" }; return v }
