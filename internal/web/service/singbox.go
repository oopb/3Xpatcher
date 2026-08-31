package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	sbox "github.com/mhsanaei/3x-ui/v3/internal/singbox"
	"gorm.io/gorm"
)

type SingboxService struct {
	Runtime sbox.Runtime
}

func NewSingboxService() *SingboxService {
	return &SingboxService{Runtime: sbox.DefaultRuntime()}
}

func (s *SingboxService) GetInbounds(userID int) ([]model.SingboxInbound, error) {
	var rows []model.SingboxInbound
	err := database.GetDB().Where("user_id = ?", userID).Order("id ASC").Find(&rows).Error
	return rows, err
}

func (s *SingboxService) GetInbound(userID, id int) (model.SingboxInbound, error) {
	var row model.SingboxInbound
	err := database.GetDB().Where("user_id = ? AND id = ?", userID, id).First(&row).Error
	return row, err
}

func (s *SingboxService) AddInbound(row model.SingboxInbound) (model.SingboxInbound, error) {
	autoTag := strings.TrimSpace(row.Tag) == ""
	if autoTag {
		// Temporary unique tag; replaced after the row obtains its numeric id.
		row.Tag = fmt.Sprintf("sbox-%s-p%d-u%d", row.Protocol, row.Port, row.UserId)
	}
	if err := row.Validate(); err != nil {
		return row, err
	}
	db := database.GetDB()
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		// IDs are stable and make auto-generated tags collision-proof.
		if autoTag {
			row.Tag = fmt.Sprintf("sbox-%s-%d", row.Protocol, row.Id)
			if err := tx.Model(&row).Update("tag", row.Tag).Error; err != nil {
				return err
			}
		}
		return s.applyAll(tx)
	})
	return row, err
}

func (s *SingboxService) UpdateInbound(userID, id int, next model.SingboxInbound) (model.SingboxInbound, error) {
	db := database.GetDB()
	var result model.SingboxInbound
	err := db.Transaction(func(tx *gorm.DB) error {
		var old model.SingboxInbound
		if err := tx.Where("user_id = ? AND id = ?", userID, id).First(&old).Error; err != nil {
			return err
		}
		next.Id, next.UserId = old.Id, old.UserId
		if strings.TrimSpace(next.Tag) == "" {
			next.Tag = old.Tag
		}
		if err := next.Validate(); err != nil {
			return err
		}
		if err := tx.Save(&next).Error; err != nil {
			return err
		}
		result = next
		return s.applyAll(tx)
	})
	return result, err
}

func (s *SingboxService) DeleteInbound(userID, id int) error {
	return database.GetDB().Transaction(func(tx *gorm.DB) error {
		res := tx.Where("user_id = ? AND id = ?", userID, id).Delete(&model.SingboxInbound{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return s.applyAll(tx)
	})
}

func (s *SingboxService) SetEnable(userID, id int, enabled bool) error {
	return database.GetDB().Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.SingboxInbound{}).Where("user_id = ? AND id = ?", userID, id).Update("enable", enabled)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return s.applyAll(tx)
	})
}

func (s *SingboxService) CheckDatabaseConfig() error {
	var rows []model.SingboxInbound
	if err := database.GetDB().Where("enable = ?", true).Order("id ASC").Find(&rows).Error; err != nil {
		return err
	}
	cfg, err := buildRows(rows)
	if err != nil {
		return err
	}
	return s.Runtime.CheckBytes(cfg)
}

func (s *SingboxService) Restart() error { return s.Runtime.Restart() }
func (s *SingboxService) Status() (string, string, error) {
	status, statusErr := s.Runtime.Status()
	version, versionErr := s.Runtime.Version()
	if statusErr != nil && versionErr != nil {
		return status, version, errors.Join(statusErr, versionErr)
	}
	if statusErr != nil {
		return status, version, statusErr
	}
	if versionErr != nil {
		return status, version, versionErr
	}
	return status, version, nil
}

func (s *SingboxService) applyAll(tx *gorm.DB) error {
	var rows []model.SingboxInbound
	if err := tx.Where("enable = ?", true).Order("id ASC").Find(&rows).Error; err != nil {
		return err
	}
	cfg, err := buildRows(rows)
	if err != nil {
		return err
	}
	return s.Runtime.Apply(cfg)
}

func buildRows(rows []model.SingboxInbound) ([]byte, error) {
	records := make([]sbox.InboundRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, row.ToSingboxRecord())
	}
	return sbox.BuildConfig(records)
}
