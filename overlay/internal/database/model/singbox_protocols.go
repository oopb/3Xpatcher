package model

// Supplemental protocols are stored in the same inbounds table and share the
// panel's ClientRecord/ClientInbound identity model. They are never rendered
// into Xray configuration. sing-box protocols use x-ui-singbox; Mieru uses
// isolated official mita instances.
const (
	TUIC      Protocol = "tuic"
	AnyTLS    Protocol = "anytls"
	ShadowTLS Protocol = "shadowtls"
	Naive     Protocol = "naive"
	Mieru     Protocol = "mieru"
)

func IsSingboxProtocol(p Protocol) bool {
	switch p {
	case TUIC, AnyTLS, ShadowTLS, Naive:
		return true
	default:
		return false
	}
}

func IsMieruProtocol(p Protocol) bool { return p == Mieru }
func IsSupplementalProtocol(p Protocol) bool { return IsSingboxProtocol(p) || IsMieruProtocol(p) }

func IsSingboxTLSProtocol(p Protocol) bool { return p == TUIC || p == AnyTLS || p == Naive }
