package job

import (
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	mcore "github.com/mhsanaei/3x-ui/v3/internal/mieru"
	sbox "github.com/mhsanaei/3x-ui/v3/internal/singbox"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/websocket"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

type SupplementalTrafficJob struct {
	inboundService service.InboundService
}

func NewSupplementalTrafficJob() *SupplementalTrafficJob { return new(SupplementalTrafficJob) }

func (j *SupplementalTrafficJob) Run() {
	var traffics []*xray.Traffic
	clientMap := make(map[string]*xray.ClientTraffic)
	activeEmails := make(map[string]struct{})
	activeTags := make(map[string]struct{})

	merge := func(ts []*xray.Traffic, cs []*xray.ClientTraffic, emails, tags []string) {
		traffics = append(traffics, ts...)
		for _, c := range cs {
			if c == nil || c.Email == "" {
				continue
			}
			dst := clientMap[c.Email]
			if dst == nil {
				dst = &xray.ClientTraffic{Email: c.Email}
				clientMap[c.Email] = dst
			}
			dst.Up += c.Up
			dst.Down += c.Down
		}
		for _, email := range emails {
			if email != "" {
				activeEmails[email] = struct{}{}
			}
		}
		for _, tag := range tags {
			if tag != "" {
				activeTags[tag] = struct{}{}
			}
		}
	}

	if ts, cs, emails, tags, err := sbox.CollectTraffic(); err != nil {
		logger.Debug("supplemental sing-box stats unavailable:", err)
	} else {
		merge(ts, cs, emails, tags)
	}
	if ts, cs, emails, tags, err := mcore.CollectTraffic(database.GetDB()); err != nil {
		logger.Debug("supplemental Mieru stats partially unavailable:", err)
		merge(ts, cs, emails, tags)
	} else {
		merge(ts, cs, emails, tags)
	}

	clients := make([]*xray.ClientTraffic, 0, len(clientMap))
	for _, c := range clientMap {
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

	if len(traffics) > 0 || len(clients) > 0 {
		needRestart, clientsDisabled, err := j.inboundService.AddTraffic(traffics, clients)
		if err != nil {
			logger.Warning("add supplemental traffic failed:", err)
		} else if needRestart || clientsDisabled {
			if err := sbox.Reconcile(); err != nil {
				logger.Warning("reconcile sing-box after supplemental traffic update failed:", err)
			}
			if err := mcore.Reconcile(); err != nil {
				logger.Warning("reconcile Mieru after supplemental traffic update failed:", err)
			}
		}
	}

	// The native cache is grace-window based and accumulates activity from each
	// caller, so this unions supplemental activity with Xray instead of replacing it.
	j.inboundService.RefreshLocalOnlineClients(emails, tags)

	if !websocket.HasClients() || (len(traffics) == 0 && len(clients) == 0 && len(emails) == 0) {
		return
	}
	lastOnlineMap, err := j.inboundService.GetClientsLastOnline()
	if err != nil {
		lastOnlineMap = map[string]int64{}
	}
	websocket.BroadcastTraffic(map[string]any{
		"traffics":       traffics,
		"clientTraffics": clients,
		"onlineClients":  j.inboundService.GetOnlineClients(),
		"onlineByGuid":   j.inboundService.GetOnlineClientsByGuid(),
		"activeInbounds": j.inboundService.GetActiveInboundsByGuid(),
		"lastOnlineMap":  lastOnlineMap,
	})

	snapshot := true
	if total, countErr := j.inboundService.CountClientTraffics(); countErr == nil && total > clientStatsSnapshotMaxClients {
		snapshot = false
	}
	var stats []*xray.ClientTraffic
	if snapshot {
		stats, _ = j.inboundService.GetAllClientTraffics()
	} else {
		stats, _ = j.inboundService.GetActiveClientTraffics(emails)
	}
	payload := map[string]any{"snapshot": snapshot}
	if len(stats) > 0 {
		payload["clients"] = stats
	}
	if summary, summaryErr := j.inboundService.GetInboundsTrafficSummary(); summaryErr == nil && len(summary) > 0 {
		payload["inbounds"] = summary
	}
	if len(payload) > 1 {
		websocket.BroadcastClientStats(payload)
	}
}
