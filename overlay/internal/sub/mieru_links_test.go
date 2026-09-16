package sub

import (
	"net/url"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestBuildMieruSimpleLinkPrimaryUDP(t *testing.T) {
	inbound := &model.Inbound{Port: 45678, Remark: "Mieru UDP"}
	settings := map[string]any{
		"transport":           "UDP",
		"portRangeEnd":        float64(0),
		"mtu":                 float64(1400),
		"clientMultiplexing":  "MULTIPLEXING_LOW",
		"clientHandshakeMode": "HANDSHAKE_STANDARD",
	}
	svc := &SubService{}
	link := svc.buildMieruSimpleLink(
		inbound,
		model.Client{Email: "udp-user", Password: "udp-pass"},
		settings,
		nil,
		"mieru.example",
		45678,
	)
	if link == "" {
		t.Fatal("expected Mieru UDP link")
	}
	u, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	if got := u.Query()["port"]; len(got) != 1 || got[0] != "45678" {
		t.Fatalf("ports = %#v, want [45678]", got)
	}
	if got := u.Query()["protocol"]; len(got) != 1 || got[0] != "UDP" {
		t.Fatalf("protocols = %#v, want [UDP]; link=%s", got, link)
	}
}

func TestMieruShareBindingsPrimaryTransportCaseInsensitive(t *testing.T) {
	inbound := &model.Inbound{Port: 45678}
	for _, raw := range []string{"UDP", "udp", "Udp"} {
		settings := map[string]any{"transport": raw}
		bindings := mieruShareBindings(inbound, settings)
		if len(bindings) != 1 || bindings[0].Transport != "UDP" {
			t.Fatalf("transport %q => %#v, want UDP", raw, bindings)
		}
	}
}
