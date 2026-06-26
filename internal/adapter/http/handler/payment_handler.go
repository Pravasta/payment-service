package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/google/uuid"

	"github.com/Pravasta/payment-service/internal/adapter/http/apperror"
	"github.com/Pravasta/payment-service/internal/adapter/http/dto"
	appmw "github.com/Pravasta/payment-service/internal/adapter/http/middleware"
	usecase "github.com/Pravasta/payment-service/internal/usecase/payment"
)

// maxRequestBody membatasi ukuran body request (anti memory abuse).
const maxRequestBody = 1 << 20 // 1 MiB

// PaymentHandler menerjemahkan HTTP <-> use-case pembayaran.
// Endpoint masih scaffold (501) — diisi di langkah implementasi berikutnya.
type PaymentHandler struct {
	svc *usecase.Service
}

func NewPaymentHandler(svc *usecase.Service) *PaymentHandler {
	return &PaymentHandler{svc: svc}
}

// Create — POST /v1/payments (detailed-design §4.1).
func (h *PaymentHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreatePaymentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, r, err)
		return
	}

	appID := appmw.AppIDFromContext(r.Context())
	if appID == uuid.Nil {
		writeError(w, r, apperror.Internal())
		return
	}

	txn, err := h.svc.CreatePayment(r.Context(), usecase.CreatePaymentInput{
		AppID:             appID,
		IdempotencyKey:    r.Header.Get("Idempotency-Key"),
		ExternalReference: req.ExternalReference,
		AmountMinor:       req.Amount,
		Currency:          req.Currency,
		CustomerRef:       req.CustomerRef,
		Description:       req.Description,
		ExpiryMinutes:     req.ExpiryMinutes,
		ReturnURL:         req.ReturnURL,
		Metadata:          req.Metadata,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, dto.NewPaymentResponse(txn))
}

// decodeJSON membaca & memvalidasi body JSON request. Body kosong / JSON tak valid
// dikembalikan sebagai apperror.BadRequest agar pesan ramah ke user.
func decodeJSON(r *http.Request, dst any) error {
	if r.Body == nil {
		return apperror.BadRequest("request body kosong")
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, maxRequestBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return apperror.BadRequest("request body kosong")
		}
		return apperror.BadRequest("body JSON tidak valid: " + err.Error())
	}
	return nil
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
