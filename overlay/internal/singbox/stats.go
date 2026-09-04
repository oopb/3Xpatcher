package singbox

import (
	"context"
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
	statsService "github.com/xtls/xray-core/app/stats/command"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	defaultStatsListenAddress = "127.0.0.1:62789"
	statsListenAddressFile    = "/etc/3xpatcher/singbox-stats.addr"
	queryStatsMethod          = "/v2ray.core.app.stats.command.StatsService/QueryStats"
)

// StatsListenAddress is intentionally loopback-only. The installer persists an
// available address so 3Xpatcher does not collide with unrelated local services
// that happen to use the historical 62789 port. The same value is used both by
// the sing-box config renderer and by the panel-side stats collector.
var StatsListenAddress = loadStatsListenAddress(statsListenAddressFile)

func loadStatsListenAddress(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return defaultStatsListenAddress
	}
	addr := strings.TrimSpace(string(raw))
	host, portRaw, err := net.SplitHostPort(addr)
	if err != nil || host != "127.0.0.1" {
		return defaultStatsListenAddress
	}
	port, err := strconv.Atoi(portRaw)
	if err != nil || port < 1 || port > 65535 {
		return defaultStatsListenAddress
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

var (
	statsTrafficRE = regexp.MustCompile(`^(inbound)>>>([^>]+)>>>traffic>>>(downlink|uplink)$`)
	statsClientRE  = regexp.MustCompile(`^user>>>([^>]+)>>>traffic>>>(downlink|uplink)$`)
	statsState     = struct {
		sync.Mutex
		initialized bool
		last        map[string]int64
	}{last: make(map[string]int64)}
)

// CollectTraffic returns byte deltas since the previous successful poll plus
// the users/inbounds that moved traffic during this poll. The first successful
// poll is baseline-only so a panel restart never re-adds counters already held
// by the still-running sing-box process.
func CollectTraffic() ([]*xray.Traffic, []*xray.ClientTraffic, []string, []string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(StatsListenAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("sing-box stats dial: %w", err)
	}
	defer conn.Close()

	resp := new(statsService.QueryStatsResponse)
	if err := conn.Invoke(ctx, queryStatsMethod, &statsService.QueryStatsRequest{Reset_: false}, resp); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("sing-box stats query: %w", err)
	}
	return applyStatsSnapshot(resp.GetStat())
}

func applyStatsSnapshot(stats []*statsService.Stat) ([]*xray.Traffic, []*xray.ClientTraffic, []string, []string, error) {
	statsState.Lock()
	defer statsState.Unlock()

	baseline := !statsState.initialized
	statsState.initialized = true
	inboundMap := make(map[string]*xray.Traffic)
	clientMap := make(map[string]*xray.ClientTraffic)
	activeUsers := make(map[string]struct{})
	activeTags := make(map[string]struct{})
	seen := make(map[string]struct{}, len(stats))

	for _, stat := range stats {
		if stat == nil || stat.Name == "" || stat.Value < 0 {
			continue
		}
		seen[stat.Name] = struct{}{}
		prev, ok := statsState.last[stat.Name]
		statsState.last[stat.Name] = stat.Value
		if baseline {
			continue
		}
		if !ok || stat.Value < prev {
			prev = 0 // new counter or sing-box restart
		}
		delta := stat.Value - prev
		if delta <= 0 {
			continue
		}
		if m := statsTrafficRE.FindStringSubmatch(stat.Name); len(m) == 4 {
			t := inboundMap[m[2]]
			if t == nil {
				t = &xray.Traffic{IsInbound: true, Tag: m[2]}
				inboundMap[m[2]] = t
			}
			if m[3] == "downlink" {
				t.Down += delta
			} else {
				t.Up += delta
			}
			activeTags[m[2]] = struct{}{}
			continue
		}
		if m := statsClientRE.FindStringSubmatch(stat.Name); len(m) == 3 {
			ct := clientMap[m[1]]
			if ct == nil {
				ct = &xray.ClientTraffic{Email: m[1]}
				clientMap[m[1]] = ct
			}
			if m[2] == "downlink" {
				ct.Down += delta
			} else {
				ct.Up += delta
			}
			activeUsers[m[1]] = struct{}{}
		}
	}

	// Prune baselines for deleted users/inbounds without churning the map every
	// five seconds in the steady state.
	if len(statsState.last) > 2*len(seen)+16 {
		for name := range statsState.last {
			if _, ok := seen[name]; !ok {
				delete(statsState.last, name)
			}
		}
	}

	traffics := make([]*xray.Traffic, 0, len(inboundMap))
	for _, t := range inboundMap {
		traffics = append(traffics, t)
	}
	clients := make([]*xray.ClientTraffic, 0, len(clientMap))
	for _, c := range clientMap {
		clients = append(clients, c)
	}
	emails := make([]string, 0, len(activeUsers))
	for email := range activeUsers {
		emails = append(emails, email)
	}
	tags := make([]string, 0, len(activeTags))
	for tag := range activeTags {
		tags = append(tags, tag)
	}
	return traffics, clients, emails, tags, nil
}

// resetStatsState is test-only but intentionally unexported so production code
// cannot accidentally discard a live baseline.
func resetStatsState() {
	statsState.Lock()
	defer statsState.Unlock()
	statsState.initialized = false
	statsState.last = make(map[string]int64)
}
