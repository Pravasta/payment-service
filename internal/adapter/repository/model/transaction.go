// Package model berisi GORM model (DB representation) — TERPISAH dari entitas
// domain agar perubahan skema DB tidak bocor ke business rules.
package model

import (
	"time"

	"github.com/google/uuid"
)

// Transaction adalah GORM model untuk tabel `transaction` (detailed-design §2.5).
type Transaction struct {
	ID                uuid.UUID `gorm:"type:uuid;primaryKey"`
	AppID             uuid.UUID `gorm:"type:uuid;not null;index:idx_txn_app_status_created,priority:1"`
	ExternalReference string    `gorm:"not null;uniqueIndex:uq_txn_app_extref,priority:2"`
	IdempotencyKey    string    `gorm:"uniqueIndex:uq_txn_app_idem,priority:2"`
	Status            string    `gorm:"not null;index:idx_txn_app_status_created,priority:2"`

	Currency         string `gorm:"type:char(3);not null;default:'IDR'"`
	CurrencyExponent int16  `gorm:"not null;default:0"`
	GrossAmount      int64  `gorm:"not null"`
	FeeAmount        int64  `gorm:"not null;default:0"`
	NetAmount        int64  `gorm:"not null;default:0"`
	RefundedAmount   int64  `gorm:"not null;default:0"`

	Gateway          string `gorm:"not null;index:idx_txn_gateway_txnid,priority:1"`
	GatewayTxnID     string `gorm:"index:idx_txn_gateway_txnid,priority:2"`
	GatewayRequestID string
	PaymentURL       string
	PaymentMethod    string

	CustomerRef string
	Metadata    JSONMap `gorm:"type:jsonb"`
	Description string

	ExpiresAt *time.Time
	PaidAt    *time.Time
	SettledAt *time.Time
	FailedAt  *time.Time
	ExpiredAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time

	// app_id + external_reference unik; app_id + idempotency_key unik.
}

func (Transaction) TableName() string { return "transaction" }

// TransactionEvent adalah GORM model untuk tabel append-only `transaction_event` (§2.6).
type TransactionEvent struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	TransactionID  uuid.UUID `gorm:"type:uuid;not null;index"`
	AppID          uuid.UUID `gorm:"type:uuid;not null"`
	EventType      string    `gorm:"not null"`
	FromStatus     string
	ToStatus       string
	AmountMinor    int64
	Source         string
	GatewayEventID string `gorm:"index"` // dedup webhook; partial UNIQUE WHERE NOT NULL dibuat di cmd/migrate

	Payload    JSONMap `gorm:"type:jsonb"`
	OccurredAt time.Time
	CreatedAt  time.Time
}

func (TransactionEvent) TableName() string { return "transaction_event" }

// Refund adalah GORM model untuk tabel `refund` (§2.9).
type Refund struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey"`
	AppID           uuid.UUID `gorm:"type:uuid;not null"`
	TransactionID   uuid.UUID `gorm:"type:uuid;not null;index"`
	IdempotencyKey  string    `gorm:"uniqueIndex:uq_refund_app_idem,priority:2"`
	AmountMinor     int64     `gorm:"not null"`
	Currency        string    `gorm:"type:char(3);not null"`
	Status          string    `gorm:"not null"`
	GatewayRefundID string
	Reason          string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (Refund) TableName() string { return "refund" }
