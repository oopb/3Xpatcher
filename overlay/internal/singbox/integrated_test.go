package singbox

import "testing"

func TestInstallTLSFromPersistedFileCertificateShape(t *testing.T) {
	settings := map[string]any{}
	stream := `{"security":"tls","tlsSettings":{"serverName":"example.com","alpn":["h2"],"certificates":[{"certificateFile":"/etc/x-ui/server.crt","keyFile":"/etc/x-ui/server.key"}]}}`
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
