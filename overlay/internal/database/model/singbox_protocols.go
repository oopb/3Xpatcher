package model

// Supplemental sing-box protocols are stored in the same inbounds table and
// share the panel's ClientRecord/ClientInbound identity model. They are never
// rendered into Xray configuration.
const (
	TUIC      Protocol = "tuic"
	AnyTLS    Protocol = "anytls"
	ShadowTLS Protocol = "shadowtls"
	Naive     Protocol = "naive"
)

func IsSingboxProtocol(p Protocol) bool {
	switch p {
	case TUIC, AnyTLS, ShadowTLS, Naive:
		return true
	default:
		return false
	}
}

func IsSingboxTLSProtocol(p Protocol) bool {
	return p == TUIC || p == AnyTLS || p == Naive
}
