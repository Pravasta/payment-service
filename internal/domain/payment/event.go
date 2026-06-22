package payment

import (
	"time"

	"github.com/google/uuid"
)

// EventType untuk transaction_event (append-only, detailed-design §2.6).
type EventType string

const (
	EventCreated           EventType = "payment.created"
	EventPending           EventType = "payment.pending"
	EventPaid              EventType = "payment.paid"
	EventFailed            EventType = "payment.failed"
	EventExpired           EventType = "payment.expired"
	EventRefunded          EventType = "payment.refunded"
	EventPartiallyRefunded EventType = "payment.partially_refunded"
)

// TransactionEvent adalah catatan immutable (TIDAK PERNAH di-update/delete) —
// audit trail sekaligus sumber reporting.
type TransactionEvent struct {
	ID             uuid.UUID
	TransactionID  uuid.UUID
	AppID          uuid.UUID
	EventType      EventType
	FromStatus     Status
	ToStatus       Status
	AmountMinor    int64
	Source         string // "api" | "webhook" | "reconciler" | "manual"
	GatewayEventID string // DOKU Request-Id — dedup webhook
	Payload        map[string]any
	OccurredAt     time.Time
	CreatedAt      time.Time
}
