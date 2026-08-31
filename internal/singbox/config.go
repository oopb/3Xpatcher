package singbox

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Protocol is deliberately limited to protocols not managed by Xray in 3x-ui.
type Protocol string

const (
	ProtocolTUIC      Protocol = "tuic"
	ProtocolAnyTLS    Protocol = "anytls"
	ProtocolShadowTLS Protocol = "shadowtls"
	ProtocolNaive     Protocol = "naive"
)

var supportedProtocols = map[Protocol]struct{}{
	ProtocolTUIC: {}, ProtocolAnyTLS: {}, ProtocolShadowTLS: {}, ProtocolNaive: {},
}

func IsSupportedProtocol(p Protocol) bool {
	_, ok := supportedProtocols[p]
	return ok
}

func SupportedProtocols() []Protocol {
	return []Protocol{ProtocolTUIC, ProtocolAnyTLS, ProtocolShadowTLS, ProtocolNaive}
}

// InboundRecord is the stable boundary between the DB model and config renderer.
type InboundRecord struct {
	ID       int
	Remark   string
	Enable   bool
	Listen   string
	Port     int
	Protocol Protocol
	Tag      string
	Settings json.RawMessage
}

type TLSSettings struct {
	Enabled         bool     `json:"enabled"`
	ServerName      string   `json:"serverName,omitempty"`
	ALPN            []string `json:"alpn,omitempty"`
	CertificatePath string   `json:"certificatePath,omitempty"`
	KeyPath         string   `json:"keyPath,omitempty"`
}

type TUICUser struct {
	Name     string `json:"name,omitempty"`
	UUID     string `json:"uuid"`
	Password string `json:"password"`
}

type TUICSettings struct {
	Users             []TUICUser  `json:"users"`
	CongestionControl string      `json:"congestionControl,omitempty"`
	AuthTimeout       string      `json:"authTimeout,omitempty"`
	ZeroRTTHandshake  bool        `json:"zeroRTTHandshake,omitempty"`
	Heartbeat         string      `json:"heartbeat,omitempty"`
	TLS               TLSSettings `json:"tls"`
}

type PasswordUser struct {
	Name     string `json:"name,omitempty"`
	Password string `json:"password"`
}

type AnyTLSSettings struct {
	Users         []PasswordUser `json:"users"`
	PaddingScheme []string       `json:"paddingScheme,omitempty"`
	TLS           TLSSettings    `json:"tls"`
}

type ShadowTLSSettings struct {
	Users           []PasswordUser `json:"users"`
	HandshakeServer string         `json:"handshakeServer"`
	HandshakePort   int            `json:"handshakePort,omitempty"`
	StrictMode      bool           `json:"strictMode,omitempty"`
	WildcardSNI     string         `json:"wildcardSNI,omitempty"`
	InnerMethod     string         `json:"innerMethod,omitempty"`
	InnerPassword   string         `json:"innerPassword"`
}

type NaiveUser struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type NaiveSettings struct {
	Network               string      `json:"network,omitempty"`
	Users                 []NaiveUser `json:"users"`
	QUICCongestionControl string      `json:"quicCongestionControl,omitempty"`
	TLS                   TLSSettings `json:"tls"`
}

var uuidRE = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

func BuildConfig(records []InboundRecord) ([]byte, error) {
	inbounds := make([]any, 0, len(records)+2)
	seenTags := make(map[string]struct{})
	for _, r := range records {
		if !r.Enable {
			continue
		}
		if err := validateCommon(r); err != nil {
			return nil, err
		}
		if _, exists := seenTags[r.Tag]; exists {
			return nil, fmt.Errorf("duplicate sing-box tag %q", r.Tag)
		}
		seenTags[r.Tag] = struct{}{}

		rendered, extraTags, err := renderInbound(r)
		if err != nil {
			return nil, fmt.Errorf("inbound %q: %w", r.Remark, err)
		}
		for _, t := range extraTags {
			if _, exists := seenTags[t]; exists {
				return nil, fmt.Errorf("generated tag %q conflicts with another inbound", t)
			}
			seenTags[t] = struct{}{}
		}
		inbounds = append(inbounds, rendered...)
	}

	root := map[string]any{
		"log":      map[string]any{"level": "info", "timestamp": true},
		"inbounds": inbounds,
	}
	return json.MarshalIndent(root, "", "  ")
}

func validateCommon(r InboundRecord) error {
	if !IsSupportedProtocol(r.Protocol) {
		return fmt.Errorf("unsupported supplemental protocol %q", r.Protocol)
	}
	if strings.TrimSpace(r.Tag) == "" {
		return errors.New("tag is required")
	}
	if strings.TrimSpace(r.Listen) == "" {
		return errors.New("listen address is required")
	}
	if r.Port < 1 || r.Port > 65535 {
		return fmt.Errorf("port %d is outside 1..65535", r.Port)
	}
	if len(r.Settings) == 0 || string(r.Settings) == "null" {
		return errors.New("settings are required")
	}
	return nil
}

func renderInbound(r InboundRecord) ([]any, []string, error) {
	switch r.Protocol {
	case ProtocolTUIC:
		var s TUICSettings
		if err := json.Unmarshal(r.Settings, &s); err != nil {
			return nil, nil, err
		}
		if len(s.Users) == 0 {
			return nil, nil, errors.New("at least one TUIC user is required")
		}
		for _, u := range s.Users {
			if !uuidRE.MatchString(u.UUID) {
				return nil, nil, fmt.Errorf("invalid TUIC UUID %q", u.UUID)
			}
			if u.Password == "" {
				return nil, nil, errors.New("TUIC user password is required")
			}
		}
		cc := s.CongestionControl
		if cc == "" {
			cc = "cubic"
		}
		if cc != "bbr" && cc != "cubic" && cc != "new_reno" {
			return nil, nil, fmt.Errorf("invalid TUIC congestion control %q", cc)
		}
		tls, err := renderTLS(s.TLS)
		if err != nil {
			return nil, nil, err
		}
		m := baseInbound("tuic", r)
		m["users"] = s.Users
		m["congestion_control"] = cc
		if s.AuthTimeout != "" {
			m["auth_timeout"] = s.AuthTimeout
		}
		if s.ZeroRTTHandshake {
			m["zero_rtt_handshake"] = true
		}
		if s.Heartbeat != "" {
			m["heartbeat"] = s.Heartbeat
		}
		m["tls"] = tls
		return []any{m}, nil, nil

	case ProtocolAnyTLS:
		var s AnyTLSSettings
		if err := json.Unmarshal(r.Settings, &s); err != nil {
			return nil, nil, err
		}
		if err := validatePasswordUsers(s.Users); err != nil {
			return nil, nil, err
		}
		tls, err := renderTLS(s.TLS)
		if err != nil {
			return nil, nil, err
		}
		m := baseInbound("anytls", r)
		m["users"] = s.Users
		if len(s.PaddingScheme) > 0 {
			m["padding_scheme"] = s.PaddingScheme
		}
		m["tls"] = tls
		return []any{m}, nil, nil

	case ProtocolNaive:
		var s NaiveSettings
		if err := json.Unmarshal(r.Settings, &s); err != nil {
			return nil, nil, err
		}
		if len(s.Users) == 0 {
			return nil, nil, errors.New("at least one Naive user is required")
		}
		for _, u := range s.Users {
			if u.Username == "" || u.Password == "" {
				return nil, nil, errors.New("Naive username and password are required")
			}
		}
		if s.Network != "" && s.Network != "tcp" && s.Network != "udp" {
			return nil, nil, fmt.Errorf("invalid Naive network %q", s.Network)
		}
		if s.QUICCongestionControl != "" && s.QUICCongestionControl != "bbr" && s.QUICCongestionControl != "cubic" && s.QUICCongestionControl != "reno" {
			return nil, nil, fmt.Errorf("invalid Naive QUIC congestion control %q", s.QUICCongestionControl)
		}
		tls, err := renderTLS(s.TLS)
		if err != nil {
			return nil, nil, err
		}
		m := baseInbound("naive", r)
		if s.Network != "" {
			m["network"] = s.Network
		}
		m["users"] = s.Users
		if s.QUICCongestionControl != "" {
			m["quic_congestion_control"] = s.QUICCongestionControl
		}
		m["tls"] = tls
		return []any{m}, nil, nil

	case ProtocolShadowTLS:
		var s ShadowTLSSettings
		if err := json.Unmarshal(r.Settings, &s); err != nil {
			return nil, nil, err
		}
		if err := validatePasswordUsers(s.Users); err != nil {
			return nil, nil, err
		}
		if strings.TrimSpace(s.HandshakeServer) == "" {
			return nil, nil, errors.New("ShadowTLS handshake server is required")
		}
		hp := s.HandshakePort
		if hp == 0 {
			hp = 443
		}
		if hp < 1 || hp > 65535 {
			return nil, nil, errors.New("invalid ShadowTLS handshake port")
		}
		if s.WildcardSNI != "" && s.WildcardSNI != "off" && s.WildcardSNI != "authed" && s.WildcardSNI != "all" {
			return nil, nil, fmt.Errorf("invalid wildcardSNI %q", s.WildcardSNI)
		}
		method := s.InnerMethod
		if method == "" {
			method = "2022-blake3-aes-128-gcm"
		}
		if err := validateShadowTLSInnerKey(method, s.InnerPassword); err != nil {
			return nil, nil, err
		}
		innerTag := r.Tag + "-inner"
		outer := baseInbound("shadowtls", r)
		outer["version"] = 3
		outer["users"] = s.Users
		outer["detour"] = innerTag
		outer["handshake"] = map[string]any{"server": s.HandshakeServer, "server_port": hp}
		if s.StrictMode {
			outer["strict_mode"] = true
		}
		if s.WildcardSNI != "" && s.WildcardSNI != "off" {
			outer["wildcard_sni"] = s.WildcardSNI
		}

		// ShadowTLS is a carrier. The hidden injectable Shadowsocks inbound is
		// generated automatically and is never exposed as a selectable protocol.
		inner := map[string]any{
			"type":     "shadowsocks",
			"tag":      innerTag,
			"listen":   "127.0.0.1",
			"network":  "tcp",
			"method":   method,
			"password": s.InnerPassword,
		}
		return []any{outer, inner}, []string{innerTag}, nil
	}
	return nil, nil, errors.New("unreachable protocol")
}

func baseInbound(kind string, r InboundRecord) map[string]any {
	return map[string]any{
		"type":        kind,
		"tag":         r.Tag,
		"listen":      r.Listen,
		"listen_port": r.Port,
	}
}

func validatePasswordUsers(users []PasswordUser) error {
	if len(users) == 0 {
		return errors.New("at least one user is required")
	}
	for _, u := range users {
		if strings.TrimSpace(u.Password) == "" {
			return errors.New("user password is required")
		}
	}
	return nil
}

func validateShadowTLSInnerKey(method, password string) error {
	want := 0
	switch method {
	case "2022-blake3-aes-128-gcm":
		want = 16
	case "2022-blake3-aes-256-gcm", "2022-blake3-chacha20-poly1305":
		want = 32
	default:
		return fmt.Errorf("unsupported hidden ShadowTLS Shadowsocks method %q", method)
	}
	b, err := base64.StdEncoding.DecodeString(password)
	if err != nil || len(b) != want {
		return fmt.Errorf("innerPassword for %s must be standard base64 encoding of exactly %d bytes", method, want)
	}
	return nil
}

func renderTLS(s TLSSettings) (map[string]any, error) {
	if !s.Enabled {
		return nil, errors.New("TLS must be enabled for this protocol")
	}
	if strings.TrimSpace(s.CertificatePath) == "" || strings.TrimSpace(s.KeyPath) == "" {
		return nil, errors.New("TLS certificatePath and keyPath are required")
	}
	m := map[string]any{
		"enabled":          true,
		"certificate_path": s.CertificatePath,
		"key_path":         s.KeyPath,
	}
	if s.ServerName != "" {
		m["server_name"] = s.ServerName
	}
	if len(s.ALPN) > 0 {
		m["alpn"] = s.ALPN
	}
	return m, nil
}

// ValidateRecord validates protocol-specific settings even when the DB row is disabled.
func ValidateRecord(r InboundRecord) error {
	r.Enable = true
	_, err := BuildConfig([]InboundRecord{r})
	return err
}
