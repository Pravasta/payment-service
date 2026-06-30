package doku

import (
	"context"
	"encoding/json"
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

// gatewayName adalah label gateway untuk metrik/observability.
const gatewayName = "doku"

// Observer adalah port observability opsional untuk adapter (di-inject dari cmd).
// Dideklarasikan di sisi konsumen agar adapter tidak meng-import package metrics.
type Observer interface {
	ObserveCharge(gateway string, success bool, dur time.Duration)
	ObserveWebhook(gateway string, ok bool)
}

// Option mengonfigurasi Adapter saat konstruksi (functional options).
type Option func(*Adapter)

// WithObserver memasang observer metrik. Tanpa ini, instrumentasi di-skip (nil-safe).
func WithObserver(o Observer) Option {
	return func(a *Adapter) { a.obs = o }
}

// Adapter mengimplementasi domain.Gateway untuk DOKU.
type Adapter struct {
	cfg    Config
	client *http.Client
	obs    Observer

	// now & newRequestID dapat di-override pada test untuk hasil deterministik.
	now          func() time.Time
	newRequestID func() string
}

func New(cfg Config, opts ...Option) *Adapter {
	a := &Adapter{
		cfg:          cfg,
		client:       &http.Client{Timeout: 30 * time.Second},
		now:          time.Now,
		newRequestID: uuid.NewString,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
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

// checkoutResponse: DOKU membungkus payload sukses di dalam objek "response"
// (order/payment ada di dalamnya), bukan di level atas. "message" tetap di atas.
type checkoutResponse struct {
	Message  json.RawMessage `json:"message"` // string atau []string
	Response struct {
		Order struct {
			InvoiceNumber string `json:"invoice_number"`
			Amount        string `json:"amount"`
			SessionID     string `json:"session_id"`
		} `json:"order"`
		Payment struct {
			TokenID         string `json:"token_id"`
			URL             string `json:"url"`
			Status          string `json:"status"`
			ExpiredDate     string `json:"expired_date"`     // yyyyMMddHHmmss (WIB)
			ExpiredDatetime string `json:"expired_datetime"` // RFC3339 UTC
		} `json:"payment"`
	} `json:"response"`
}

// CreateCharge memanggil DOKU Checkout (Generate Payment). DOKU mengembalikan
// payment.url SINKRON pada response yang sama (spec §2). Alur canonical
// created → pending terjadi dalam satu panggilan ini.
func (a *Adapter) CreateCharge(ctx context.Context, req domain.ChargeRequest) (res domain.ChargeResult, err error) {
	if a.obs != nil {
		start := a.now()
		defer func() { a.obs.ObserveCharge(gatewayName, err == nil, a.now().Sub(start)) }()
	}
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

	resp, err := a.doSigned(ctx, http.MethodPost, checkoutPath, body)
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
	if parsed.Response.Payment.URL == "" {
		return domain.ChargeResult{}, fmt.Errorf("doku.CreateCharge: response tanpa payment.url")
	}

	var raw map[string]any
	_ = json.Unmarshal(resp.body, &raw)

	return domain.ChargeResult{
		GatewayTxnID:     gatewayTxnID(parsed),
		GatewayRequestID: resp.requestID, // Request-Id yang kita kirim — dipakai GetStatus/Refund
		PaymentURL:       parsed.Response.Payment.URL,
		Status:           domain.StatusPending, // checkout dibuat; menunggu pembayaran
		ExpiresAt:        parseCheckoutExpiry(parsed),
		Raw:              raw,
	}, nil
}

// gatewayTxnID memilih identifier transaksi DOKU dari response.
func gatewayTxnID(p checkoutResponse) string {
	if p.Response.Payment.TokenID != "" {
		return p.Response.Payment.TokenID
	}
	return p.Response.Order.SessionID
}

// parseCheckoutExpiry mengembalikan expiry sebagai UTC. Prefer expired_datetime
// (RFC3339 UTC); fallback expired_date (yyyyMMddHHmmss, WIB/UTC+7) → ke UTC.
func parseCheckoutExpiry(p checkoutResponse) *time.Time {
	if s := p.Response.Payment.ExpiredDatetime; s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			u := t.UTC()
			return &u
		}
	}
	if s := p.Response.Payment.ExpiredDate; s != "" {
		const layout = "20060102150405"
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
func (a *Adapter) ParseWebhook(_ context.Context, raw domain.WebhookPayload) (evt domain.WebhookEvent, err error) {
	if a.obs != nil {
		defer func() { a.obs.ObserveWebhook(gatewayName, err == nil) }()
	}
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

// statusResponse adalah subset response Check Status DOKU.
type statusResponse struct {
	// Amount bisa berupa ANGKA (Check Status: 50000) ATAU string (notifikasi:
	// "50000.00") — pakai RawMessage agar tahan kedua bentuk (dinormalkan via rawAmount).
	Order struct {
		Amount json.RawMessage `json:"amount"`
	} `json:"order"`
	Transaction struct {
		Status string `json:"status"`
	} `json:"transaction"`
	Channel struct {
		ID string `json:"id"`
	} `json:"channel"`
}

// rawAmount menormalkan amount DOKU yang bisa berupa angka (50000) ATAU string
// ("50000.00") menjadi string untuk parseAmountMinor.
func rawAmount(raw json.RawMessage) string {
	return strings.Trim(string(raw), `"`)
}

// GetStatus memanggil Check Status DOKU: GET /orders/v1/status/{invoice_number}
// (doku-integration-spec §5). Catatan: DOKU menyarankan cek >=60 detik setelah
// pembayaran (status mungkin belum final lebih awal) — di-handle oleh caller.
func (a *Adapter) GetStatus(ctx context.Context, ref domain.StatusRef) (domain.StatusResult, error) {
	id := ref.ExternalReference
	if id == "" {
		id = ref.GatewayRequestID
	}
	if id == "" {
		return domain.StatusResult{}, fmt.Errorf("doku.GetStatus: butuh external_reference atau gateway_request_id")
	}

	target := "/orders/v1/status/" + id
	resp, err := a.doSigned(ctx, http.MethodGet, target, nil)
	if err != nil {
		return domain.StatusResult{}, err
	}
	if resp.statusCode < 200 || resp.statusCode >= 300 {
		return domain.StatusResult{}, parseError(resp.statusCode, resp.body)
	}

	var parsed statusResponse
	if err := json.Unmarshal(resp.body, &parsed); err != nil {
		return domain.StatusResult{}, fmt.Errorf("doku.GetStatus: parse response: %w", err)
	}

	var rawMap map[string]any
	_ = json.Unmarshal(resp.body, &rawMap)

	return domain.StatusResult{
		Status:        mapStatus(parsed.Transaction.Status),
		PaymentMethod: parsed.Channel.ID,
		AmountMinor:   parseAmountMinor(rawAmount(parsed.Order.Amount)),
		Raw:           rawMap,
	}, nil
}

type refundBody struct {
	Order struct {
		InvoiceNumber string `json:"invoice_number"`
	} `json:"order"`
	Payment struct {
		OriginalRequestID string `json:"original_request_id"`
	} `json:"payment"`
	Refund struct {
		Amount string `json:"amount"`
		Type   string `json:"type"`
		Reason string `json:"reason,omitempty"`
	} `json:"refund"`
}

type refundResponse struct {
	Refund struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"refund"`
	Transaction struct {
		Status string `json:"status"`
	} `json:"transaction"`
}

// refundEndpoint memilih endpoint refund DOKU berdasarkan channel (spec §9.1).
// VA tidak punya API refund → ditolak (keterbatasan, refund manual via Back Office).
func refundEndpoint(paymentMethod string) (string, bool) {
	pm := strings.ToUpper(paymentMethod)
	switch {
	case strings.Contains(pm, "VIRTUAL_ACCOUNT") || pm == "VA":
		return "", false
	case strings.Contains(pm, "QRIS"):
		return "/snap-adapter/b2b/v1.0/qr/qr-mpm-refund", true
	case strings.Contains(pm, "EMONEY") || strings.Contains(pm, "EWALLET") ||
		strings.Contains(pm, "WALLET") || strings.Contains(pm, "OVO") ||
		strings.Contains(pm, "DANA") || strings.Contains(pm, "SHOPEE"):
		return "/direct-debit/core/v1/debit/refund", true
	default:
		// kartu kredit & default
		return "/cancellation/credit-card/refund", true
	}
}

// Refund memilih endpoint sesuai channel (spec §9.1) & jenis (PARTIAL/FULL),
// kirim original_request_id (= gateway_request_id), lalu map hasil ke RefundResult.
func (a *Adapter) Refund(ctx context.Context, req domain.RefundRequest) (domain.RefundResult, error) {
	target, ok := refundEndpoint(req.PaymentMethod)
	if !ok {
		return domain.RefundResult{}, domain.ErrRefundNotSupported
	}

	var body refundBody
	body.Order.InvoiceNumber = req.ExternalReference
	body.Payment.OriginalRequestID = req.OriginalRequestID
	body.Refund.Amount = FormatAmount(EndpointVA, req.AmountMinor, req.Currency) // 2 desimal
	body.Refund.Type = string(req.Type)
	body.Refund.Reason = req.Reason

	raw, err := json.Marshal(body)
	if err != nil {
		return domain.RefundResult{}, fmt.Errorf("doku.Refund: marshal body: %w", err)
	}

	resp, err := a.doSigned(ctx, http.MethodPost, target, raw)
	if err != nil {
		return domain.RefundResult{}, err
	}
	if resp.statusCode < 200 || resp.statusCode >= 300 {
		return domain.RefundResult{}, parseError(resp.statusCode, resp.body)
	}

	var parsed refundResponse
	if err := json.Unmarshal(resp.body, &parsed); err != nil {
		return domain.RefundResult{}, fmt.Errorf("doku.Refund: parse response: %w", err)
	}

	var rawMap map[string]any
	_ = json.Unmarshal(resp.body, &rawMap)

	refundID := parsed.Refund.ID
	if refundID == "" {
		refundID = resp.requestID // fallback: Request-Id yang kita kirim
	}

	return domain.RefundResult{
		GatewayRefundID: refundID,
		Status:          mapRefundStatus(parsed.Refund.Status),
		Raw:             rawMap,
	}, nil
}

// mapRefundStatus memetakan status refund DOKU → canonical RefundStatus.
func mapRefundStatus(s string) domain.RefundStatus {
	switch strings.ToUpper(s) {
	case "SUCCESS", "SUCCEEDED", "REFUNDED":
		return domain.RefundSucceeded
	case "PENDING", "PROCESSING", "REQUESTED":
		return domain.RefundPending
	case "FAILED", "FAILURE", "REJECTED":
		return domain.RefundFailed
	default:
		// status tak dikenal → perlakukan pending (finalisasi via notification/poll)
		return domain.RefundPending
	}
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
