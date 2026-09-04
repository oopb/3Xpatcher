package mieru

import (
	"testing"
	"time"
)

func TestMieruMetricDelta(t *testing.T) {
	resetMieruStatsState()
	base := map[string]map[string]int64{
		"traffic":                  {"UploadBytes": 100, "DownloadBytes": 200},
		"user - alice@example.com": {"UploadBytes": 10, "DownloadBytes": 20},
	}
	if d := mieruMetricDelta(7, base); len(d) != 0 {
		t.Fatalf("baseline billed: %#v", d)
	}
	next := map[string]map[string]int64{
		"traffic":                  {"UploadBytes": 130, "DownloadBytes": 260},
		"user - alice@example.com": {"UploadBytes": 17, "DownloadBytes": 31},
	}
	d := mieruMetricDelta(7, next)
	if d["traffic\x00UploadBytes"] != 30 || d["traffic\x00DownloadBytes"] != 60 {
		t.Fatalf("bad traffic delta: %#v", d)
	}
	if d["user - alice@example.com\x00UploadBytes"] != 7 || d["user - alice@example.com\x00DownloadBytes"] != 11 {
		t.Fatalf("bad user delta: %#v", d)
	}
}

func TestParseMetricsJSONWithLogPrefix(t *testing.T) {
	m, err := parseMetricsJSON([]byte("INFO metrics follows\n{\"traffic\":{\"UploadBytes\":12,\"DownloadBytes\":34}}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if m["traffic"]["UploadBytes"] != 12 || m["traffic"]["DownloadBytes"] != 34 {
		t.Fatalf("bad parse: %#v", m)
	}
}

func TestParseMetricsJSONOfficialUsersLayout(t *testing.T) {
	out := []byte(`INFO {"traffic":{"UploadBytes":100,"DownloadBytes":200},"users":{"alice@example.com":{"UploadBytes":11,"DownloadBytes":22},"bob@example.com":{"UploadBytes":33,"DownloadBytes":44}}}`)
	m, err := parseMetricsJSON(out)
	if err != nil {
		t.Fatal(err)
	}
	if got := m["user - alice@example.com"]["UploadBytes"]; got != 11 {
		t.Fatalf("alice upload = %d, want 11; parsed=%#v", got, m)
	}
	if got := m["user - bob@example.com"]["DownloadBytes"]; got != 44 {
		t.Fatalf("bob download = %d, want 44; parsed=%#v", got, m)
	}
}

func TestParseRecentUsers(t *testing.T) {
	now := time.Date(2026, 9, 3, 10, 0, 20, 0, time.UTC)
	out := []byte("User  LastActive  1DayDown  1DayUp\nalice@example.com  2026-09-03T10:00:15Z  1 KiB  2 KiB\nbob@example.com  2026-09-03T09:59:00Z  0 B  0 B\n")
	users := parseRecentUsers(out, now, 20*time.Second)
	if len(users) != 1 || users[0] != "alice@example.com" {
		t.Fatalf("unexpected recent users: %#v", users)
	}
}
