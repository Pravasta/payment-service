package middleware

import (
	"log/slog"
	"net/http"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
)

// AccessLog menulis satu log terstruktur per request HTTP, selalu menyertakan
// correlation id (request_id) agar satu request bisa ditelusuri end-to-end
// (detailed-design §12). Status >= 500 dilog sebagai Error, sisanya Info.
func AccessLog(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			attrs := []any{
				"request_id", RequestIDFromContext(r.Context()),
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration_ms", time.Since(start).Milliseconds(),
			}
			if ww.Status() >= 500 {
				log.Error("http request", attrs...)
			} else {
				log.Info("http request", attrs...)
			}
		})
	}
}
