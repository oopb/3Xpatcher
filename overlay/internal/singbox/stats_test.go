package singbox

import (
	"testing"

	statsService "github.com/xtls/xray-core/app/stats/command"
)

func TestApplyStatsSnapshotDelta(t *testing.T) {
	resetStatsState()
	base := []*statsService.Stat{
		{Name: "inbound>>>sb-anytls-1>>>traffic>>>uplink", Value: 100},
		{Name: "inbound>>>sb-anytls-1>>>traffic>>>downlink", Value: 200},
		{Name: "user>>>alice@example.com>>>traffic>>>uplink", Value: 30},
		{Name: "user>>>alice@example.com>>>traffic>>>downlink", Value: 40},
	}
	if ts, cs, _, _, err := applyStatsSnapshot(base); err != nil || len(ts) != 0 || len(cs) != 0 {
		t.Fatalf("baseline must not be billed: ts=%v cs=%v err=%v", ts, cs, err)
	}
	next := []*statsService.Stat{
		{Name: "inbound>>>sb-anytls-1>>>traffic>>>uplink", Value: 130},
		{Name: "inbound>>>sb-anytls-1>>>traffic>>>downlink", Value: 250},
		{Name: "user>>>alice@example.com>>>traffic>>>uplink", Value: 35},
		{Name: "user>>>alice@example.com>>>traffic>>>downlink", Value: 47},
	}
	ts, cs, emails, tags, err := applyStatsSnapshot(next)
	if err != nil { t.Fatal(err) }
	if len(ts) != 1 || ts[0].Up != 30 || ts[0].Down != 50 || ts[0].Tag != "sb-anytls-1" {
		t.Fatalf("unexpected inbound delta: %#v", ts)
	}
	if len(cs) != 1 || cs[0].Email != "alice@example.com" || cs[0].Up != 5 || cs[0].Down != 7 {
		t.Fatalf("unexpected client delta: %#v", cs)
	}
	if len(emails) != 1 || len(tags) != 1 { t.Fatalf("activity missing: %v %v", emails, tags) }
}

func TestApplyStatsSnapshotCoreRestart(t *testing.T) {
	resetStatsState()
	_, _, _, _, _ = applyStatsSnapshot([]*statsService.Stat{{Name: "user>>>u>>>traffic>>>uplink", Value: 100}})
	_, cs, _, _, err := applyStatsSnapshot([]*statsService.Stat{{Name: "user>>>u>>>traffic>>>uplink", Value: 9}})
	if err != nil { t.Fatal(err) }
	if len(cs) != 1 || cs[0].Up != 9 { t.Fatalf("counter reset must count new bytes once: %#v", cs) }
}
