package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/Pravasta/payment-service/internal/adapter/http/apperror"
	appmw "github.com/Pravasta/payment-service/internal/adapter/http/middleware"
)

// errorResponse adalah format JSON standar untuk semua error di API ini.
// Field request_id membantu debugging lintas-service.
type errorResponse struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

// writeJSON menulis response JSON dengan Content-Type dan status code yang tepat.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError memetakan error ke HTTP response JSON.
// Jika err adalah *apperror.AppError, kode dan pesan-nya dipakai langsung.
// Error lain (mis. dari use-case atau DB) diperlakukan sebagai internal error
// agar detail implementasi tidak bocor ke client.
func writeError(w http.ResponseWriter, r *http.Request, err error) {
	var ae *apperror.AppError
	if !errors.As(err, &ae) {
		ae = apperror.FromDomain(err)
	}
	// Error 5xx = tak terduga (mis. gateway gagal). Pesan asli SENGAJA tidak
	// dibocorkan ke client, jadi log di sini — dengan request_id — agar penyebab
	// sebenarnya tetap bisa ditelusuri di server. Error 4xx tidak dilog (wajar).
	if ae.HTTPStatus >= http.StatusInternalServerError {
		slog.Default().Error("internal server error",
			"request_id", appmw.RequestIDFromContext(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
			"code", ae.Code,
			"err", err,
		)
	}
	writeJSON(w, ae.HTTPStatus, errorResponse{
		Code:      ae.Code,
		Message:   ae.Message,
		RequestID: appmw.RequestIDFromContext(r.Context()),
	})
}
