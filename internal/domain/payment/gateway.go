package payment

import (
	"context"
	"time"
)

// Gateway adalah port anti-corruption layer ke payment gateway
// (detailed-design §6.1 / architecture-review §6.1). Inti PS hanya bicara model
// canonical ini; detail DOKU/Midtrans/Xendit disembunyikan di adapter.
type Gateway interface {
	// CreateCharge membuat checkout di gateway. DOKU mengembalikan payment_url
	// secara sinkron (doku-integration-spec §2).
	CreateCharge(ctx context.Context, req ChargeRequest) (ChargeResult, error)

	// ParseWebhook memverifikasi signature & memetakan payload mentah ke event canonical.
	ParseWebhook(ctx context.Context, raw WebhookPayload) (WebhookEvent, error)

	// GetStatus menanyakan status terkini ke gateway (untuk reconciler & /sync).
	GetStatus(ctx context.Context, ref StatusRef) (StatusResult, error)

	// Refund mengeksekusi refund (penuh/sebagian). Bisa sync atau async tergantung
	// channel (doku-integration-spec §3, §9.1).
	Refund(ctx context.Context, req RefundRequest) (RefundResult, error)
}

// ChargeRequest adalah input canonical untuk membuat pembayaran.
type ChargeRequest struct {
	ExternalReference string
	AmountMinor       int64
	Currency          string
	CurrencyExponent  int
	CustomerRef       string
	CustomerEmail     string
	CustomerName      string
	Description       string
	ExpiryMinutes     int
	ReturnURL         string
	Metadata          map[string]any
}

// ChargeResult adalah hasil canonical pembuatan pembayaran.
type ChargeResult struct {
	GatewayTxnID     string
	GatewayRequestID string
	PaymentURL       string
	Status           Status
	ExpiresAt        *time.Time
	Raw              map[string]any
}

// WebhookPayload adalah notifikasi mentah dari gateway (untuk verifikasi signature).
type WebhookPayload struct {
	Headers map[string]string
	RawBody []byte
	URLPath string // Request-Target untuk verifikasi signature (spec §1)
}

// WebhookEvent adalah event canonical hasil parsing webhook.
type WebhookEvent struct {
	GatewayEventID    string // DOKU Request-Id notifikasi (dedup gateway_event_id)
	ExternalReference string // order.invoice_number
	GatewayTxnID      string // identifier transaksi DOKU (bila ada)
	OriginalRequestID string // DOKU original_request_id == gateway_request_id kita (match utama)
	Status            Status
	PaymentMethod     string // dari channel.id (spec §9.3)
	AmountMinor       int64
	OccurredAt        time.Time
	Raw               map[string]any
}

// StatusRef mengidentifikasi transaksi untuk GetStatus.
type StatusRef struct {
	ExternalReference string
	GatewayRequestID  string
}

// StatusResult adalah status canonical dari GetStatus.
type StatusResult struct {
	Status        Status
	PaymentMethod string
	AmountMinor   int64
	Raw           map[string]any
}

// RefundType memetakan jenis refund DOKU (spec §3).
type RefundType string

const (
	RefundTypeVoid    RefundType = "VOID"
	RefundTypePartial RefundType = "PARTIAL_REFUND"
	RefundTypeFull    RefundType = "FULL_REFUND"
)

// RefundRequest adalah input canonical refund.
type RefundRequest struct {
	ExternalReference string
	OriginalRequestID string // DOKU payment.original_request_id
	AmountMinor       int64
	Currency          string
	Reason            string
	Type              RefundType
}

// RefundResult adalah hasil canonical refund.
type RefundResult struct {
	GatewayRefundID string
	Status          RefundStatus // sync: succeeded/failed; async: pending
	Raw             map[string]any
}
