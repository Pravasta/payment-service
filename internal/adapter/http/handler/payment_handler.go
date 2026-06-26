package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Pravasta/payment-service/internal/adapter/http/apperror"
	"github.com/Pravasta/payment-service/internal/adapter/http/dto"
	appmw "github.com/Pravasta/payment-service/internal/adapter/http/middleware"
	domain "github.com/Pravasta/payment-service/internal/domain/payment"
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

// Get — GET /v1/payments/{id} (§4.2). Ter-scope per app_id (isolasi).
func (h *PaymentHandler) Get(w http.ResponseWriter, r *http.Request) {
	appID := appmw.AppIDFromContext(r.Context())
	if appID == uuid.Nil {
		writeError(w, r, apperror.Internal())
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, apperror.BadRequest("id pembayaran tidak valid"))
		return
	}

	txn, err := h.svc.GetPayment(r.Context(), appID, id)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.NewPaymentResponse(txn))
}

// GetByReference — GET /v1/payments?external_reference=... (§4.2).
func (h *PaymentHandler) GetByReference(w http.ResponseWriter, r *http.Request) {
	appID := appmw.AppIDFromContext(r.Context())
	if appID == uuid.Nil {
		writeError(w, r, apperror.Internal())
		return
	}

	ref := r.URL.Query().Get("external_reference")
	if ref == "" {
		writeError(w, r, apperror.BadRequest("query parameter external_reference wajib"))
		return
	}

	txn, err := h.svc.GetPaymentByReference(r.Context(), appID, ref)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.NewPaymentResponse(txn))
}

// List — GET /v1/transactions (§4.5). Filter status/from/to + cursor pagination,
// ter-scope per app_id.
func (h *PaymentHandler) List(w http.ResponseWriter, r *http.Request) {
	appID := appmw.AppIDFromContext(r.Context())
	if appID == uuid.Nil {
		writeError(w, r, apperror.Internal())
		return
	}

	q := r.URL.Query()
	in := usecase.ListPaymentsInput{AppID: appID, Status: q.Get("status")}

	if v := q.Get("from"); v != "" {
		t, err := parseDateParam(v)
		if err != nil {
			writeError(w, r, apperror.BadRequest("parameter from tidak valid (gunakan YYYY-MM-DD atau RFC3339)"))
			return
		}
		in.From = &t
	}
	if v := q.Get("to"); v != "" {
		t, err := parseDateParam(v)
		if err != nil {
			writeError(w, r, apperror.BadRequest("parameter to tidak valid (gunakan YYYY-MM-DD atau RFC3339)"))
			return
		}
		in.To = &t
	}
	if v := q.Get("limit"); v != "" {
		n, err := parseIntParam(v)
		if err != nil || n < 0 {
			writeError(w, r, apperror.BadRequest("parameter limit tidak valid"))
			return
		}
		in.Limit = n
	}
	if v := q.Get("cursor"); v != "" {
		created, id, err := dto.DecodeCursor(v)
		if err != nil {
			writeError(w, r, apperror.BadRequest("cursor tidak valid"))
			return
		}
		in.CursorCreated = &created
		in.CursorID = &id
	}

	res, err := h.svc.ListPayments(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}

	items := make([]dto.PaymentResponse, len(res.Items))
	for i, t := range res.Items {
		items[i] = dto.NewPaymentResponse(t)
	}
	out := dto.ListPaymentsResponse{Items: items, HasMore: res.HasMore}
	if res.HasMore && res.NextCursorCreated != nil && res.NextCursorID != nil {
		out.NextCursor = dto.EncodeCursor(*res.NextCursorCreated, *res.NextCursorID)
	}
	writeJSON(w, http.StatusOK, out)
}

// parseDateParam menerima YYYY-MM-DD atau RFC3339.
func parseDateParam(s string) (time.Time, error) {
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.UTC(), nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}

// parseIntParam mem-parse query parameter integer.
func parseIntParam(s string) (int, error) {
	return strconv.Atoi(s)
}

// Sync — POST /v1/payments/{id}/sync (§4.3): paksa refresh status dari gateway,
// rate-limited per transaksi.
func (h *PaymentHandler) Sync(w http.ResponseWriter, r *http.Request) {
	appID := appmw.AppIDFromContext(r.Context())
	if appID == uuid.Nil {
		writeError(w, r, apperror.Internal())
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, apperror.BadRequest("id pembayaran tidak valid"))
		return
	}

	txn, err := h.svc.SyncPayment(r.Context(), appID, id)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.NewPaymentResponse(txn))
}

// Refund — POST /v1/payments/{id}/refunds (§4.4). TODO(impl).
func (h *PaymentHandler) Refund(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "refund")
}

// WebhookDOKU — POST /v1/webhooks/doku (§4.6). Diverifikasi via signature DOKU
// di dalam use-case (bukan auth API key). Selalu balas cepat (§6.1 langkah 6).
func (h *PaymentHandler) WebhookDOKU(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBody))
	if err != nil {
		writeError(w, r, apperror.BadRequest("gagal membaca body webhook"))
		return
	}

	raw := domain.WebhookPayload{
		Headers: map[string]string{
			"Client-Id":         r.Header.Get("Client-Id"),
			"Request-Id":        r.Header.Get("Request-Id"),
			"Request-Timestamp": r.Header.Get("Request-Timestamp"),
			"Signature":         r.Header.Get("Signature"),
		},
		RawBody: body,
		URLPath: r.URL.Path, // Request-Target untuk verifikasi signature
	}

	if err := h.svc.HandleDOKUWebhook(r.Context(), raw); err != nil {
		// Signature invalid → 401; error lain → 5xx (DOKU retry).
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func notImplemented(w http.ResponseWriter, what string) {
	writeJSON(w, http.StatusNotImplemented, map[string]string{
		"error":   "not_implemented",
		"message": what + " belum diimplementasikan",
	})
}
