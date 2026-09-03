package service

import (
	"encoding/json"
	"testing"
)

func TestNormalizeShadowsocks2022KeysRepairsServerAndClient(t *testing.T) {
	settings := `{"method":"2022-blake3-aes-128-gcm","password":"browser-autofill","clients":[{"email":"u","password":"also-not-base64"}]}`
	healed, changed := normalizeShadowsocks2022Keys(settings)
	if !changed { t.Fatal("expected invalid SS2022 keys to be regenerated") }
	var m struct {
		Method string `json:"method"`
		Password string `json:"password"`
		Clients []struct { Email string `json:"email"`; Password string `json:"password"` } `json:"clients"`
	}
	if err := json.Unmarshal([]byte(healed), &m); err != nil { t.Fatal(err) }
	if !validShadowsocksClientKey(m.Method, m.Password) { t.Fatalf("server PSK still invalid: %q", m.Password) }
	if len(m.Clients) != 1 || !validShadowsocksClientKey(m.Method, m.Clients[0].Password) { t.Fatalf("client PSK still invalid: %#v", m.Clients) }
}

func TestNormalizeShadowsocks2022KeysPreservesValidKeys(t *testing.T) {
	method := "2022-blake3-aes-128-gcm"
	server := randomShadowsocksClientKey(method)
	client := randomShadowsocksClientKey(method)
	settings := `{"method":"` + method + `","password":"` + server + `","clients":[{"email":"u","password":"` + client + `"}]}`
	healed, changed := normalizeShadowsocks2022Keys(settings)
	if changed { t.Fatalf("valid keys rotated unexpectedly: %s", healed) }
	if healed != settings { t.Fatalf("valid settings changed unexpectedly") }
}
