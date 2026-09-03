package singbox

import (
	"os"
	"testing"
)

func TestInstallTLSFromPersistedFileCertificateShape(t *testing.T) {
	settings := map[string]any{}
	stream := `{"security":"tls","tlsSettings":{"serverName":"example.com","minVersion":"1.2","maxVersion":"1.3","curvePreferences":["X25519"],"alpn":["h2"],"certificates":[{"certificateFile":"/etc/x-ui/server.crt","keyFile":"/etc/x-ui/server.key"}]}}`
	if err := installTLSFromStream(settings, stream); err != nil { t.Fatal(err) }
	tlsMap, ok := settings["tls"].(map[string]any); if !ok { t.Fatalf("tls map missing: %#v", settings) }
	if tlsMap["certificatePath"] != "/etc/x-ui/server.crt" || tlsMap["keyPath"] != "/etc/x-ui/server.key" { t.Fatalf("persisted file certificate not translated: %#v", tlsMap) }
	if tlsMap["minVersion"] != "1.2" || tlsMap["maxVersion"] != "1.3" { t.Fatalf("TLS versions not translated: %#v", tlsMap) }
}

func TestInstallTLSFromPersistedInlineCertificateShape(t *testing.T) {
	settings := map[string]any{}
	stream := `{"security":"tls","tlsSettings":{"certificates":[{"certificate":["CERT"],"key":["KEY"]}]}}`
	if err := installTLSFromStream(settings, stream); err != nil { t.Fatal(err) }
	tlsMap := settings["tls"].(map[string]any)
	cert, ok := tlsMap["certificate"].([]string); if !ok || len(cert) != 1 || cert[0] != "CERT" { t.Fatalf("inline certificate not translated: %#v", tlsMap) }
}

func TestInstallGeneratedCamouflageSNICertificate(t *testing.T) {
	old := supplementalCertBase
	supplementalCertBase = t.TempDir()
	defer func() { supplementalCertBase = old }()
	settings := map[string]any{"tlsMode": "self_signed_sni", "camouflageSNI": "www.microsoft.com", "selfSignedValidityDays": float64(365)}
	stream := `{"security":"tls","tlsSettings":{"alpn":["h2","http/1.1"],"minVersion":"1.2","maxVersion":"1.3","certificates":[]}}`
	if err := installTLSFromStream(settings, stream); err != nil { t.Fatal(err) }
	tlsMap := settings["tls"].(map[string]any)
	if tlsMap["serverName"] != "www.microsoft.com" { t.Fatalf("wrong SNI: %#v", tlsMap) }
	certPath, _ := tlsMap["certificatePath"].(string); keyPath, _ := tlsMap["keyPath"].(string)
	if _, err := os.Stat(certPath); err != nil { t.Fatalf("generated cert missing: %v", err) }
	if _, err := os.Stat(keyPath); err != nil { t.Fatalf("generated key missing: %v", err) }
}
