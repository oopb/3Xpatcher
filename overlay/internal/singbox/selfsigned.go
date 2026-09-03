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

type SelfSignedCertificateInfo struct {
	ServerName      string `json:"serverName"`
	CertificatePath string `json:"certificatePath"`
	KeyPath         string `json:"keyPath"`
	NotAfter        string `json:"notAfter"`
	Created         bool   `json:"created"`
}

func installSelfSignedTLS(settings map[string]any, tlsIn map[string]any) error {
	sni, _ := tlsIn["serverName"].(string)
	sni = strings.TrimSpace(sni)
	days := intNumber(tlsIn["selfSignedValidityDays"], 3650)
	if sni == "" {
		if oldSNI, _ := settings["camouflageSNI"].(string); strings.TrimSpace(oldSNI) != "" {
			sni = strings.TrimSpace(oldSNI)
		}
	}
	if _, exists := tlsIn["selfSignedValidityDays"]; !exists {
		days = intNumber(settings["selfSignedValidityDays"], days)
	}
	info, err := generateSelfSignedCertificate(sni, days, false)
	if err != nil {
		return err
	}
	tlsOut := map[string]any{
		"enabled": true,
		"serverName": sni,
		"certificatePath": info.CertificatePath,
		"keyPath": info.KeyPath,
	}
	if alpn := stringSlice(tlsIn["alpn"]); len(alpn) > 0 {
		tlsOut["alpn"] = alpn
	}
	copyNativeTLSOptions(tlsIn, tlsOut)
	settings["tls"] = tlsOut
	return nil
}

func GenerateSelfSignedCertificate(sni string, days int) (SelfSignedCertificateInfo, error) {
	return generateSelfSignedCertificate(sni, days, false)
}

func RegenerateSelfSignedCertificate(sni string, days int) (SelfSignedCertificateInfo, error) {
	return generateSelfSignedCertificate(sni, days, true)
}

func generateSelfSignedCertificate(sni string, days int, force bool) (SelfSignedCertificateInfo, error) {
	sni = strings.TrimSpace(sni)
	if err := validateCamouflageSNI(sni); err != nil {
		return SelfSignedCertificateInfo{}, err
	}
	if days < 1 || days > 3650 {
		return SelfSignedCertificateInfo{}, errors.New("self-signed certificate validity must be between 1 and 3650 days")
	}
	h := sha256.Sum256([]byte(strings.ToLower(sni)))
	dir := filepath.Join(supplementalCertBase, hex.EncodeToString(h[:8]))
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	if !force {
		if cert, ok := reusableCertificate(certPath, keyPath, sni); ok {
			return SelfSignedCertificateInfo{ServerName: sni, CertificatePath: certPath, KeyPath: keyPath, NotAfter: cert.NotAfter.UTC().Format(time.RFC3339), Created: false}, nil
		}
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return SelfSignedCertificateInfo{}, err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return SelfSignedCertificateInfo{}, err
	}
	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return SelfSignedCertificateInfo{}, err
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
		return SelfSignedCertificateInfo{}, err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return SelfSignedCertificateInfo{}, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	if err := atomicSecretWrite(certPath, certPEM, 0644); err != nil {
		return SelfSignedCertificateInfo{}, err
	}
	if err := atomicSecretWrite(keyPath, keyPEM, 0600); err != nil {
		return SelfSignedCertificateInfo{}, err
	}
	return SelfSignedCertificateInfo{ServerName: sni, CertificatePath: certPath, KeyPath: keyPath, NotAfter: tmpl.NotAfter.UTC().Format(time.RFC3339), Created: true}, nil
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
		if len(parts) > 0 { dst["cipherSuites"] = parts }
	}
	if curves := stringSlice(src["curvePreferences"]); len(curves) > 0 { dst["curvePreferences"] = curves }
}

func validateCamouflageSNI(sni string) error {
	if sni == "" { return errors.New("SNI is required") }
	if len(sni) > 253 || strings.ContainsAny(sni, " /\\:@") { return errors.New("invalid SNI") }
	for _, label := range strings.Split(sni, ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") { return errors.New("invalid SNI") }
		for _, r := range label {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-') { return errors.New("invalid SNI") }
		}
	}
	return nil
}

func reusableCertificate(certPath, keyPath, sni string) (*x509.Certificate, bool) {
	if _, err := os.Stat(keyPath); err != nil { return nil, false }
	cert, err := readCertificate(certPath)
	if err != nil || cert.VerifyHostname(sni) != nil { return nil, false }
	if time.Until(cert.NotAfter) <= 30*24*time.Hour { return nil, false }
	return cert, true
}

func readCertificate(path string) (*x509.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil { return nil, err }
	block, _ := pem.Decode(data)
	if block == nil { return nil, errors.New("invalid certificate PEM") }
	return x509.ParseCertificate(block.Bytes)
}

func atomicSecretWrite(path string, data []byte, mode os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil { return err }
	if err := os.Chmod(tmp, mode); err != nil { _ = os.Remove(tmp); return err }
	if err := os.Rename(tmp, path); err != nil { _ = os.Remove(tmp); return err }
	return nil
}
