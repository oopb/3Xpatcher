package mieru

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
)

type User struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type PortBinding struct {
	Port         int    `json:"port,omitempty"`
	PortRangeEnd int    `json:"portRangeEnd,omitempty"`
	Transport    string `json:"transport,omitempty"`
}

type DNSHost struct {
	Domain string `json:"domain"`
	IP     string `json:"ip"`
}

type EgressProxy struct {
	Name     string `json:"name"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

type EgressRule struct {
	IPRanges    []string `json:"ipRanges,omitempty"`
	DomainNames []string `json:"domainNames,omitempty"`
	Action      string   `json:"action"`
	ProxyNames  []string `json:"proxyNames,omitempty"`
}

type Settings struct {
	Transport              string        `json:"transport,omitempty"`
	PortRangeEnd           int           `json:"portRangeEnd,omitempty"`
	AdditionalPortBindings []PortBinding `json:"additionalPortBindings,omitempty"`
	MTU                    int           `json:"mtu,omitempty"`
	LoggingLevel           string        `json:"loggingLevel,omitempty"`
	AllowPrivateIP         bool          `json:"allowPrivateIP,omitempty"`
	AllowLoopbackIP        bool          `json:"allowLoopbackIP,omitempty"`
	QuotaDays              int           `json:"quotaDays,omitempty"`
	QuotaMegabytes         int           `json:"quotaMegabytes,omitempty"`
	MetricsLoggingInterval string        `json:"metricsLoggingInterval,omitempty"`
	UserHintIsMandatory    bool          `json:"userHintIsMandatory,omitempty"`
	DNSDualStack           string        `json:"dnsDualStack,omitempty"`
	DNSHosts               []DNSHost     `json:"dnsHosts,omitempty"`
	EgressProxies          []EgressProxy `json:"egressProxies,omitempty"`
	EgressRules            []EgressRule  `json:"egressRules,omitempty"`
	TrafficPatternEnabled  bool          `json:"trafficPatternEnabled,omitempty"`
	TrafficSeed            int           `json:"trafficSeed,omitempty"`
	TrafficUnlockAll       bool          `json:"trafficUnlockAll,omitempty"`
	TCPFragmentEnable      bool          `json:"tcpFragmentEnable,omitempty"`
	TCPFragmentMaxSleepMs  int           `json:"tcpFragmentMaxSleepMs,omitempty"`
	NonceType              string        `json:"nonceType,omitempty"`
	NonceApplyToAllUDP     bool          `json:"nonceApplyToAllUDP,omitempty"`
	NonceMinLen            int           `json:"nonceMinLen,omitempty"`
	NonceMaxLen            int           `json:"nonceMaxLen,omitempty"`
	NonceCustomHexStrings  []string      `json:"nonceCustomHexStrings,omitempty"`
	PaddingMaxMiddleLen    *int          `json:"paddingMaxMiddleLen,omitempty"`
	PaddingMaxEndLen       *int          `json:"paddingMaxEndLen,omitempty"`
	LowEntropyMode         string        `json:"lowEntropyMode,omitempty"`
	LowEntropyMaskRotation string        `json:"lowEntropyMaskRotation,omitempty"`
	ClientMultiplexing     string        `json:"clientMultiplexing,omitempty"`
	ClientHandshakeMode    string        `json:"clientHandshakeMode,omitempty"`
	ClientTrafficPattern   string        `json:"clientTrafficPattern,omitempty"`
}

type Record struct {
	ID       int
	Remark   string
	Port     int
	Settings Settings
	Users    []User
}

func BuildServerConfig(r Record) ([]byte, error) {
	if r.ID <= 0 {
		return nil, errors.New("inbound id is required")
	}
	if len(r.Users) == 0 {
		return nil, errors.New("at least one Mieru user is required")
	}
	for _, u := range r.Users {
		if strings.TrimSpace(u.Name) == "" || strings.TrimSpace(u.Password) == "" {
			return nil, errors.New("Mieru user name and password are required")
		}
	}

	bindings := make([]any, 0, 1+len(r.Settings.AdditionalPortBindings))
	seenBindings := make(map[string]struct{}, cap(bindings))
	primary, key, err := buildPortBinding(r.Port, r.Settings.PortRangeEnd, r.Settings.Transport)
	if err != nil {
		return nil, err
	}
	bindings = append(bindings, primary)
	seenBindings[key] = struct{}{}
	for i, extra := range r.Settings.AdditionalPortBindings {
		binding, bindingKey, err := buildPortBinding(extra.Port, extra.PortRangeEnd, extra.Transport)
		if err != nil {
			return nil, fmt.Errorf("additional Mieru port binding #%d: %w", i+1, err)
		}
		if _, exists := seenBindings[bindingKey]; exists {
			return nil, fmt.Errorf("duplicate Mieru port binding %s", bindingKey)
		}
		seenBindings[bindingKey] = struct{}{}
		bindings = append(bindings, binding)
	}

	users := make([]map[string]any, 0, len(r.Users))
	for _, u := range r.Users {
		digest := sha256.Sum256([]byte(u.Password + "\x00" + u.Name))
		m := map[string]any{"name": u.Name, "hashedPassword": hex.EncodeToString(digest[:])}
		if r.Settings.AllowPrivateIP {
			m["allowPrivateIP"] = true
		}
		if r.Settings.AllowLoopbackIP {
			m["allowLoopbackIP"] = true
		}
		if r.Settings.QuotaDays > 0 || r.Settings.QuotaMegabytes > 0 {
			if r.Settings.QuotaDays <= 0 || r.Settings.QuotaMegabytes <= 0 {
				return nil, errors.New("Mieru quota days and megabytes must be set together")
			}
			m["quotas"] = []map[string]any{{"days": r.Settings.QuotaDays, "megabytes": r.Settings.QuotaMegabytes}}
		}
		users = append(users, m)
	}

	mtu := r.Settings.MTU
	if mtu == 0 {
		mtu = 1400
	}
	if mtu < 1280 || mtu > 65535 {
		return nil, fmt.Errorf("invalid Mieru MTU %d", mtu)
	}
	logging := strings.ToUpper(strings.TrimSpace(r.Settings.LoggingLevel))
	if logging == "" {
		logging = "INFO"
	}
	switch logging {
	case "FATAL", "ERROR", "WARN", "INFO", "DEBUG", "TRACE":
	default:
		return nil, fmt.Errorf("invalid Mieru logging level %q", logging)
	}

	root := map[string]any{"portBindings": bindings, "users": users, "loggingLevel": logging, "mtu": mtu}
	advanced := map[string]any{}
	if strings.TrimSpace(r.Settings.MetricsLoggingInterval) != "" {
		advanced["metricsLoggingInterval"] = strings.TrimSpace(r.Settings.MetricsLoggingInterval)
	}
	if r.Settings.UserHintIsMandatory {
		advanced["userHintIsMandatory"] = true
	}
	if len(advanced) > 0 {
		root["advancedSettings"] = advanced
	}

	dns, err := buildDNS(r.Settings)
	if err != nil {
		return nil, err
	}
	if len(dns) > 0 {
		root["dns"] = dns
	}
	egress, err := buildEgress(r.Settings)
	if err != nil {
		return nil, err
	}
	if len(egress) > 0 {
		root["egress"] = egress
	}
	if r.Settings.TrafficPatternEnabled {
		traffic, err := buildTrafficPattern(r.Settings)
		if err != nil {
			return nil, err
		}
		root["trafficPattern"] = traffic
	}
	return json.MarshalIndent(root, "", "  ")
}

func buildPortBinding(port, rangeEnd int, transport string) (map[string]any, string, error) {
	if port < 1 || port > 65535 {
		return nil, "", fmt.Errorf("Mieru port %d is outside 1..65535", port)
	}
	transport = strings.ToUpper(strings.TrimSpace(transport))
	if transport == "" {
		transport = "TCP"
	}
	if transport != "TCP" && transport != "UDP" {
		return nil, "", fmt.Errorf("invalid Mieru transport %q", transport)
	}
	binding := map[string]any{"protocol": transport}
	key := transport + ":"
	if rangeEnd > 0 {
		if rangeEnd < port || rangeEnd > 65535 {
			return nil, "", fmt.Errorf("Mieru port range %d-%d is invalid", port, rangeEnd)
		}
		rangeText := fmt.Sprintf("%d-%d", port, rangeEnd)
		binding["portRange"] = rangeText
		key += rangeText
	} else {
		binding["port"] = port
		key += fmt.Sprintf("%d", port)
	}
	return binding, key, nil
}

func buildDNS(s Settings) (map[string]any, error) {
	dualStack := strings.TrimSpace(s.DNSDualStack)
	if dualStack != "" {
		switch dualStack {
		case "USE_FIRST_IP", "PREFER_IPv4", "PREFER_IPv6", "ONLY_IPv4", "ONLY_IPv6":
		default:
			return nil, fmt.Errorf("invalid Mieru DNS dual-stack policy %q", dualStack)
		}
	}
	hosts := make(map[string]string, len(s.DNSHosts))
	for i, entry := range s.DNSHosts {
		domain := strings.TrimSpace(entry.Domain)
		ipText := strings.TrimSpace(entry.IP)
		if domain == "" || ipText == "" {
			return nil, fmt.Errorf("Mieru DNS host #%d requires domain and IP", i+1)
		}
		ip := net.ParseIP(strings.Trim(ipText, "[]"))
		if ip == nil {
			return nil, fmt.Errorf("Mieru DNS host #%d has invalid IP %q", i+1, ipText)
		}
		if _, exists := hosts[domain]; exists {
			return nil, fmt.Errorf("duplicate Mieru DNS host %q", domain)
		}
		hosts[domain] = ip.String()
	}
	if dualStack == "" && len(hosts) == 0 {
		return nil, nil
	}
	out := map[string]any{}
	if dualStack != "" {
		out["dualStack"] = dualStack
	}
	if len(hosts) > 0 {
		out["hosts"] = hosts
	}
	return out, nil
}

func buildEgress(s Settings) (map[string]any, error) {
	if len(s.EgressProxies) == 0 && len(s.EgressRules) == 0 {
		return nil, nil
	}
	proxyNames := make(map[string]struct{}, len(s.EgressProxies))
	proxies := make([]map[string]any, 0, len(s.EgressProxies))
	for i, proxy := range s.EgressProxies {
		name := strings.TrimSpace(proxy.Name)
		host := strings.TrimSpace(proxy.Host)
		if name == "" || host == "" {
			return nil, fmt.Errorf("Mieru egress proxy #%d requires name and host", i+1)
		}
		if proxy.Port < 1 || proxy.Port > 65535 {
			return nil, fmt.Errorf("Mieru egress proxy %q has invalid port %d", name, proxy.Port)
		}
		if _, exists := proxyNames[name]; exists {
			return nil, fmt.Errorf("duplicate Mieru egress proxy name %q", name)
		}
		proxyNames[name] = struct{}{}
		m := map[string]any{
			"name":     name,
			"protocol": "SOCKS5_PROXY_PROTOCOL",
			"host":     host,
			"port":     proxy.Port,
		}
		user := strings.TrimSpace(proxy.Username)
		password := proxy.Password
		if (user == "") != (password == "") {
			return nil, fmt.Errorf("Mieru egress proxy %q SOCKS5 username and password must be set together", name)
		}
		if user != "" {
			m["socks5Authentication"] = map[string]any{"user": user, "password": password}
		}
		proxies = append(proxies, m)
	}

	rules := make([]map[string]any, 0, len(s.EgressRules))
	for i, rule := range s.EgressRules {
		action := strings.ToUpper(strings.TrimSpace(rule.Action))
		if action == "" {
			action = "DIRECT"
		}
		switch action {
		case "DIRECT", "PROXY", "REJECT":
		default:
			return nil, fmt.Errorf("Mieru egress rule #%d has invalid action %q", i+1, rule.Action)
		}
		ipRanges := cleanStrings(rule.IPRanges)
		domainNames := cleanStrings(rule.DomainNames)
		if len(ipRanges) == 0 && len(domainNames) == 0 {
			return nil, fmt.Errorf("Mieru egress rule #%d requires at least one IP range or domain", i+1)
		}
		for _, cidr := range ipRanges {
			if cidr == "*" {
				continue
			}
			if _, _, err := net.ParseCIDR(cidr); err != nil {
				return nil, fmt.Errorf("Mieru egress rule #%d has invalid IP CIDR %q", i+1, cidr)
			}
		}
		m := map[string]any{"action": action}
		if len(ipRanges) > 0 {
			m["ipRanges"] = ipRanges
		}
		if len(domainNames) > 0 {
			m["domainNames"] = domainNames
		}
		if action == "PROXY" {
			names := cleanStrings(rule.ProxyNames)
			if len(names) == 0 {
				return nil, fmt.Errorf("Mieru egress rule #%d PROXY action requires proxy names", i+1)
			}
			for _, name := range names {
				if _, exists := proxyNames[name]; !exists {
					return nil, fmt.Errorf("Mieru egress rule #%d references unknown proxy %q", i+1, name)
				}
			}
			m["proxyNames"] = names
		}
		rules = append(rules, m)
	}
	out := map[string]any{}
	if len(proxies) > 0 {
		out["proxies"] = proxies
	}
	if len(rules) > 0 {
		out["rules"] = rules
	}
	return out, nil
}

func cleanStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func buildTrafficPattern(s Settings) (map[string]any, error) {
	m := map[string]any{}
	if s.TrafficSeed != 0 {
		m["seed"] = s.TrafficSeed
	}
	if s.TrafficUnlockAll {
		m["unlockAll"] = true
	}
	if s.TCPFragmentEnable || s.TCPFragmentMaxSleepMs != 0 {
		if s.TCPFragmentMaxSleepMs < 0 || s.TCPFragmentMaxSleepMs > 100 {
			return nil, errors.New("Mieru tcpFragment maxSleepMs must be 0..100")
		}
		t := map[string]any{"enable": s.TCPFragmentEnable}
		if s.TCPFragmentMaxSleepMs > 0 {
			t["maxSleepMs"] = s.TCPFragmentMaxSleepMs
		}
		m["tcpFragment"] = t
	}
	if strings.TrimSpace(s.NonceType) != "" {
		typeName := strings.ToUpper(strings.TrimSpace(s.NonceType))
		switch typeName {
		case "NONCE_TYPE_RANDOM", "NONCE_TYPE_PRINTABLE", "NONCE_TYPE_PRINTABLE_SUBSET", "NONCE_TYPE_FIXED":
		default:
			return nil, fmt.Errorf("invalid Mieru nonce type %q", typeName)
		}
		if s.NonceMinLen < 0 || s.NonceMinLen > 12 || s.NonceMaxLen < 0 || s.NonceMaxLen > 12 || (s.NonceMaxLen > 0 && s.NonceMinLen > s.NonceMaxLen) {
			return nil, errors.New("Mieru nonce lengths must satisfy 0 <= min <= max <= 12")
		}
		n := map[string]any{"type": typeName}
		if s.NonceApplyToAllUDP {
			n["applyToAllUDPPacket"] = true
		}
		if s.NonceMinLen > 0 {
			n["minLen"] = s.NonceMinLen
		}
		if s.NonceMaxLen > 0 {
			n["maxLen"] = s.NonceMaxLen
		}
		if len(s.NonceCustomHexStrings) > 0 {
			for _, v := range s.NonceCustomHexStrings {
				b, err := hex.DecodeString(strings.TrimSpace(v))
				if err != nil || len(b) > 12 {
					return nil, fmt.Errorf("invalid Mieru fixed nonce %q", v)
				}
			}
			n["customHexStrings"] = s.NonceCustomHexStrings
		}
		m["nonce"] = n
	}
	if s.PaddingMaxMiddleLen != nil || s.PaddingMaxEndLen != nil {
		p := map[string]any{}
		if s.PaddingMaxMiddleLen != nil {
			if *s.PaddingMaxMiddleLen < 0 || *s.PaddingMaxMiddleLen > 255 {
				return nil, errors.New("Mieru middle padding must be 0..255")
			}
			p["maxMiddlePaddingLen"] = *s.PaddingMaxMiddleLen
		}
		if s.PaddingMaxEndLen != nil {
			if *s.PaddingMaxEndLen < 0 || *s.PaddingMaxEndLen > 255 {
				return nil, errors.New("Mieru end padding must be 0..255")
			}
			p["maxEndPaddingLen"] = *s.PaddingMaxEndLen
		}
		m["padding"] = p
	}
	if strings.TrimSpace(s.LowEntropyMode) != "" && strings.ToUpper(strings.TrimSpace(s.LowEntropyMode)) != "LOW_ENTROPY_MODE_OFF" {
		mode := strings.ToUpper(strings.TrimSpace(s.LowEntropyMode))
		switch mode {
		case "LOW_ENTROPY_MODE_32", "LOW_ENTROPY_MODE_40", "LOW_ENTROPY_MODE_48", "LOW_ENTROPY_MODE_56":
		default:
			return nil, fmt.Errorf("invalid Mieru low entropy mode %q", mode)
		}
		l := map[string]any{"mode": mode}
		if rot := strings.ToUpper(strings.TrimSpace(s.LowEntropyMaskRotation)); rot != "" {
			if rot != "LOW_ENTROPY_MASK_NO_ROTATION" && !validRotation(rot) {
				return nil, fmt.Errorf("invalid Mieru low entropy mask rotation %q", rot)
			}
			l["maskRotation"] = rot
		}
		m["lowEntropy"] = l
	}
	return m, nil
}

func validRotation(v string) bool {
	for _, side := range []string{"RIGHT", "LEFT"} {
		for i := 1; i <= 15; i++ {
			if v == fmt.Sprintf("LOW_ENTROPY_MASK_ROTATE_%s_%d", side, i) {
				return true
			}
		}
	}
	return false
}

func EffectiveClientMultiplexing(s Settings) string {
	v := strings.ToUpper(strings.TrimSpace(s.ClientMultiplexing))
	if v == "" {
		return "MULTIPLEXING_LOW"
	}
	return v
}

func EffectiveClientHandshakeMode(s Settings) string {
	v := strings.ToUpper(strings.TrimSpace(s.ClientHandshakeMode))
	if v == "" {
		return "HANDSHAKE_STANDARD"
	}
	return v
}
