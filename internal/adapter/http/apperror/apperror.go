// Package apperror mendefinisikan tipe error HTTP yang dipakai di seluruh adapter
// layer (handler & middleware). Setiap error membawa kode mesin yang stabil
// (untuk client) dan pesan Bahasa Indonesia yang ramah user.
package apperror

import (
	"errors"
	"net/http"

	domain "github.com/Pravasta/payment-service/internal/domain/payment"
)

// AppError adalah error HTTP terstruktur. Implementasi error interface sehingga
// bisa dipakai dengan errors.As dari mana saja di delivery layer.
type AppError struct {
	Code       string // kode stabil, mis. "unauthorized", "not_found"
	Message    string // pesan user-friendly (Bahasa Indonesia)
	HTTPStatus int    // HTTP status code
}

func (e *AppError) Error() string { return e.Message }

// --- konstruktor ---

func Unauthorized(msg string) *AppError {
	return &AppError{HTTPStatus: http.StatusUnauthorized, Code: "unauthorized", Message: msg}
}

func Forbidden(msg string) *AppError {
	return &AppError{HTTPStatus: http.StatusForbidden, Code: "forbidden", Message: msg}
}

func BadRequest(msg string) *AppError {
	return &AppError{HTTPStatus: http.StatusBadRequest, Code: "invalid_request", Message: msg}
}

func NotFound(msg string) *AppError {
	return &AppError{HTTPStatus: http.StatusNotFound, Code: "not_found", Message: msg}
}

func Conflict(msg string) *AppError {
	return &AppError{HTTPStatus: http.StatusConflict, Code: "conflict", Message: msg}
}

func TooManyRequests(msg string) *AppError {
	return &AppError{HTTPStatus: http.StatusTooManyRequests, Code: "rate_limited", Message: msg}
}

func UnprocessableEntity(msg string) *AppError {
	return &AppError{HTTPStatus: http.StatusUnprocessableEntity, Code: "unprocessable", Message: msg}
}

func Internal() *AppError {
	return &AppError{
		HTTPStatus: http.StatusInternalServerError,
		Code:       "internal_error",
		Message:    "terjadi kesalahan internal, silakan coba beberapa saat lagi",
	}
}

// FromDomain memetakan domain error ke AppError HTTP yang sesuai.
// Error yang tidak dikenali diperlakukan sebagai internal error.
func FromDomain(err error) *AppError {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return NotFound("data tidak ditemukan")
	case errors.Is(err, domain.ErrIdempotencyConflict):
		return Conflict("idempotency key sudah digunakan dengan payload berbeda")
	case errors.Is(err, domain.ErrRefundExceedsAmount):
		return BadRequest("jumlah refund melebihi saldo yang tersedia")
	case errors.Is(err, domain.ErrNotRefundable):
		return BadRequest("transaksi tidak dalam status yang bisa direfund")
	case errors.Is(err, domain.ErrRefundNotSupported):
		return UnprocessableEntity("refund tidak didukung untuk channel pembayaran ini")
	case errors.Is(err, domain.ErrUnsupportedCurrency):
		return BadRequest("mata uang tidak didukung")
	case errors.Is(err, domain.ErrInvalidAmount):
		return BadRequest("amount harus lebih besar dari nol")
	case errors.Is(err, domain.ErrDuplicateReference):
		return Conflict("external_reference sudah dipakai untuk app ini")
	case errors.Is(err, domain.ErrInvalidTransition):
		return UnprocessableEntity("transisi status pembayaran tidak valid")
	case errors.Is(err, domain.ErrInvalidSignature):
		return Unauthorized("signature webhook tidak valid")
	case errors.Is(err, domain.ErrRateLimited):
		return TooManyRequests("terlalu sering; coba lagi nanti")
	default:
		return Internal()
	}
}
