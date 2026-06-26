package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/Pravasta/payment-service/internal/adapter/http/apperror"
)

// IdempotencyStore adalah port untuk menyimpan & mengambil hasil request tulis.
// Diimplementasikan oleh repository.IdempotencyRepository (interface milik consumer).
type IdempotencyStore interface {
	// Begin mencoba membuat record in-progress untuk (appID, key).
	// created=true bila record baru berhasil dibuat (request pertama — pemanggil
	// "memiliki" eksekusi). created=false bila key sudah ada; rec berisi record lama.
	Begin(ctx context.Context, appID uuid.UUID, key, requestHash string) (rec *IdempotencyRecord, created bool, err error)
	// Complete menyimpan response final (status + body) ke record.
	Complete(ctx context.Context, id uuid.UUID, statusCode int, responseBody []byte) error
	// Discard menghapus record in-progress agar request bisa di-retry (mis. handler 5xx).
	Discard(ctx context.Context, id uuid.UUID) error
}

// IdempotencyRecord adalah proyeksi minimal yang dibutuhkan middleware.
type IdempotencyRecord struct {
	ID           uuid.UUID
	RequestHash  string
	StatusCode   int // 0 = masih diproses
	ResponseBody []byte
}

// maxIdempotentBody adalah batas body yang dibaca untuk hashing (anti memory abuse).
const maxIdempotentBody = 1 << 20 // 1 MiB

// Idempotency mewajibkan header Idempotency-Key pada endpoint tulis dan menjamin
// eksekusi tepat-sekali (detailed-design §5):
//
//   - Tanpa header                       → 400
//   - Key sama, payload berbeda          → 409
//   - Key sama, request pertama berjalan → 409 (sedang diproses)
//   - Key sama, sudah selesai            → replay response pertama (header Idempotent-Replay: true)
//
// Harus dipasang SETELAH Authenticate (butuh app_id dari context).
func Idempotency(store IdempotencyStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
			if key == "" {
				respondAuthError(w, r, apperror.BadRequest(
					"header Idempotency-Key wajib untuk request ini",
				))
				return
			}

			appID := AppIDFromContext(r.Context())
			if appID == uuid.Nil {
				// Konfigurasi salah: Idempotency dipasang tanpa Authenticate di depannya.
				respondAuthError(w, r, apperror.Internal())
				return
			}

			// Baca body untuk hashing, lalu restore agar handler hilir bisa membacanya.
			var body []byte
			if r.Body != nil {
				var err error
				body, err = io.ReadAll(io.LimitReader(r.Body, maxIdempotentBody))
				if err != nil {
					respondAuthError(w, r, apperror.Internal())
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(body))
			}
			hash := requestHash(r.Method, r.URL.RequestURI(), body)

			rec, created, err := store.Begin(r.Context(), appID, key, hash)
			if err != nil {
				respondAuthError(w, r, apperror.Internal())
				return
			}

			if !created {
				handleExisting(w, r, rec, hash)
				return
			}

			// Request pertama — jalankan handler dengan buffering recorder.
			rec.StatusCode = 0
			rw := &bufferingWriter{header: make(http.Header)}
			next.ServeHTTP(rw, r)

			status := rw.status
			if status == 0 {
				status = http.StatusOK
			}

			// Status 5xx tidak di-cache: hapus record agar client boleh retry.
			if status >= http.StatusInternalServerError {
				_ = store.Discard(r.Context(), rec.ID)
			} else if err := store.Complete(r.Context(), rec.ID, status, rw.body.Bytes()); err != nil {
				respondAuthError(w, r, apperror.Internal())
				return
			}

			rw.flushTo(w)
		})
	}
}

// handleExisting menangani request dengan key yang sudah ada.
func handleExisting(w http.ResponseWriter, r *http.Request, rec *IdempotencyRecord, hash string) {
	if rec.RequestHash != hash {
		respondAuthError(w, r, apperror.Conflict(
			"Idempotency-Key sudah dipakai dengan payload berbeda",
		))
		return
	}
	if rec.StatusCode == 0 {
		respondAuthError(w, r, apperror.Conflict(
			"request dengan Idempotency-Key ini masih diproses, coba lagi sebentar",
		))
		return
	}
	// Selesai — replay response pertama.
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Idempotent-Replay", "true")
	w.WriteHeader(rec.StatusCode)
	_, _ = w.Write(rec.ResponseBody)
}

// requestHash menghitung sidik jari request untuk deteksi payload conflict.
func requestHash(method, uri string, body []byte) string {
	h := sha256.New()
	h.Write([]byte(method))
	h.Write([]byte{'\n'})
	h.Write([]byte(uri))
	h.Write([]byte{'\n'})
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

// bufferingWriter menahan response handler agar bisa di-cache dulu sebelum dikirim.
type bufferingWriter struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (b *bufferingWriter) Header() http.Header { return b.header }

func (b *bufferingWriter) WriteHeader(status int) {
	if b.status == 0 {
		b.status = status
	}
}

func (b *bufferingWriter) Write(p []byte) (int, error) {
	if b.status == 0 {
		b.status = http.StatusOK
	}
	return b.body.Write(p)
}

// flushTo menyalin status, header, dan body yang ditahan ke writer asli.
func (b *bufferingWriter) flushTo(w http.ResponseWriter) {
	for k, vv := range b.header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	status := b.status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = w.Write(b.body.Bytes())
}
