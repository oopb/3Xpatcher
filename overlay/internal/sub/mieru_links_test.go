package sub

import (
	"net/url"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestBuildMieruSimpleLinkPrimaryTransport(t *testing.T) {
	for _, transport := range []string{"TCP", "UDP"} {
		t.Run(transport, func(t *testing.T) {
			inbound := &model.Inbound{Port: 45678, Remark: "Mieru " + transport}
			settings := map[string]any{
				"transport":           transport,
				"portRangeEnd":        float64(0),
				"mtu":                 float64(1400),
				"clientMultiplexing":  "MULTIPLEXING_LOW",
				"clientHandshakeMode": "HANDSHAKE_STANDARD",
			}
			svc := &SubService{}
			link := svc.buildMieruSimpleLink(
				inbound,
				model.Client{Email: "transport-user", Password: "transport-pass"},
				settings,
				nil,
				"mieru.example",
				45678,
			)
			if link == "" {
				t.Fatalf("expected Mieru %s link", transport)
			}
			u, err := url.Parse(link)
			if err != nil {
				t.Fatal(err)
			}
			if got := u.Query()["port"]; len(got) != 1 || got[0] != "45678" {
				t.Fatalf("ports = %#v, want [45678]", got)
			}
			if got := u.Query()["protocol"]; len(got) != 1 || got[0] != transport {
				t.Fatalf("protocols = %#v, want [%s]; link=%s", got, transport, link)
			}
		})
	}
}

func TestMieruShareBindingsPrimaryTransportCaseInsensitive(t *testing.T) {
	inbound := &model.Inbound{Port: 45678}
	for _, tc := range []struct {
		raw  string
		want string
	}{
		{raw: "TCP", want: "TCP"},
		{raw: "tcp", want: "TCP"},
		{raw: "Tcp", want: "TCP"},
		{raw: "UDP", want: "UDP"},
		{raw: "udp", want: "UDP"},
		{raw: "Udp", want: "UDP"},
	} {
		settings := map[string]any{"transport": tc.raw}
		bindings := mieruShareBindings(inbound, settings)
		if len(bindings) != 1 || bindings[0].Transport != tc.want {
			t.Fatalf("transport %q => %#v, want %s", tc.raw, bindings, tc.want)
		}
	}
}

func TestBuildMieruClashProxyTransportFollowsBinding(t *testing.T) {
	clash := NewSubClashService(false, "", NewSubService(""))
	subReq := NewSubService("")
	inbound := &model.Inbound{Port: 45678, Remark: "Mieru"}
	client := model.Client{Email: "transport-user", Password: "transport-pass"}
	settings := map[string]any{
		"clientMultiplexing":  "MULTIPLEXING_LOW",
		"clientHandshakeMode": "HANDSHAKE_STANDARD",
	}
	ep := map[string]any{"dest": "mieru.example", "port": float64(45678)}

	for _, transport := range []string{"TCP", "UDP"} {
		t.Run(transport, func(t *testing.T) {
			proxy := clash.buildMieruProxyForBinding(
				subReq,
				inbound,
				client,
				ep,
				settings,
				mieruShareBinding{Port: 45678, Transport: transport},
				false,
			)
			if got := proxy["transport"]; got != transport {
				t.Fatalf("transport = %#v, want %s; proxy=%#v", got, transport, proxy)
			}
			if got := proxy["udp"]; got != true {
				t.Fatalf("udp = %#v, want true (UDP Associate capability); proxy=%#v", got, proxy)
			}
		})
	}
}
