package doku

import (
	"context"
	"errors"
	"net/http"
	"time"

	domain "github.com/Pravasta/payment-service/internal/domain/payment"
)

// Config kredensial & endpoint DOKU (di-supply dari config + secret terenkripsi).
type Config struct {
	BaseURL   string // mis. https://api-sandbox.doku.com
	ClientID  string
	SecretKey string
}

// Adapter mengimplementasi domain.Gateway untuk DOKU.
type Adapter struct {
	cfg    Config
	client *http.Client
}

func New(cfg Config) *Adapter {
	return &Adapter{
		cfg:    cfg,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// pastikan memenuhi kontrak port.
var _ domain.Gateway = (*Adapter)(nil)

// CreateCharge memanggil DOKU Checkout (generate order). DOKU mengembalikan
// payment.url sinkron (spec §2). TODO(impl): bangun request bertanda tangan,
// kirim, map response -> ChargeResult.
func (a *Adapter) CreateCharge(ctx context.Context, req domain.ChargeRequest) (domain.ChargeResult, error) {
	return domain.ChargeResult{}, errors.New("doku.CreateCharge: belum diimplementasikan")
}

// ParseWebhook memverifikasi signature (crypto.go) lalu map payload -> WebhookEvent.
// TODO(impl): parse JSON body, mapStatus(transaction.status), isi PaymentMethod dari channel.id.
func (a *Adapter) ParseWebhook(ctx context.Context, raw domain.WebhookPayload) (domain.WebhookEvent, error) {
	return domain.WebhookEvent{}, errors.New("doku.ParseWebhook: belum diimplementasikan")
}

// GetStatus memanggil Check Status DOKU: GET /orders/v1/status/{invoice|request-id} (spec §5).
// TODO(impl): ingat saran DOKU cek >=60s setelah pembayaran.
func (a *Adapter) GetStatus(ctx context.Context, ref domain.StatusRef) (domain.StatusResult, error) {
	return domain.StatusResult{}, errors.New("doku.GetStatus: belum diimplementasikan")
}

// Refund memilih endpoint sesuai channel (spec §9.1) & jenis (VOID/PARTIAL/FULL).
// TODO(impl): kartu -> /cancellation/credit-card/refund; QRIS/e-wallet endpoint berbeda.
func (a *Adapter) Refund(ctx context.Context, req domain.RefundRequest) (domain.RefundResult, error) {
	return domain.RefundResult{}, errors.New("doku.Refund: belum diimplementasikan")
}

// mapStatus memetakan status notifikasi DOKU -> canonical (spec §5).
func mapStatus(dokuStatus string) domain.Status {
	switch dokuStatus {
	case "SUCCESS":
		return domain.StatusPaid
	case "PENDING", "REDIRECT":
		return domain.StatusPending
	case "FAILED", "TIMEOUT":
		return domain.StatusFailed
	case "EXPIRED":
		return domain.StatusExpired
	case "REFUNDED":
		return domain.StatusRefunded
	default:
		return domain.StatusPending
	}
}
