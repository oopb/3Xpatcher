package singbox

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var supplementalCertBase = "/usr/local/x-ui-singbox/certs"

// installSelfSignedTLS implements the explicit "camouflage SNI" mode. It does
// not impersonate a public CA or REALITY: the generated certificate is
// self-signed, carries the requested SNI in SAN, and subscriptions must opt in
// to insecure/skip-cert-verify.
func installSelfSignedTLS(settings map[string]any, tlsIn map[string]any) error {
	sni, _ := settings["camouflageSNI"].(string)
	sni = strings.TrimSpace(sni)
	if err := validateCamouflageSNI(sni); err != nil {
		return err
	}
	days := intNumber(settings["selfSignedValidityDays"], 3650)
	if days < 1 || days > 3650 {
		return errors.New("self-signed certificate validity must be between 1 and 3650 days")
	}
	certPath, keyPath, err := ensureSelfSignedCertificate(sni, days)
	if err != nil {
		return err
	}
	tlsOut := map[string]any{
		"enabled":         true,
		"serverName":      sni,
		"certificatePath": certPath,
		"keyPath":         keyPath,
	}
	if alpn := stringSlice(tlsIn["alpn"]); len(alpn) > 0 {
		tlsOut["alpn"] = alpn
	}
	copyNativeTLSOptions(tlsIn, tlsOut)
	settings["tls"] = tlsOut
	return nil
}

func copyNativeTLSOptions(src, dst map[string]any) {
	copyString := func(srcKey, dstKey string) {
		if v, ok := src[srcKey].(string); ok && strings.TrimSpace(v) != "" {
			dst[dstKey] = v
		}
	}
	copyString("minVersion", "minVersion")
	copyString("maxVersion", "maxVersion")
	if raw, _ := src["cipherSuites"].(string); strings.TrimSpace(raw) != "" {
		parts := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ':' || r == ';' || r == ' ' })
		if len(parts) > 0 {
			dst["cipherSuites"] = parts
		}
	}
	if curves := stringSlice(src["curvePreferences"]); len(curves) > 0 {
		dst["curvePreferences"] = curves
	}
}

func validateCamouflageSNI(sni string) error {
	if sni == "" {
		return errors.New("camouflage SNI is required")
	}
	if len(sni) > 253 || strings.ContainsAny(sni, " /\\:@") {
		return errors.New("invalid camouflage SNI")
	}
	for _, label := range strings.Split(sni, ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return errors.New("invalid camouflage SNI")
		}
		for _, r := range label {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-') {
				return errors.New("invalid camouflage SNI")
			}
		}
	}
	return nil
}

func ensureSelfSignedCertificate(sni string, days int) (string, string, error) {
	h := sha256.Sum256([]byte(strings.ToLower(sni)))
	dir := filepath.Join(supplementalCertBase, hex.EncodeToString(h[:8]))
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	if certificateStillValid(certPath, sni) {
		if _, err := os.Stat(keyPath); err == nil {
			return certPath, keyPath, nil
		}
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", "", err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", err
	}
	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return "", "", err
	}
	now := time.Now()
	tmpl := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{CommonName: sni},
		DNSNames: []string{sni},
		NotBefore: now.Add(-5 * time.Minute),
		NotAfter: now.Add(time.Duration(days) * 24 * time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		return "", "", err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return "", "", err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	if err := atomicSecretWrite(certPath, certPEM, 0644); err != nil {
		return "", "", err
	}
	if err := atomicSecretWrite(keyPath, keyPEM, 0600); err != nil {
		return "", "", err
	}
	return certPath, keyPath, nil
}

func certificateStillValid(path, sni string) bool {
	data, err := os.ReadFile(path)
	if err != nil { return false }
	block, _ := pem.Decode(data)
	if block == nil { return false }
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil || cert.VerifyHostname(sni) != nil { return false }
	return time.Until(cert.NotAfter) > 30*24*time.Hour
}

func atomicSecretWrite(path string, data []byte, mode os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil { return err }
	if err := os.Chmod(tmp, mode); err != nil { _ = os.Remove(tmp); return err }
	if err := os.Rename(tmp, path); err != nil { _ = os.Remove(tmp); return err }
	return nil
}
