// Package payment adalah domain layer (enterprise business rules) untuk
// pembayaran. TIDAK boleh bergantung pada framework, DB, atau HTTP — hanya
// entitas, value object, state machine, dan port (interface).
package payment

import (
	"time"

	"github.com/google/uuid"
)

// Status adalah canonical state machine lintas-gateway (detailed-design §3.1).
type Status string

const (
	StatusCreated            Status = "created"
	StatusPending            Status = "pending"
	StatusPaid               Status = "paid"
	StatusSettled            Status = "settled"
	StatusExpired            Status = "expired"
	StatusFailed             Status = "failed"
	StatusPartiallyRefunded  Status = "partially_refunded"
	StatusRefunded           Status = "refunded"
)

// allowedTransitions mendefinisikan transisi maju yang sah. Transisi di luar ini
// ditolak (anti out-of-order/duplikat — mis. webhook `expired` setelah `paid`).
var allowedTransitions = map[Status]map[Status]bool{
	StatusCreated: {StatusPending: true, StatusFailed: true},
	StatusPending: {StatusPaid: true, StatusExpired: true, StatusFailed: true},
	StatusPaid:    {StatusSettled: true, StatusPartiallyRefunded: true, StatusRefunded: true},
	StatusSettled: {StatusPartiallyRefunded: true, StatusRefunded: true},
	StatusPartiallyRefunded: {StatusPartiallyRefunded: true, StatusRefunded: true},
	// terminal: expired, failed, refunded
}

// CanTransition melaporkan apakah perpindahan from->to diizinkan.
func CanTransition(from, to Status) bool {
	return allowedTransitions[from][to]
}

// IsTerminal melaporkan apakah status tidak punya transisi keluar.
func IsTerminal(s Status) bool {
	return len(allowedTransitions[s]) == 0
}

// Transaction adalah source of truth fakta pembayaran (detailed-design §2.5).
// Uang selalu dalam minor unit (bigint); untuk IDR exponent=0 (rupiah utuh).
type Transaction struct {
	ID                uuid.UUID
	AppID             uuid.UUID
	ExternalReference string // id milik app (mis. invoice_id) — UNIQUE per app
	IdempotencyKey    string
	Status            Status

	// uang
	Currency         string // ISO-4217, mis. "IDR"
	CurrencyExponent int    // IDR=0, USD=2
	GrossAmount      int64  // minor unit
	FeeAmount        int64
	NetAmount        int64
	RefundedAmount   int64

	// gateway
	Gateway          string // "doku"
	GatewayTxnID     string // id transaksi di DOKU
	GatewayRequestID string // DOKU Request-Id — wajib untuk GetStatus & Refund (spec §3,§9)
	PaymentURL       string // url redirect checkout (sinkron dari DOKU)
	PaymentMethod    string // diisi dari notifikasi channel.id (spec §9.3)

	// konteks opaque untuk reporting (PS tidak menafsirkan)
	CustomerRef string
	Metadata    map[string]any
	Description string

	// lifecycle
	ExpiresAt  *time.Time // diisi dari expired_date_utc DOKU (spec §6), bukan lokal
	PaidAt     *time.Time
	SettledAt  *time.Time
	FailedAt   *time.Time
	ExpiredAt  *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// RefundableAmount mengembalikan sisa yang masih bisa di-refund.
func (t *Transaction) RefundableAmount() int64 {
	return t.GrossAmount - t.RefundedAmount
}
