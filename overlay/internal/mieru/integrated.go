package mieru

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
	"gorm.io/gorm"
)

func Reconcile() error {
	configs, err := BuildIntegratedConfigs(database.GetDB())
	if err != nil { return err }
	rt := DefaultRuntime()
	active := make(map[int]struct{}, len(configs))
	for id, cfg := range configs {
		active[id] = struct{}{}
		if err := rt.Apply(id, cfg); err != nil { return fmt.Errorf("Mieru inbound #%d: %w", id, err) }
	}
	existing, err := rt.ConfiguredIDs()
	if err != nil { return err }
	for _, id := range existing {
		if _, ok := active[id]; !ok {
			if err := rt.Remove(id); err != nil { return fmt.Errorf("remove stale Mieru inbound #%d: %w", id, err) }
		}
	}
	return nil
}

func BuildIntegratedConfigs(db *gorm.DB) (map[int][]byte, error) {
	if db == nil { return nil, fmt.Errorf("database is unavailable") }
	var rows []model.Inbound
	if err := db.Model(&model.Inbound{}).Where("enable = ? AND node_id IS NULL AND protocol = ?", true, model.Mieru).Order("id ASC").Find(&rows).Error; err != nil { return nil, err }
	out := make(map[int][]byte)
	for i := range rows {
		clients, err := activeMieruClients(db, rows[i].Id)
		if err != nil { return nil, fmt.Errorf("inbound %q (#%d): %w", rows[i].Remark, rows[i].Id, err) }
		if len(clients) == 0 { continue }
		var settings Settings
		if strings.TrimSpace(rows[i].Settings) != "" {
			if err := json.Unmarshal([]byte(rows[i].Settings), &settings); err != nil { return nil, fmt.Errorf("inbound %q (#%d): invalid settings JSON: %w", rows[i].Remark, rows[i].Id, err) }
		}
		users := make([]User, 0, len(clients))
		for _, c := range clients {
			if strings.TrimSpace(c.Password) == "" { return nil, fmt.Errorf("inbound %q (#%d): client %q has no password", rows[i].Remark, rows[i].Id, c.Email) }
			users = append(users, User{Name: c.Email, Password: c.Password})
		}
		cfg, err := BuildServerConfig(Record{ID: rows[i].Id, Remark: rows[i].Remark, Port: rows[i].Port, Settings: settings, Users: users})
		if err != nil { return nil, fmt.Errorf("inbound %q (#%d): %w", rows[i].Remark, rows[i].Id, err) }
		out[rows[i].Id] = cfg
	}
	return out, nil
}

func activeMieruClients(db *gorm.DB, inboundID int) ([]model.ClientRecord, error) {
	var clients []model.ClientRecord
	if err := db.Table("clients AS c").Select("c.*").Joins("JOIN client_inbounds AS ci ON ci.client_id = c.id").Where("ci.inbound_id = ?", inboundID).Order("ci.created_at ASC, c.id ASC").Scan(&clients).Error; err != nil { return nil, err }
	if len(clients) == 0 { return clients, nil }
	emails := make([]string, 0, len(clients)); for i := range clients { emails = append(emails, clients[i].Email) }
	var traffics []xray.ClientTraffic
	if err := db.Where("email IN ?", emails).Find(&traffics).Error; err != nil { return nil, err }
	trafficEnabled := make(map[string]bool, len(traffics)); for _, st := range traffics { trafficEnabled[strings.ToLower(st.Email)] = st.Enable }
	now := time.Now().UnixMilli(); filtered := clients[:0]
	for _, c := range clients {
		if !c.Enable { continue }
		if c.ExpiryTime > 0 && c.ExpiryTime <= now { continue }
		if enabled, exists := trafficEnabled[strings.ToLower(c.Email)]; exists && !enabled { continue }
		filtered = append(filtered, c)
	}
	return filtered, nil
}
