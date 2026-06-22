package model

import (
	"time"

	"github.com/google/uuid"
)

// WebhookInbox menyimpan notifikasi mentah dari gateway apa adanya untuk
// audit/replay (detailed-design §2.7). Disimpan SEBELUM verifikasi signature.
type WebhookInbox struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey"`
	Gateway       string     `gorm:"not null;index"`
	Signature     string     // header Signature DOKU
	Headers       JSONMap    `gorm:"type:jsonb"`
	RawBody       []byte     `gorm:"type:bytea"` // untuk hitung Digest & audit
	Verified      bool       `gorm:"not null;default:false"`
	Processed     bool       `gorm:"not null;default:false;index"`
	TransactionID *uuid.UUID `gorm:"type:uuid;index"` // nullable sampai termatch
	ReceivedAt    time.Time  `gorm:"not null"`
}

func (WebhookInbox) TableName() string { return "webhook_inbox" }
