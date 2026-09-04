package mieru

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
	"gorm.io/gorm"
)

const mieruOnlineWindow = 20 * time.Second

var mieruStatsState = struct {
	sync.Mutex
	initialized map[int]bool
	last        map[int]map[string]int64
}{initialized: make(map[int]bool), last: make(map[int]map[string]int64)}

func (r Runtime) command(id int, args ...string) ([]byte, error) {
	r = r.normalized()
	cmd := exec.Command(r.BinaryPath, args...)
	cmd.Env = append(os.Environ(),
		"MITA_CONFIG_JSON_FILE="+r.configPath(id),
		"MITA_UDS_PATH="+r.udsPath(id),
		"MITA_LOG_NO_TIMESTAMP=true",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("mita %s for inbound #%d: %w: %s", strings.Join(args, " "), id, err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

// CollectTraffic queries every enabled local Mieru instance. Mieru exposes
// cumulative per-user counters through its official management RPC/CLI; this
// function turns them into the delta shape already consumed by 3x-ui's native
// InboundService.AddTraffic path. LastActive from `mita get users` feeds the
// panel's existing online grace-window model.
func CollectTraffic(db *gorm.DB) ([]*xray.Traffic, []*xray.ClientTraffic, []string, []string, error) {
	if db == nil {
		return nil, nil, nil, nil, errors.New("database is unavailable")
	}
	var rows []model.Inbound
	if err := db.Model(&model.Inbound{}).
		Where("enable = ? AND node_id IS NULL AND protocol = ?", true, model.Mieru).
		Order("id ASC").Find(&rows).Error; err != nil {
		return nil, nil, nil, nil, err
	}

	rt := DefaultRuntime()
	inboundTraffic := make([]*xray.Traffic, 0, len(rows))
	clientByEmail := make(map[string]*xray.ClientTraffic)
	activeEmails := make(map[string]struct{})
	activeTags := make(map[string]struct{})
	var errs []error

	for i := range rows {
		metricsOut, err := rt.command(rows[i].Id, "get", "metrics")
		if err != nil {
			errs = append(errs, err)
			continue
		}
		metrics, err := parseMetricsJSON(metricsOut)
		if err != nil {
			errs = append(errs, fmt.Errorf("Mieru inbound #%d metrics: %w", rows[i].Id, err))
			continue
		}
		delta := mieruMetricDelta(rows[i].Id, metrics)

		up := delta["traffic\x00UploadBytes"]
		down := delta["traffic\x00DownloadBytes"]
		if up > 0 || down > 0 {
			inboundTraffic = append(inboundTraffic, &xray.Traffic{IsInbound: true, Tag: rows[i].Tag, Up: up, Down: down})
			activeTags[rows[i].Tag] = struct{}{}
		}
		for key, value := range delta {
			if value <= 0 || !strings.HasPrefix(key, "user - ") {
				continue
			}
			parts := strings.SplitN(key, "\x00", 2)
			if len(parts) != 2 {
				continue
			}
			email := strings.TrimPrefix(parts[0], "user - ")
			if email == "" {
				continue
			}
			ct := clientByEmail[email]
			if ct == nil {
				ct = &xray.ClientTraffic{Email: email}
				clientByEmail[email] = ct
			}
			switch parts[1] {
			case "UploadBytes":
				ct.Up += value
			case "DownloadBytes":
				ct.Down += value
			default:
				continue
			}
			activeEmails[email] = struct{}{}
			activeTags[rows[i].Tag] = struct{}{}
		}

		usersOut, userErr := rt.command(rows[i].Id, "get", "users")
		if userErr != nil {
			errs = append(errs, userErr)
			continue
		}
		for _, email := range parseRecentUsers(usersOut, time.Now(), mieruOnlineWindow) {
			activeEmails[email] = struct{}{}
			activeTags[rows[i].Tag] = struct{}{}
		}
	}

	clients := make([]*xray.ClientTraffic, 0, len(clientByEmail))
	for _, c := range clientByEmail {
		clients = append(clients, c)
	}
	emails := make([]string, 0, len(activeEmails))
	for email := range activeEmails {
		emails = append(emails, email)
	}
	tags := make([]string, 0, len(activeTags))
	for tag := range activeTags {
		tags = append(tags, tag)
	}
	return inboundTraffic, clients, emails, tags, errors.Join(errs...)
}

// parseMetricsJSON normalizes mita's official metrics layout into the flat
// group shape consumed by mieruMetricDelta. mita serializes ordinary groups as
// e.g. {"traffic":{"UploadBytes":1}} but nests user counters under a special
// top-level "users" object: {"users":{"alice":{"UploadBytes":2}}}.
// Older code attempted to unmarshal the whole document directly into
// map[string]map[string]int64, which fails as soon as the nested users object is
// present and therefore prevented all Mieru accounting on real multi-user
// configurations.
func parseMetricsJSON(out []byte) (map[string]map[string]int64, error) {
	text := strings.TrimSpace(string(out))
	start := strings.IndexByte(text, '{')
	end := strings.LastIndexByte(text, '}')
	if start < 0 || end < start {
		return nil, fmt.Errorf("JSON object not found in output %q", text)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text[start:end+1]), &raw); err != nil {
		return nil, err
	}
	metrics := make(map[string]map[string]int64, len(raw))
	for group, payload := range raw {
		if group == "users" {
			var users map[string]map[string]int64
			if err := json.Unmarshal(payload, &users); err != nil {
				return nil, fmt.Errorf("decode users metrics: %w", err)
			}
			for user, counters := range users {
				if strings.TrimSpace(user) == "" {
					continue
				}
				metrics["user - "+user] = counters
			}
			continue
		}
		var counters map[string]int64
		if err := json.Unmarshal(payload, &counters); err != nil {
			return nil, fmt.Errorf("decode metric group %q: %w", group, err)
		}
		metrics[group] = counters
	}
	return metrics, nil
}

func mieruMetricDelta(id int, snapshot map[string]map[string]int64) map[string]int64 {
	mieruStatsState.Lock()
	defer mieruStatsState.Unlock()
	prev := mieruStatsState.last[id]
	if prev == nil {
		prev = make(map[string]int64)
	}
	baseline := !mieruStatsState.initialized[id]
	mieruStatsState.initialized[id] = true
	next := make(map[string]int64)
	delta := make(map[string]int64)
	for group, metrics := range snapshot {
		for name, value := range metrics {
			if value < 0 {
				continue
			}
			key := group + "\x00" + name
			next[key] = value
			if baseline {
				continue
			}
			old, ok := prev[key]
			if !ok || value < old {
				old = 0
			}
			if value > old {
				delta[key] = value - old
			}
		}
	}
	mieruStatsState.last[id] = next
	return delta
}

func parseRecentUsers(out []byte, now time.Time, window time.Duration) []string {
	seen := make(map[string]struct{})
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		for i, field := range fields {
			ts, err := time.Parse(time.RFC3339, field)
			if err != nil || i == 0 {
				continue
			}
			email := fields[i-1]
			if email == "" || email == "User" || now.Sub(ts) < 0 || now.Sub(ts) > window {
				continue
			}
			seen[email] = struct{}{}
			break
		}
	}
	outUsers := make([]string, 0, len(seen))
	for email := range seen {
		outUsers = append(outUsers, email)
	}
	return outUsers
}

func resetMieruStatsState() {
	mieruStatsState.Lock()
	defer mieruStatsState.Unlock()
	mieruStatsState.initialized = make(map[int]bool)
	mieruStatsState.last = make(map[int]map[string]int64)
}
