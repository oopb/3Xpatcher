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

func IsSupportedProtocol(p Protocol) bool { _, ok := supportedProtocols[p]; return ok }
func SupportedProtocols() []Protocol { return []Protocol{ProtocolTUIC, ProtocolAnyTLS, ProtocolShadowTLS, ProtocolNaive} }

type InboundRecord struct {
	ID int
	Remark string
	Enable bool
	Listen string
	Port int
	Protocol Protocol
	Tag string
	Settings json.RawMessage
}

type TLSSettings struct {
	Enabled bool `json:"enabled"`
	ServerName string `json:"serverName,omitempty"`
	ALPN []string `json:"alpn,omitempty"`
	MinVersion string `json:"minVersion,omitempty"`
	MaxVersion string `json:"maxVersion,omitempty"`
	CipherSuites []string `json:"cipherSuites,omitempty"`
	CurvePreferences []string `json:"curvePreferences,omitempty"`
	CertificatePath string `json:"certificatePath,omitempty"`
	KeyPath string `json:"keyPath,omitempty"`
	Certificate []string `json:"certificate,omitempty"`
	Key []string `json:"key,omitempty"`
}

type ListenSettings struct {
	BindInterface string `json:"bindInterface,omitempty"`
	RoutingMark int `json:"routingMark,omitempty"`
	ReuseAddr bool `json:"reuseAddr,omitempty"`
	NetNS string `json:"netns,omitempty"`
	TCPFastOpen bool `json:"tcpFastOpen,omitempty"`
	TCPMultiPath bool `json:"tcpMultiPath,omitempty"`
	DisableTCPKeepAlive bool `json:"disableTCPKeepAlive,omitempty"`
	TCPKeepAlive string `json:"tcpKeepAlive,omitempty"`
	TCPKeepAliveInterval string `json:"tcpKeepAliveInterval,omitempty"`
	UDPFragment bool `json:"udpFragment,omitempty"`
	UDPTimeout string `json:"udpTimeout,omitempty"`
}

type QUICSettings struct {
	IdleTimeout string `json:"idleTimeout,omitempty"`
	KeepAlivePeriod string `json:"keepAlivePeriod,omitempty"`
	StreamReceiveWindow any `json:"streamReceiveWindow,omitempty"`
	ConnectionReceiveWindow any `json:"connectionReceiveWindow,omitempty"`
	MaxConcurrentStreams int `json:"maxConcurrentStreams,omitempty"`
	InitialPacketSize int `json:"initialPacketSize,omitempty"`
	DisablePathMTUDiscovery bool `json:"disablePathMTUDiscovery,omitempty"`
}

type TUICUser struct { Name string `json:"name,omitempty"`; UUID string `json:"uuid"`; Password string `json:"password"` }
type TUICSettings struct {
	ListenSettings
	QUICSettings
	Users []TUICUser `json:"users"`
	CongestionControl string `json:"congestionControl,omitempty"`
	AuthTimeout string `json:"authTimeout,omitempty"`
	ZeroRTTHandshake bool `json:"zeroRTTHandshake,omitempty"`
	Heartbeat string `json:"heartbeat,omitempty"`
	TLS TLSSettings `json:"tls"`
}

type PasswordUser struct { Name string `json:"name,omitempty"`; Password string `json:"password"` }
type AnyTLSSettings struct {
	ListenSettings
	Users []PasswordUser `json:"users"`
	PaddingScheme []string `json:"paddingScheme,omitempty"`
	TLS TLSSettings `json:"tls"`
}

type ShadowTLSSettings struct {
	ListenSettings
	Users []PasswordUser `json:"users"`
	HandshakeServer string `json:"handshakeServer"`
	HandshakePort int `json:"handshakePort,omitempty"`
	HandshakeForServerNameJSON string `json:"handshakeForServerNameJson,omitempty"`
	StrictMode bool `json:"strictMode,omitempty"`
	WildcardSNI string `json:"wildcardSNI,omitempty"`
	InnerMethod string `json:"innerMethod,omitempty"`
	InnerPassword string `json:"innerPassword"`
}

type NaiveUser struct { Username string `json:"username"`; Password string `json:"password"` }
type NaiveSettings struct {
	ListenSettings
	Network string `json:"network,omitempty"`
	Users []NaiveUser `json:"users"`
	QUICCongestionControl string `json:"quicCongestionControl,omitempty"`
	TLS TLSSettings `json:"tls"`
}

var uuidRE = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

func BuildConfig(records []InboundRecord) ([]byte, error) {
	inbounds := make([]any, 0, len(records)+2)
	seenTags := make(map[string]struct{})
	for _, r := range records {
		if !r.Enable { continue }
		if err := validateCommon(r); err != nil { return nil, err }
		if _, exists := seenTags[r.Tag]; exists { return nil, fmt.Errorf("duplicate sing-box tag %q", r.Tag) }
		seenTags[r.Tag] = struct{}{}
		rendered, extraTags, err := renderInbound(r)
		if err != nil { return nil, fmt.Errorf("inbound %q: %w", r.Remark, err) }
		for _, t := range extraTags {
			if _, exists := seenTags[t]; exists { return nil, fmt.Errorf("generated tag %q conflicts with another inbound", t) }
			seenTags[t] = struct{}{}
		}
		inbounds = append(inbounds, rendered...)
	}
	return json.MarshalIndent(map[string]any{"log": map[string]any{"level":"info","timestamp":true}, "inbounds":inbounds}, "", "  ")
}

func validateCommon(r InboundRecord) error {
	if !IsSupportedProtocol(r.Protocol) { return fmt.Errorf("unsupported supplemental protocol %q", r.Protocol) }
	if strings.TrimSpace(r.Tag)=="" { return errors.New("tag is required") }
	if strings.TrimSpace(r.Listen)=="" { return errors.New("listen address is required") }
	if r.Port<1 || r.Port>65535 { return fmt.Errorf("port %d is outside 1..65535", r.Port) }
	if len(r.Settings)==0 || string(r.Settings)=="null" { return errors.New("settings are required") }
	return nil
}

func renderInbound(r InboundRecord) ([]any, []string, error) {
	switch r.Protocol {
	case ProtocolTUIC:
		var s TUICSettings; if err:=json.Unmarshal(r.Settings,&s); err!=nil { return nil,nil,err }
		if len(s.Users)==0 { return nil,nil,errors.New("at least one TUIC user is required") }
		for _,u:=range s.Users { if !uuidRE.MatchString(u.UUID) { return nil,nil,fmt.Errorf("invalid TUIC UUID %q",u.UUID) }; if u.Password=="" { return nil,nil,errors.New("TUIC user password is required") } }
		cc:=s.CongestionControl; if cc=="" { cc="cubic" }; if cc!="bbr"&&cc!="cubic"&&cc!="new_reno" { return nil,nil,fmt.Errorf("invalid TUIC congestion control %q",cc) }
		tls,err:=renderTLS(s.TLS); if err!=nil { return nil,nil,err }
		m:=baseInbound("tuic",r); applyListen(m,s.ListenSettings); applyQUIC(m,s.QUICSettings)
		m["users"]=s.Users; m["congestion_control"]=cc
		if s.AuthTimeout!="" { m["auth_timeout"]=s.AuthTimeout }; if s.ZeroRTTHandshake { m["zero_rtt_handshake"]=true }; if s.Heartbeat!="" { m["heartbeat"]=s.Heartbeat }; m["tls"]=tls
		return []any{m},nil,nil
	case ProtocolAnyTLS:
		var s AnyTLSSettings; if err:=json.Unmarshal(r.Settings,&s); err!=nil { return nil,nil,err }; if err:=validatePasswordUsers(s.Users); err!=nil { return nil,nil,err }
		tls,err:=renderTLS(s.TLS); if err!=nil { return nil,nil,err }; m:=baseInbound("anytls",r); applyListen(m,s.ListenSettings); m["users"]=s.Users; if len(s.PaddingScheme)>0 { m["padding_scheme"]=s.PaddingScheme }; m["tls"]=tls; return []any{m},nil,nil
	case ProtocolNaive:
		var s NaiveSettings; if err:=json.Unmarshal(r.Settings,&s); err!=nil { return nil,nil,err }; if len(s.Users)==0 { return nil,nil,errors.New("at least one Naive user is required") }; for _,u:=range s.Users { if u.Username==""||u.Password=="" { return nil,nil,errors.New("Naive username and password are required") } }
		if s.Network!=""&&s.Network!="tcp"&&s.Network!="udp" { return nil,nil,fmt.Errorf("invalid Naive network %q",s.Network) }; if s.QUICCongestionControl!=""&&s.QUICCongestionControl!="bbr"&&s.QUICCongestionControl!="cubic"&&s.QUICCongestionControl!="reno" { return nil,nil,fmt.Errorf("invalid Naive QUIC congestion control %q",s.QUICCongestionControl) }
		tls,err:=renderTLS(s.TLS); if err!=nil { return nil,nil,err }; m:=baseInbound("naive",r); applyListen(m,s.ListenSettings); if s.Network!="" { m["network"]=s.Network }; m["users"]=s.Users; if s.QUICCongestionControl!="" { m["quic_congestion_control"]=s.QUICCongestionControl }; m["tls"]=tls; return []any{m},nil,nil
	case ProtocolShadowTLS:
		var s ShadowTLSSettings; if err:=json.Unmarshal(r.Settings,&s); err!=nil { return nil,nil,err }; if err:=validatePasswordUsers(s.Users); err!=nil { return nil,nil,err }
		if strings.TrimSpace(s.HandshakeServer)=="" && s.WildcardSNI!="all" { return nil,nil,errors.New("ShadowTLS handshake server is required") }; hp:=s.HandshakePort; if hp==0 { hp=443 }; if hp<1||hp>65535 { return nil,nil,errors.New("invalid ShadowTLS handshake port") }
		if s.WildcardSNI!=""&&s.WildcardSNI!="off"&&s.WildcardSNI!="authed"&&s.WildcardSNI!="all" { return nil,nil,fmt.Errorf("invalid wildcardSNI %q",s.WildcardSNI) }
		method:=s.InnerMethod; if method=="" { method="2022-blake3-aes-128-gcm" }; if err:=validateShadowTLSInnerKey(method,s.InnerPassword); err!=nil { return nil,nil,err }
		innerTag:=r.Tag+"-inner"; outer:=baseInbound("shadowtls",r); applyListen(outer,s.ListenSettings); outer["version"]=3; outer["users"]=s.Users; outer["detour"]=innerTag
		if strings.TrimSpace(s.HandshakeServer)!="" { outer["handshake"]=map[string]any{"server":s.HandshakeServer,"server_port":hp} }
		if strings.TrimSpace(s.HandshakeForServerNameJSON)!="" { var routes map[string]map[string]any; if err:=json.Unmarshal([]byte(s.HandshakeForServerNameJSON),&routes); err!=nil { return nil,nil,fmt.Errorf("invalid handshakeForServerNameJson: %w",err) }; if len(routes)>0 { outer["handshake_for_server_name"]=routes } }
		if s.StrictMode { outer["strict_mode"]=true }; if s.WildcardSNI!=""&&s.WildcardSNI!="off" { outer["wildcard_sni"]=s.WildcardSNI }
		inner:=map[string]any{"type":"shadowsocks","tag":innerTag,"listen":"127.0.0.1","network":"tcp","method":method,"password":s.InnerPassword}; return []any{outer,inner},[]string{innerTag},nil
	}
	return nil,nil,errors.New("unreachable protocol")
}

func baseInbound(kind string,r InboundRecord) map[string]any { return map[string]any{"type":kind,"tag":r.Tag,"listen":r.Listen,"listen_port":r.Port} }
func applyListen(m map[string]any,s ListenSettings) {
	if s.BindInterface!="" { m["bind_interface"]=s.BindInterface }; if s.RoutingMark!=0 { m["routing_mark"]=s.RoutingMark }; if s.ReuseAddr { m["reuse_addr"]=true }; if s.NetNS!="" { m["netns"]=s.NetNS }; if s.TCPFastOpen { m["tcp_fast_open"]=true }; if s.TCPMultiPath { m["tcp_multi_path"]=true }; if s.DisableTCPKeepAlive { m["disable_tcp_keep_alive"]=true }; if s.TCPKeepAlive!="" { m["tcp_keep_alive"]=s.TCPKeepAlive }; if s.TCPKeepAliveInterval!="" { m["tcp_keep_alive_interval"]=s.TCPKeepAliveInterval }; if s.UDPFragment { m["udp_fragment"]=true }; if s.UDPTimeout!="" { m["udp_timeout"]=s.UDPTimeout }
}
func applyQUIC(m map[string]any,s QUICSettings) {
	if s.IdleTimeout!="" { m["idle_timeout"]=s.IdleTimeout }; if s.KeepAlivePeriod!="" { m["keep_alive_period"]=s.KeepAlivePeriod }; if s.StreamReceiveWindow!=nil { m["stream_receive_window"]=s.StreamReceiveWindow }; if s.ConnectionReceiveWindow!=nil { m["connection_receive_window"]=s.ConnectionReceiveWindow }; if s.MaxConcurrentStreams>0 { m["max_concurrent_streams"]=s.MaxConcurrentStreams }; if s.InitialPacketSize>0 { m["initial_packet_size"]=s.InitialPacketSize }; if s.DisablePathMTUDiscovery { m["disable_path_mtu_discovery"]=true }
}
func validatePasswordUsers(users []PasswordUser) error { if len(users)==0 { return errors.New("at least one user is required") }; for _,u:=range users { if strings.TrimSpace(u.Password)=="" { return errors.New("user password is required") } }; return nil }
func validateShadowTLSInnerKey(method,password string) error { want:=0; switch method { case "2022-blake3-aes-128-gcm": want=16; case "2022-blake3-aes-256-gcm","2022-blake3-chacha20-poly1305": want=32; default: return fmt.Errorf("unsupported hidden ShadowTLS Shadowsocks method %q",method) }; b,err:=base64.StdEncoding.DecodeString(password); if err!=nil||len(b)!=want { return fmt.Errorf("innerPassword for %s must be standard base64 encoding of exactly %d bytes",method,want) }; return nil }
func renderTLS(s TLSSettings)(map[string]any,error){ if !s.Enabled { return nil,errors.New("TLS must be enabled for this protocol") }; m:=map[string]any{"enabled":true}; if strings.TrimSpace(s.CertificatePath)!=""||strings.TrimSpace(s.KeyPath)!="" { if strings.TrimSpace(s.CertificatePath)==""||strings.TrimSpace(s.KeyPath)=="" { return nil,errors.New("TLS certificatePath and keyPath must be provided together") }; m["certificate_path"]=s.CertificatePath; m["key_path"]=s.KeyPath } else { if len(s.Certificate)==0||len(s.Key)==0 { return nil,errors.New("TLS certificate and key are required") }; m["certificate"]=s.Certificate; m["key"]=s.Key }; if s.ServerName!="" { m["server_name"]=s.ServerName }; if len(s.ALPN)>0 { m["alpn"]=s.ALPN }; if s.MinVersion!="" { m["min_version"]=s.MinVersion }; if s.MaxVersion!="" { m["max_version"]=s.MaxVersion }; if len(s.CipherSuites)>0 { m["cipher_suites"]=s.CipherSuites }; if len(s.CurvePreferences)>0 { m["curve_preferences"]=s.CurvePreferences }; return m,nil }
func ValidateRecord(r InboundRecord) error { r.Enable=true; _,err:=BuildConfig([]InboundRecord{r}); return err }
