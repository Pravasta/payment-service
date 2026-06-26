// Package dto berisi struct request/response HTTP (representasi wire) yang
// TERPISAH dari entity domain — perubahan kontrak API tidak bocor ke domain.
package dto

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	domain "github.com/Pravasta/payment-service/internal/domain/payment"
)

// CreatePaymentRequest adalah body POST /v1/payments (detailed-design §4.1).
type CreatePaymentRequest struct {
	ExternalReference string         `json:"external_reference"`
	Amount            int64          `json:"amount"` // minor unit; IDR → rupiah
	Currency          string         `json:"currency"`
	CustomerRef       string         `json:"customer_ref"`
	Description       string         `json:"description"`
	ExpiryMinutes     int            `json:"expiry_minutes"`
	ReturnURL         string         `json:"return_url"`
	Metadata          map[string]any `json:"metadata"`
}

// PaymentResponse adalah representasi transaksi pada response API.
type PaymentResponse struct {
	ID                string         `json:"id"`
	Status            string         `json:"status"`
	ExternalReference string         `json:"external_reference"`
	Amount            int64          `json:"amount"`
	Currency          string         `json:"currency"`
	RefundedAmount    int64          `json:"refunded_amount"`
	PaymentURL        string         `json:"payment_url,omitempty"`
	PaymentMethod     string         `json:"payment_method,omitempty"`
	Description       string         `json:"description,omitempty"`
	CustomerRef       string         `json:"customer_ref,omitempty"`
	Metadata          map[string]any `json:"metadata,omitempty"`
	ExpiresAt         *time.Time     `json:"expires_at,omitempty"`
	PaidAt            *time.Time     `json:"paid_at,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
}

// ListPaymentsResponse adalah response list + pagination (detailed-design §4.5).
type ListPaymentsResponse struct {
	Items      []PaymentResponse `json:"items"`
	HasMore    bool              `json:"has_more"`
	NextCursor string            `json:"next_cursor,omitempty"`
}

// EncodeCursor membuat cursor opaque dari (created_at, id) baris terakhir.
func EncodeCursor(createdAt time.Time, id uuid.UUID) string {
	raw := fmt.Sprintf("%d:%s", createdAt.UTC().UnixNano(), id.String())
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// DecodeCursor mem-parse cursor opaque kembali ke (created_at, id).
func DecodeCursor(s string) (time.Time, uuid.UUID, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("cursor tidak valid")
	}
	parts := strings.SplitN(string(b), ":", 2)
	if len(parts) != 2 {
		return time.Time{}, uuid.Nil, fmt.Errorf("cursor tidak valid")
	}
	ns, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("cursor tidak valid")
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("cursor tidak valid")
	}
	return time.Unix(0, ns).UTC(), id, nil
}

// CreateRefundRequest adalah body POST /v1/payments/{id}/refunds (§4.4).
type CreateRefundRequest struct {
	Amount int64  `json:"amount"` // 0 / diabaikan → full refund
	Reason string `json:"reason"`
}

// RefundResponse adalah representasi refund pada response API.
type RefundResponse struct {
	ID            string `json:"id"`
	TransactionID string `json:"transaction_id"`
	Amount        int64  `json:"amount"`
	Currency      string `json:"currency"`
	Status        string `json:"status"`
	Reason        string `json:"reason,omitempty"`
}

// NewRefundResponse memetakan entity refund → response wire.
func NewRefundResponse(r *domain.Refund) RefundResponse {
	return RefundResponse{
		ID:            r.ID.String(),
		TransactionID: r.TransactionID.String(),
		Amount:        r.AmountMinor,
		Currency:      r.Currency,
		Status:        string(r.Status),
		Reason:        r.Reason,
	}
}

// NewPaymentResponse memetakan entity domain → response wire.
func NewPaymentResponse(t *domain.Transaction) PaymentResponse {
	return PaymentResponse{
		ID:                t.ID.String(),
		Status:            string(t.Status),
		ExternalReference: t.ExternalReference,
		Amount:            t.GrossAmount,
		Currency:          t.Currency,
		RefundedAmount:    t.RefundedAmount,
		PaymentURL:        t.PaymentURL,
		PaymentMethod:     t.PaymentMethod,
		Description:       t.Description,
		CustomerRef:       t.CustomerRef,
		Metadata:          t.Metadata,
		ExpiresAt:         t.ExpiresAt,
		PaidAt:            t.PaidAt,
		CreatedAt:         t.CreatedAt,
	}
}
