package singbox

import (
	"os"
	"testing"
)

func TestInstallTLSFromPersistedFileCertificateShape(t *testing.T) {
	settings := map[string]any{}
	stream := `{"security":"tls","tlsSettings":{"serverName":"example.com","minVersion":"1.2","maxVersion":"1.3","curvePreferences":["X25519"],"alpn":["h2"],"certificates":[{"certificateFile":"/etc/x-ui/server.crt","keyFile":"/etc/x-ui/server.key"}]}}`
	if err := installTLSFromStream(settings, stream); err != nil {
		t.Fatal(err)
	}
	tlsMap, ok := settings["tls"].(map[string]any)
	if !ok {
		t.Fatalf("tls map missing: %#v", settings)
	}
	if tlsMap["certificatePath"] != "/etc/x-ui/server.crt" || tlsMap["keyPath"] != "/etc/x-ui/server.key" {
		t.Fatalf("persisted file certificate not translated: %#v", tlsMap)
	}
	if tlsMap["minVersion"] != "1.2" || tlsMap["maxVersion"] != "1.3" {
		t.Fatalf("TLS versions not translated: %#v", tlsMap)
	}
}

func TestInstallTLSFromPersistedInlineCertificateShape(t *testing.T) {
	settings := map[string]any{}
	stream := `{"security":"tls","tlsSettings":{"certificates":[{"certificate":["CERT"],"key":["KEY"]}]}}`
	if err := installTLSFromStream(settings, stream); err != nil {
		t.Fatal(err)
	}
	tlsMap := settings["tls"].(map[string]any)
	cert, ok := tlsMap["certificate"].([]string)
	if !ok || len(cert) != 1 || cert[0] != "CERT" {
		t.Fatalf("inline certificate not translated: %#v", tlsMap)
	}
}

func TestInstallAnyTLSRealityFromNative3xui(t *testing.T) {
	settings := map[string]any{}
	stream := `{"security":"reality","realitySettings":{"target":"www.microsoft.com:443","serverNames":["www.microsoft.com","microsoft.com"],"privateKey":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","maxTimediff":60000,"shortIds":["0123456789abcdef"],"settings":{"publicKey":"BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB","fingerprint":"chrome"}}}`
	if err := installTLSOrRealityFromStream(settings, stream); err != nil {
		t.Fatal(err)
	}
	tlsMap := settings["tls"].(map[string]any)
	if tlsMap["serverName"] != "www.microsoft.com" {
		t.Fatalf("first native SNI was not reused: %#v", tlsMap)
	}
	reality := tlsMap["reality"].(map[string]any)
	if reality["handshakeServer"] != "www.microsoft.com" || reality["handshakePort"] != 443 {
		t.Fatalf("target not translated: %#v", reality)
	}
	if reality["maxTimeDifference"] != "60000ms" {
		t.Fatalf("maxTimediff must preserve native 3x-ui milliseconds: %#v", reality)
	}
	ids, ok := reality["shortIds"].([]string)
	if !ok || len(ids) != 1 || ids[0] != "0123456789abcdef" {
		t.Fatalf("short IDs not translated: %#v", reality)
	}
}

func TestInstallGeneratedSNICertificateFromTLSSettings(t *testing.T) {
	old := supplementalCertBase
	supplementalCertBase = t.TempDir()
	defer func() { supplementalCertBase = old }()
	settings := map[string]any{}
	stream := `{"security":"tls","tlsSettings":{"certificateMode":"self_signed_sni","serverName":"www.microsoft.com","selfSignedValidityDays":365,"alpn":["h2","http/1.1"],"minVersion":"1.2","maxVersion":"1.3","certificates":[]}}`
	if err := installTLSFromStream(settings, stream); err != nil {
		t.Fatal(err)
	}
	tlsMap := settings["tls"].(map[string]any)
	if tlsMap["serverName"] != "www.microsoft.com" {
		t.Fatalf("wrong SNI: %#v", tlsMap)
	}
	certPath, _ := tlsMap["certificatePath"].(string)
	keyPath, _ := tlsMap["keyPath"].(string)
	if _, err := os.Stat(certPath); err != nil {
		t.Fatalf("generated cert missing: %v", err)
	}
	if _, err := os.Stat(keyPath); err != nil {
		t.Fatalf("generated key missing: %v", err)
	}
}

func TestManualGenerateReturnsMetadataAndReuses(t *testing.T) {
	old := supplementalCertBase
	supplementalCertBase = t.TempDir()
	defer func() { supplementalCertBase = old }()
	first, err := GenerateSelfSignedCertificate("www.apple.com", 365)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Created || first.CertificatePath == "" || first.KeyPath == "" || first.NotAfter == "" {
		t.Fatalf("bad first result: %#v", first)
	}
	second, err := GenerateSelfSignedCertificate("www.apple.com", 365)
	if err != nil {
		t.Fatal(err)
	}
	if second.Created {
		t.Fatalf("expected reuse: %#v", second)
	}
	if first.CertificatePath != second.CertificatePath || first.KeyPath != second.KeyPath {
		t.Fatalf("paths changed: %#v %#v", first, second)
	}
}

func TestLegacy06SelfSignedFieldsStillWork(t *testing.T) {
	old := supplementalCertBase
	supplementalCertBase = t.TempDir()
	defer func() { supplementalCertBase = old }()
	settings := map[string]any{"tlsMode": "self_signed_sni", "camouflageSNI": "www.cloudflare.com", "selfSignedValidityDays": float64(90)}
	stream := `{"security":"tls","tlsSettings":{"certificates":[]}}`
	if err := installTLSFromStream(settings, stream); err != nil {
		t.Fatal(err)
	}
	if settings["tls"].(map[string]any)["serverName"] != "www.cloudflare.com" {
		t.Fatal("legacy SNI migration failed")
	}
}
