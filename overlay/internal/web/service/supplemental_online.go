package service

import (
	"sort"
	"sync"
	"time"
)

// supplementalOnlineState keeps local online/activity signals produced by
// supplemental runtimes (sing-box and Mieru) independently of the Xray process.
//
// 3x-ui historically stores the local online cache on xray.Process. That works
// for Xray and mtproto while an Xray process exists, but it makes supplemental
// protocols disappear from /client/onlines and the dashboard when Xray is not
// running. Keep the supplemental signal at the service layer and merge it in the
// public getters instead.
var supplementalOnlineState = struct {
	sync.Mutex
	emails map[string]int64
	tags   map[string]int64
}{
	emails: make(map[string]int64),
	tags:   make(map[string]int64),
}

// RefreshSupplementalOnlineClients adds this poll's active supplemental users
// and inbound tags, then ages out stale entries using the same grace window as
// the native Xray online cache. Passing nil only prunes stale entries.
func (s *InboundService) RefreshSupplementalOnlineClients(activeEmails, activeInboundTags []string) {
	refreshSupplementalOnlineAt(activeEmails, activeInboundTags, time.Now().UnixMilli())
}

func refreshSupplementalOnlineAt(activeEmails, activeInboundTags []string, now int64) {
	supplementalOnlineState.Lock()
	defer supplementalOnlineState.Unlock()

	if supplementalOnlineState.emails == nil {
		supplementalOnlineState.emails = make(map[string]int64)
	}
	if supplementalOnlineState.tags == nil {
		supplementalOnlineState.tags = make(map[string]int64)
	}
	for _, email := range activeEmails {
		if email != "" {
			supplementalOnlineState.emails[email] = now
		}
	}
	for _, tag := range activeInboundTags {
		if tag != "" {
			supplementalOnlineState.tags[tag] = now
		}
	}
	pruneSupplementalOnlineLocked(now)
}

func pruneSupplementalOnlineLocked(now int64) {
	cutoff := now - onlineGracePeriodMs
	for email, lastSeen := range supplementalOnlineState.emails {
		if lastSeen < cutoff {
			delete(supplementalOnlineState.emails, email)
		}
	}
	for tag, lastSeen := range supplementalOnlineState.tags {
		if lastSeen < cutoff {
			delete(supplementalOnlineState.tags, tag)
		}
	}
}

func supplementalOnlineSnapshot() ([]string, []string) {
	return supplementalOnlineSnapshotAt(time.Now().UnixMilli())
}

func supplementalOnlineSnapshotAt(now int64) ([]string, []string) {
	supplementalOnlineState.Lock()
	defer supplementalOnlineState.Unlock()
	pruneSupplementalOnlineLocked(now)

	emails := make([]string, 0, len(supplementalOnlineState.emails))
	for email := range supplementalOnlineState.emails {
		emails = append(emails, email)
	}
	tags := make([]string, 0, len(supplementalOnlineState.tags))
	for tag := range supplementalOnlineState.tags {
		tags = append(tags, tag)
	}
	sort.Strings(emails)
	sort.Strings(tags)
	return emails, tags
}

func resetSupplementalOnlineState() {
	supplementalOnlineState.Lock()
	defer supplementalOnlineState.Unlock()
	supplementalOnlineState.emails = make(map[string]int64)
	supplementalOnlineState.tags = make(map[string]int64)
}
