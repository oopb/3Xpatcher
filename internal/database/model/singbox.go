package model

import (
	"fmt"

	sbox "github.com/mhsanaei/3x-ui/v3/internal/singbox"
)

// SingboxInbound is intentionally separate from Inbound. Inbound remains an
// Xray-only model so Xray updates and normal 3x-ui behavior are not coupled to
// the supplemental sing-box core.
type SingboxInbound struct {
	Id        int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	UserId    int    `json:"-" gorm:"index"`
	Remark    string `json:"remark" form:"remark"`
	Enable    bool   `json:"enable" form:"enable" gorm:"index"`
	Listen    string `json:"listen" form:"listen"`
	Port      int    `json:"port" form:"port" gorm:"index"`
	Protocol  string `json:"protocol" form:"protocol" gorm:"index"`
	Settings  string `json:"settings" form:"settings"`
	Tag       string `json:"tag" form:"tag" gorm:"uniqueIndex"`
	CreatedAt int64  `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt int64  `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (SingboxInbound) TableName() string { return "singbox_inbounds" }

func (i SingboxInbound) Validate() error {
	if !sbox.IsSupportedProtocol(sbox.Protocol(i.Protocol)) {
		return fmt.Errorf("unsupported sing-box supplemental protocol %q", i.Protocol)
	}
	if i.Port < 1 || i.Port > 65535 {
		return fmt.Errorf("invalid port %d", i.Port)
	}
	return sbox.ValidateRecord(i.ToSingboxRecord())
}

func (i SingboxInbound) ToSingboxRecord() sbox.InboundRecord {
	return sbox.InboundRecord{
		ID: i.Id, Remark: i.Remark, Enable: i.Enable, Listen: i.Listen,
		Port: i.Port, Protocol: sbox.Protocol(i.Protocol), Tag: i.Tag,
		Settings: []byte(i.Settings),
	}
}
