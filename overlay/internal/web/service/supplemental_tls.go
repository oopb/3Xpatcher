package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	sbox "github.com/mhsanaei/3x-ui/v3/internal/singbox"
)

// materializeSupplementalSelfSignedTLS makes the supplemental self-signed-SNI
// mode look like an ordinary native 3x-ui file-backed TLS certificate before
// v3.8.5's backend save validator runs. The certificate remains on disk and the
// canonical streamSettings stores certificateFile/keyFile, so subsequent edits
// and saves use the same representation as 3x-ui's built-in TLS path mode.
func materializeSupplementalSelfSignedTLS(inbound *model.Inbound) error {
	if inbound == nil || !model.IsSingboxProtocol(inbound.Protocol) || strings.TrimSpace(inbound.StreamSettings) == "" {
		return nil
	}

	var stream map[string]any
	if err := json.Unmarshal([]byte(inbound.StreamSettings), &stream); err != nil {
		return fmt.Errorf("invalid supplemental stream settings: %w", err)
	}
	security, _ := stream["security"].(string)
	if security != "tls" {
		return nil
	}
	tlsSettings, ok := stream["tlsSettings"].(map[string]any)
	if !ok || tlsSettings == nil {
		return nil
	}
	mode, _ := tlsSettings["certificateMode"].(string)
	if strings.TrimSpace(mode) != "self_signed_sni" {
		return nil
	}

	sni, _ := tlsSettings["serverName"].(string)
	days := supplementalTLSValidityDays(tlsSettings["selfSignedValidityDays"])
	info, err := sbox.GenerateSelfSignedCertificate(strings.TrimSpace(sni), days)
	if err != nil {
		return fmt.Errorf("generate supplemental TLS certificate: %w", err)
	}

	serverCert := map[string]any{
		"usage":           "encipherment",
		"certificateFile": info.CertificatePath,
		"keyFile":         info.KeyPath,
		"ocspStapling":    0,
		"oneTimeLoading":  false,
		"buildChain":      false,
	}
	certificates := []any{serverCert}
	if existing, ok := tlsSettings["certificates"].([]any); ok {
		for _, raw := range existing {
			cert, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			usage, _ := cert["usage"].(string)
			if usage == "verify" {
				certificates = append(certificates, cert)
			}
		}
	}
	tlsSettings["certificates"] = certificates
	stream["tlsSettings"] = tlsSettings

	encoded, err := json.Marshal(stream)
	if err != nil {
		return fmt.Errorf("encode supplemental TLS settings: %w", err)
	}
	inbound.StreamSettings = string(encoded)
	return nil
}

func supplementalTLSValidityDays(v any) int {
	const fallback = 3650
	var days int
	switch n := v.(type) {
	case float64:
		days = int(n)
	case float32:
		days = int(n)
	case int:
		days = n
	case int64:
		days = int(n)
	case json.Number:
		if parsed, err := n.Int64(); err == nil {
			days = int(parsed)
		}
	}
	if days < 1 || days > 3650 {
		return fallback
	}
	return days
}
