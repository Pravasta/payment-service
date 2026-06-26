package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

// MetricsRecorder adalah port observability yang dibutuhkan middleware HTTP.
// Diimplementasi oleh infrastructure/metrics.Metrics (di-inject dari cmd),
// sehingga adapter tidak meng-import package infrastructure secara langsung.
type MetricsRecorder interface {
	ObserveHTTP(method, route string, status int, dur time.Duration)
}

// Metrics mencatat jumlah & latency request HTTP. Label route memakai pola chi
// (mis. "/v1/payments/{id}") agar kardinaltias rendah — bukan path mentah.
func Metrics(rec MetricsRecorder) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			// RoutePattern tersedia setelah routing selesai.
			route := chi.RouteContext(r.Context()).RoutePattern()
			rec.ObserveHTTP(r.Method, route, ww.Status(), time.Since(start))
		})
	}
}
