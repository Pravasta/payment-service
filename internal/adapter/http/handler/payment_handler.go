package handler

import (
	"net/http"

	usecase "github.com/Pravasta/payment-service/internal/usecase/payment"
)

// PaymentHandler menerjemahkan HTTP <-> use-case pembayaran.
// Endpoint masih scaffold (501) — diisi di langkah implementasi berikutnya.
type PaymentHandler struct {
	svc *usecase.Service
}

func NewPaymentHandler(svc *usecase.Service) *PaymentHandler {
	return &PaymentHandler{svc: svc}
}

// Create — POST /v1/payments (detailed-design §4.1). TODO(impl).
func (h *PaymentHandler) Create(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "create payment")
}

// Get — GET /v1/payments/{id} (§4.2). TODO(impl).
func (h *PaymentHandler) Get(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "get payment")
}

// Sync — POST /v1/payments/{id}/sync (§4.3). TODO(impl).
func (h *PaymentHandler) Sync(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "sync payment")
}

// Refund — POST /v1/payments/{id}/refunds (§4.4). TODO(impl).
func (h *PaymentHandler) Refund(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "refund")
}

// WebhookDOKU — POST /v1/webhooks/doku (§4.6). TODO(impl).
func (h *PaymentHandler) WebhookDOKU(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "doku webhook")
}

func notImplemented(w http.ResponseWriter, what string) {
	writeJSON(w, http.StatusNotImplemented, map[string]string{
		"error":   "not_implemented",
		"message": what + " belum diimplementasikan",
	})
}
