package model

import (
	"time"

	"github.com/google/uuid"
)

// GatewayAccount: kredensial PS -> gateway (detailed-design §2.4).
// config_enc menyimpan client_id & secret_key DOKU terenkripsi (AES-GCM, lihat §9).
type GatewayAccount struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	Gateway     string    `gorm:"not null;index:idx_gateway_env,priority:1"` // "doku"
	Environment string    `gorm:"not null;index:idx_gateway_env,priority:2"` // sandbox | production
	ConfigEnc   []byte    `gorm:"type:bytea"`                                // terenkripsi
	Status      string    `gorm:"not null;default:'active'"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (GatewayAccount) TableName() string { return "gateway_account" }
