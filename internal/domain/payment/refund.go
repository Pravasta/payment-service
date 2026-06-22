package payment

import (
	"time"

	"github.com/google/uuid"
)

// RefundStatus mengikuti state machine refund (detailed-design §3.2).
type RefundStatus string

const (
	RefundRequested RefundStatus = "requested"
	RefundPending   RefundStatus = "pending"
	RefundSucceeded RefundStatus = "succeeded"
	RefundFailed    RefundStatus = "failed"
)

// Refund merepresentasikan satu permintaan refund (penuh/sebagian).
type Refund struct {
	ID              uuid.UUID
	AppID           uuid.UUID
	TransactionID   uuid.UUID
	IdempotencyKey  string
	AmountMinor     int64
	Currency        string
	Status          RefundStatus
	GatewayRefundID string
	Reason          string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
