package model

import (
	"time"

	"github.com/google/uuid"
)

// NotificationOutbox: callback PS -> app yang andal (detailed-design §2.8).
// Worker memproses status=pending AND next_retry_at<=now (lihat issue 0009).
type NotificationOutbox struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey"`
	AppID         uuid.UUID `gorm:"type:uuid;not null;index"`
	TransactionID uuid.UUID `gorm:"type:uuid;not null;index"`
	EventID       string    `gorm:"not null;uniqueIndex"` // dikirim ke app utk dedup
	EventType     string    `gorm:"not null"`
	Payload       JSONMap   `gorm:"type:jsonb"`
	Status        string    `gorm:"not null;default:'pending';index:idx_outbox_due,priority:1"` // pending|delivered|failed|dead
	Attempts      int       `gorm:"not null;default:0"`
	NextRetryAt   time.Time `gorm:"index:idx_outbox_due,priority:2"`
	LastError     string
	CreatedAt     time.Time
	DeliveredAt   *time.Time
}

func (NotificationOutbox) TableName() string { return "notification_outbox" }
