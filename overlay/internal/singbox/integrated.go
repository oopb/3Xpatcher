package singbox

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
	"gorm.io/gorm"
)

// Reconcile rebuilds the supplemental core from the canonical 3x-ui tables:
// inbounds, clients and client_inbounds. The old singbox_inbounds table is not
// consulted; it is intentionally left untouched as a rollback artifact.
func Reconcile() error {
	cfg, err := BuildIntegratedConfig(database.GetDB())
	if err != nil {
		return err
	}
	return DefaultRuntime().Apply(cfg)
}

func CheckIntegrated() error {
	cfg, err := BuildIntegratedConfig(database.GetDB())
	if err != nil {
		return err
	}
	return DefaultRuntime().CheckBytes(cfg)
}

func BuildIntegratedConfig(db *gorm.DB) ([]byte, error) {
	if db == nil {
		return nil, errors.New("database is unavailable")
	}
	var rows []model.Inbound
	protocols := []model.Protocol{model.TUIC, model.AnyTLS, model.ShadowTLS, model.Naive}
	if err := db.Model(&model.Inbound{}).
		Where("enable = ? AND node_id IS NULL AND protocol IN ?", true, protocols).
		Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}

	records := make([]InboundRecord, 0, len(rows))
	for i := range rows {
		rec, active, err := integratedRecord(db, &rows[i])
		if err != nil {
			return nil, fmt.Errorf("inbound %q (#%d): %w", rows[i].Remark, rows[i].Id, err)
		}
		// Native 3x-ui permits creating an inbound before attaching clients.
		// Keep the row visible/configurable but do not bind a listener until at
		// least one active native ClientRecord is attached.
		if !active {
			continue
		}
		records = append(records, rec)
	}
	return BuildConfig(records)
}

func integratedRecord(db *gorm.DB, inbound *model.Inbound) (InboundRecord, bool, error) {
	clients, err := integratedClients(db, inbound.Id)
	if err != nil {
		return InboundRecord{}, false, err
	}
	if len(clients) == 0 {
		return InboundRecord{}, false, nil
	}

	var settings map[string]any
	if strings.TrimSpace(inbound.Settings) == "" {
		settings = map[string]any{}
	} else if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
		return InboundRecord{}, false, fmt.Errorf("invalid settings JSON: %w", err)
	}
	if settings == nil {
		settings = map[string]any{}
	}
	// clients is a 3x-ui compatibility mirror. The canonical identities come
	// from clients/client_inbounds and users is rebuilt below every time.
	delete(settings, "clients")
	delete(settings, "users")

	switch inbound.Protocol {
	case model.TUIC:
		users := make([]TUICUser, 0, len(clients))
		for _, c := range clients {
			if strings.TrimSpace(c.UUID) == "" {
				return InboundRecord{}, false, fmt.Errorf("client %q has no UUID", c.Email)
			}
			if strings.TrimSpace(c.Password) == "" {
				return InboundRecord{}, false, fmt.Errorf("client %q has no password", c.Email)
			}
			users = append(users, TUICUser{Name: c.Email, UUID: c.UUID, Password: c.Password})
		}
		settings["users"] = users
		if err := installTLSFromStream(settings, inbound.StreamSettings); err != nil {
			return InboundRecord{}, false, err
		}
	case model.AnyTLS:
		users := make([]PasswordUser, 0, len(clients))
		for _, c := range clients {
			if strings.TrimSpace(c.Password) == "" {
				return InboundRecord{}, false, fmt.Errorf("client %q has no password", c.Email)
			}
			users = append(users, PasswordUser{Name: c.Email, Password: c.Password})
		}
		settings["users"] = users
		if err := installTLSFromStream(settings, inbound.StreamSettings); err != nil {
			return InboundRecord{}, false, err
		}
	case model.ShadowTLS:
		users := make([]PasswordUser, 0, len(clients))
		for _, c := range clients {
			if strings.TrimSpace(c.Password) == "" {
				return InboundRecord{}, false, fmt.Errorf("client %q has no password", c.Email)
			}
			users = append(users, PasswordUser{Name: c.Email, Password: c.Password})
		}
		settings["users"] = users
	case model.Naive:
		users := make([]NaiveUser, 0, len(clients))
		for _, c := range clients {
			if strings.TrimSpace(c.Password) == "" {
				return InboundRecord{}, false, fmt.Errorf("client %q has no password", c.Email)
			}
			users = append(users, NaiveUser{Username: c.Email, Password: c.Password})
		}
		settings["users"] = users
		if err := installTLSFromStream(settings, inbound.StreamSettings); err != nil {
			return InboundRecord{}, false, err
		}
	default:
		return InboundRecord{}, false, fmt.Errorf("unsupported supplemental protocol %q", inbound.Protocol)
	}

	wire, err := json.Marshal(settings)
	if err != nil {
		return InboundRecord{}, false, err
	}
	listen := strings.TrimSpace(inbound.Listen)
	if listen == "" {
		listen = "0.0.0.0"
	}
	return InboundRecord{
		ID:       inbound.Id,
		Remark:   inbound.Remark,
		Enable:   inbound.Enable,
		Listen:   listen,
		Port:     inbound.Port,
		Protocol: Protocol(inbound.Protocol),
		Tag:      inbound.Tag,
		Settings: wire,
	}, true, nil
}

func integratedClients(db *gorm.DB, inboundID int) ([]model.ClientRecord, error) {
	var clients []model.ClientRecord
	if err := db.Table("clients AS c").
		Select("c.*").
		Joins("JOIN client_inbounds AS ci ON ci.client_id = c.id").
		Where("ci.inbound_id = ?", inboundID).
		Order("ci.created_at ASC, c.id ASC").
		Scan(&clients).Error; err != nil {
		return nil, err
	}
	if len(clients) == 0 {
		return clients, nil
	}

	emails := make([]string, 0, len(clients))
	for i := range clients {
		emails = append(emails, clients[i].Email)
	}
	var traffics []xray.ClientTraffic
	if err := db.Where("email IN ?", emails).Find(&traffics).Error; err != nil {
		return nil, err
	}
	trafficEnabled := make(map[string]bool, len(traffics))
	for _, st := range traffics {
		trafficEnabled[strings.ToLower(st.Email)] = st.Enable
	}

	now := time.Now().UnixMilli()
	out := clients[:0]
	for _, c := range clients {
		if !c.Enable {
			continue
		}
		if c.ExpiryTime > 0 && c.ExpiryTime <= now {
			continue
		}
		if enabled, exists := trafficEnabled[strings.ToLower(c.Email)]; exists && !enabled {
			continue
		}
		out = append(out, c)
	}
	return out, nil
}

func installTLSFromStream(settings map[string]any, raw string) error {
	var stream map[string]any
	if err := json.Unmarshal([]byte(raw), &stream); err != nil {
		return fmt.Errorf("invalid TLS stream settings: %w", err)
	}
	if stream == nil || stream["security"] != "tls" {
		return errors.New("TLS must be enabled for this protocol")
	}
	tlsIn, _ := stream["tlsSettings"].(map[string]any)
	if tlsIn == nil {
		return errors.New("TLS settings are missing")
	}
	tlsOut := map[string]any{"enabled": true}
	copyString := func(src, dst string) {
		if v, ok := tlsIn[src].(string); ok && strings.TrimSpace(v) != "" {
			tlsOut[dst] = v
		}
	}
	copyString("serverName", "serverName")
	if alpn := stringSlice(tlsIn["alpn"]); len(alpn) > 0 {
		tlsOut["alpn"] = alpn
	}

	certs, _ := tlsIn["certificates"].([]any)
	if len(certs) == 0 {
		return errors.New("TLS certificate is required")
	}
	cert, _ := certs[0].(map[string]any)
	if cert == nil {
		return errors.New("invalid TLS certificate entry")
	}
	// useFile is a frontend-only helper and is deliberately stripped by
	// formValuesToWirePayload before streamSettings is persisted. Detect the
	// actual stored shape instead: file paths win when either path field is
	// present; otherwise read the inline PEM arrays.
	certificatePath, _ := cert["certificateFile"].(string)
	keyPath, _ := cert["keyFile"].(string)
	if strings.TrimSpace(certificatePath) != "" || strings.TrimSpace(keyPath) != "" {
		if strings.TrimSpace(certificatePath) == "" || strings.TrimSpace(keyPath) == "" {
			return errors.New("TLS certificate/key file path is required")
		}
		tlsOut["certificatePath"] = certificatePath
		tlsOut["keyPath"] = keyPath
	} else {
		certificate := stringSlice(cert["certificate"])
		key := stringSlice(cert["key"])
		if len(certificate) == 0 || len(key) == 0 {
			return errors.New("TLS certificate/key content is required")
		}
		tlsOut["certificate"] = certificate
		tlsOut["key"] = key
	}
	settings["tls"] = tlsOut
	return nil
}

func stringSlice(v any) []string {
	switch x := v.(type) {
	case []string:
		return x
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		if strings.TrimSpace(x) == "" {
			return nil
		}
		return strings.Split(x, "\n")
	default:
		return nil
	}
}
