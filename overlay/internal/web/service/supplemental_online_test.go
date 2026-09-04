package service

import "testing"

func TestSupplementalOnlineGraceWindow(t *testing.T) {
	resetSupplementalOnlineState()
	const t0 int64 = 1_000_000
	refreshSupplementalOnlineAt([]string{"alice@example.com"}, []string{"mieru-1"}, t0)

	emails, tags := supplementalOnlineSnapshotAt(t0 + onlineGracePeriodMs - 1)
	if len(emails) != 1 || emails[0] != "alice@example.com" {
		t.Fatalf("supplemental email disappeared inside grace window: %#v", emails)
	}
	if len(tags) != 1 || tags[0] != "mieru-1" {
		t.Fatalf("supplemental inbound disappeared inside grace window: %#v", tags)
	}

	emails, tags = supplementalOnlineSnapshotAt(t0 + onlineGracePeriodMs + 1)
	if len(emails) != 0 || len(tags) != 0 {
		t.Fatalf("stale supplemental state was not pruned: emails=%#v tags=%#v", emails, tags)
	}
}

func TestSupplementalOnlineRefreshExtendsOnlySeenEntries(t *testing.T) {
	resetSupplementalOnlineState()
	const t0 int64 = 2_000_000
	refreshSupplementalOnlineAt([]string{"alice@example.com", "bob@example.com"}, []string{"sbox-a", "sbox-b"}, t0)
	refreshSupplementalOnlineAt([]string{"alice@example.com"}, []string{"sbox-a"}, t0+onlineGracePeriodMs/2)

	emails, tags := supplementalOnlineSnapshotAt(t0 + onlineGracePeriodMs + 1)
	if len(emails) != 1 || emails[0] != "alice@example.com" {
		t.Fatalf("refresh/prune produced wrong emails: %#v", emails)
	}
	if len(tags) != 1 || tags[0] != "sbox-a" {
		t.Fatalf("refresh/prune produced wrong tags: %#v", tags)
	}
}
