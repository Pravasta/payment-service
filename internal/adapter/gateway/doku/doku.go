package doku

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	domain "github.com/Pravasta/payment-service/internal/domain/payment"
)

// checkoutPath adalah Request-Target Direct API DOKU Checkout (Generate Payment).
const checkoutPath = "/checkout/v1/payment"

// maxAmountDigits: DOKU membatasi order.amount maksimal 16 digit (spec §4).
const maxAmount int64 = 9_999_999_999_999_999 // 16 digit '9'

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

	// now & newRequestID dapat di-override pada test untuk hasil deterministik.
	now          func() time.Time
	newRequestID func() string
}

func New(cfg Config) *Adapter {
	return &Adapter{
		cfg:          cfg,
		client:       &http.Client{Timeout: 30 * time.Second},
		now:          time.Now,
		newRequestID: uuid.NewString,
	}
}

// pastikan memenuhi kontrak port.
var _ domain.Gateway = (*Adapter)(nil)

// --- DTO request/response Checkout DOKU (anti-corruption layer) ---

type checkoutRequest struct {
	Order    checkoutOrder     `json:"order"`
	Payment  checkoutPayment   `json:"payment"`
	Customer *checkoutCustomer `json:"customer,omitempty"`
}

type checkoutOrder struct {
	Amount        int64  `json:"amount"` // IDR integer rupiah (exponent 0), JSON number
	InvoiceNumber string `json:"invoice_number"`
	Currency      string `json:"currency"`
	CallbackURL   string `json:"callback_url,omitempty"`
}

type checkoutPayment struct {
	PaymentDueDate int `json:"payment_due_date,omitempty"` // menit; kosong → default DOKU (60)
}

type checkoutCustomer struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
	Phone string `json:"phone,omitempty"`
}

type checkoutResponse struct {
	Message json.RawMessage `json:"message"` // string atau []string
	Order   struct {
		InvoiceNumber string `json:"invoice_number"`
		Amount        string `json:"amount"`
		SessionID     string `json:"session_id"`
	} `json:"order"`
	Payment struct {
		TokenID        string `json:"token_id"`
		URL            string `json:"url"`
		Status         string `json:"status"`
		ExpiredDate    string `json:"expired_date"`     // yyyyMMddHHmmss (WIB)
		ExpiredDateUTC string `json:"expired_date_utc"` // yyyyMMddHHmmss (UTC)
	} `json:"payment"`
}

// CreateCharge memanggil DOKU Checkout (Generate Payment). DOKU mengembalikan
// payment.url SINKRON pada response yang sama (spec §2). Alur canonical
// created → pending terjadi dalam satu panggilan ini.
func (a *Adapter) CreateCharge(ctx context.Context, req domain.ChargeRequest) (domain.ChargeResult, error) {
	if req.AmountMinor <= 0 {
		return domain.ChargeResult{}, fmt.Errorf("doku.CreateCharge: amount harus > 0")
	}
	if req.AmountMinor > maxAmount {
		return domain.ChargeResult{}, fmt.Errorf("doku.CreateCharge: amount melebihi batas DOKU (maks 16 digit)")
	}
	if req.ExternalReference == "" {
		return domain.ChargeResult{}, fmt.Errorf("doku.CreateCharge: external_reference (invoice_number) wajib")
	}
	if len(req.ExternalReference) > 64 {
		return domain.ChargeResult{}, fmt.Errorf("doku.CreateCharge: external_reference maksimal 64 karakter")
	}

	currency := req.Currency
	if currency == "" {
		currency = "IDR"
	}

	payload := checkoutRequest{
		Order: checkoutOrder{
			Amount:        req.AmountMinor,
			InvoiceNumber: req.ExternalReference,
			Currency:      currency,
			CallbackURL:   req.ReturnURL,
		},
		Payment: checkoutPayment{PaymentDueDate: req.ExpiryMinutes},
	}
	if req.CustomerName != "" || req.CustomerEmail != "" {
		payload.Customer = &checkoutCustomer{
			Name:  req.CustomerName,
			Email: req.CustomerEmail,
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return domain.ChargeResult{}, fmt.Errorf("doku.CreateCharge: marshal body: %w", err)
	}

	resp, err := a.postSigned(ctx, checkoutPath, body)
	if err != nil {
		return domain.ChargeResult{}, err
	}
	if resp.statusCode < 200 || resp.statusCode >= 300 {
		return domain.ChargeResult{}, parseError(resp.statusCode, resp.body)
	}

	var parsed checkoutResponse
	if err := json.Unmarshal(resp.body, &parsed); err != nil {
		return domain.ChargeResult{}, fmt.Errorf("doku.CreateCharge: parse response: %w", err)
	}
	if parsed.Payment.URL == "" {
		return domain.ChargeResult{}, fmt.Errorf("doku.CreateCharge: response tanpa payment.url")
	}

	var raw map[string]any
	_ = json.Unmarshal(resp.body, &raw)

	return domain.ChargeResult{
		GatewayTxnID:     gatewayTxnID(parsed),
		GatewayRequestID: resp.requestID, // Request-Id yang kita kirim — dipakai GetStatus/Refund
		PaymentURL:       parsed.Payment.URL,
		Status:           domain.StatusPending, // checkout dibuat; menunggu pembayaran
		ExpiresAt:        parseCheckoutExpiry(parsed),
		Raw:              raw,
	}, nil
}

// gatewayTxnID memilih identifier transaksi DOKU dari response.
func gatewayTxnID(p checkoutResponse) string {
	if p.Payment.TokenID != "" {
		return p.Payment.TokenID
	}
	return p.Order.SessionID
}

// parseCheckoutExpiry mengembalikan expiry sebagai UTC. Prefer expired_date_utc;
// fallback expired_date (diinterpretasikan WIB/UTC+7) → dikonversi ke UTC.
func parseCheckoutExpiry(p checkoutResponse) *time.Time {
	const layout = "20060102150405"
	if s := p.Payment.ExpiredDateUTC; s != "" {
		if t, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			return &t
		}
	}
	if s := p.Payment.ExpiredDate; s != "" {
		wib := time.FixedZone("WIB", 7*3600)
		if t, err := time.ParseInLocation(layout, s, wib); err == nil {
			u := t.UTC()
			return &u
		}
	}
	return nil
}

// dokuNotification adalah bentuk body notifikasi DOKU (subset yang dipakai).
type dokuNotification struct {
	Order struct {
		InvoiceNumber string `json:"invoice_number"`
		Amount        string `json:"amount"`
	} `json:"order"`
	Transaction struct {
		Status            string `json:"status"`
		Date              string `json:"date"`
		OriginalRequestID string `json:"original_request_id"`
	} `json:"transaction"`
	Service struct {
		ID string `json:"id"`
	} `json:"service"`
	Channel struct {
		ID string `json:"id"`
	} `json:"channel"`
	Acquirer struct {
		ID string `json:"id"`
	} `json:"acquirer"`
}

// ParseWebhook memverifikasi signature DOKU (skema HMAC-SHA256 komponen, spec §1)
// lalu memetakan body notifikasi → WebhookEvent canonical.
//
// Mengembalikan domain.ErrInvalidSignature bila signature tidak valid.
func (a *Adapter) ParseWebhook(_ context.Context, raw domain.WebhookPayload) (domain.WebhookEvent, error) {
	sig := raw.Headers["Signature"]
	params := SignatureParams{
		ClientID:         raw.Headers["Client-Id"],
		RequestID:        raw.Headers["Request-Id"],
		RequestTimestamp: raw.Headers["Request-Timestamp"],
		RequestTarget:    raw.URLPath,
		RawBody:          raw.RawBody,
	}
	if !VerifySignature(a.cfg.SecretKey, sig, params) {
		return domain.WebhookEvent{}, domain.ErrInvalidSignature
	}

	var n dokuNotification
	if err := json.Unmarshal(raw.RawBody, &n); err != nil {
		return domain.WebhookEvent{}, fmt.Errorf("doku.ParseWebhook: parse body: %w", err)
	}

	var rawMap map[string]any
	_ = json.Unmarshal(raw.RawBody, &rawMap)

	return domain.WebhookEvent{
		GatewayEventID:    raw.Headers["Request-Id"], // dedup
		ExternalReference: n.Order.InvoiceNumber,
		OriginalRequestID: n.Transaction.OriginalRequestID, // == gateway_request_id kita
		Status:            mapStatus(n.Transaction.Status),
		PaymentMethod:     n.Channel.ID, // spec §9.3
		AmountMinor:       parseAmountMinor(n.Order.Amount),
		OccurredAt:        parseNotifTime(n.Transaction.Date, raw.Headers["Request-Timestamp"]),
		Raw:               rawMap,
	}, nil
}

// parseAmountMinor mem-parse amount DOKU ("50000" atau "50000.00") menjadi minor
// unit. Untuk IDR (exponent 0) hanya bagian integer yang bermakna.
func parseAmountMinor(s string) int64 {
	if s == "" {
		return 0
	}
	if i := strings.IndexByte(s, '.'); i >= 0 {
		s = s[:i]
	}
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// parseNotifTime mencoba beberapa format waktu DOKU; fallback ke timestamp header lalu now.
func parseNotifTime(date, headerTS string) time.Time {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z", "20060102150405"} {
		if date != "" {
			if t, err := time.Parse(layout, date); err == nil {
				return t.UTC()
			}
		}
	}
	if t, err := time.Parse(dokuTimeFormat, headerTS); err == nil {
		return t.UTC()
	}
	return time.Now().UTC()
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
