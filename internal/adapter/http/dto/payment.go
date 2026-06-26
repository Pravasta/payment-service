// Package dto berisi struct request/response HTTP (representasi wire) yang
// TERPISAH dari entity domain — perubahan kontrak API tidak bocor ke domain.
package dto

import (
	"time"

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
