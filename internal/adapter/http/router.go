// Package http adalah delivery layer (interface adapter) — menerjemahkan HTTP
// ke pemanggilan use-case. Pakai chi sebagai router.
package http

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/Pravasta/payment-service/internal/adapter/http/handler"
	appmw "github.com/Pravasta/payment-service/internal/adapter/http/middleware"
	"github.com/Pravasta/payment-service/internal/infrastructure/metrics"
	"github.com/Pravasta/payment-service/internal/outbox"
	usecase "github.com/Pravasta/payment-service/internal/usecase/payment"
)

// NewRouter merakit seluruh route HTTP (detailed-design §4).
//
//   - ready: memeriksa kesiapan dependency (mis. ping DB) untuk /readyz.
//   - credRepo: dipakai oleh auth middleware untuk lookup API key.
//   - masterKey: kunci AES-GCM untuk dekripsi HMAC signing secret (boleh nil di dev).
//   - idemStore: penyimpanan idempotency untuk endpoint tulis (create/refund).
//   - outboxStore: untuk endpoint admin replay dead-letter.
//   - log: logger untuk access log terstruktur (correlation id end-to-end).
//   - m: koleksi metrik Prometheus; mengekspos /metrics + middleware instrumentasi.
func NewRouter(
	paymentSvc *usecase.Service,
	ready func(ctx context.Context) error,
	credRepo appmw.AuthRepository,
	masterKey []byte,
	idemStore appmw.IdempotencyStore,
	outboxStore outbox.Store,
	log *slog.Logger,
	m *metrics.Metrics,
) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.Recoverer)
	r.Use(appmw.StripTrailingSlash)
	r.Use(appmw.RequestID)      // correlation id sebelum logging/metrik
	r.Use(appmw.AccessLog(log)) // log terstruktur per request (request_id)
	r.Use(appmw.Metrics(m))     // http_requests_total + durasi

	// /metrics: endpoint scrape Prometheus (tanpa auth; batasi via jaringan).
	r.Handle("/metrics", m.Handler())

	h := handler.NewHealthHandler(ready)
	r.Get("/healthz", h.Healthz)
	r.Get("/readyz", h.Readyz)

	// idempotent membungkus endpoint tulis: wajib Idempotency-Key + replay aman.
	idempotent := appmw.Idempotency(idemStore)

	payments := handler.NewPaymentHandler(paymentSvc)
	r.Route("/v1", func(r chi.Router) {
		r.Use(appmw.Authenticate(credRepo, masterKey))

		// Pembayaran: butuh scope payments:write untuk mutasi, payments:read untuk baca.
		// Endpoint tulis (create/refund) memerlukan Idempotency-Key (detailed-design §5).
		r.With(appmw.RequireScope("payments:write"), idempotent).Post("/payments", payments.Create)
		r.With(appmw.RequireScope("payments:read")).Get("/payments/{id}", payments.Get)
		r.With(appmw.RequireScope("payments:read")).Get("/payments", payments.GetByReference)
		r.With(appmw.RequireScope("payments:read")).Get("/transactions", payments.List)
		r.With(appmw.RequireScope("payments:write")).Post("/payments/{id}/sync", payments.Sync)
		r.With(appmw.RequireScope("payments:write"), idempotent).Post("/payments/{id}/refunds", payments.Refund)

		// Operasional: replay dead-letter outbox (butuh scope outbox:admin).
		admin := handler.NewAdminHandler(outboxStore)
		r.With(appmw.RequireScope("outbox:admin")).Post("/admin/outbox/{id}/replay", admin.ReplayOutbox)

		// Webhook DOKU: terima dari gateway — tidak perlu auth API key,
		// diverifikasi via HMAC signature DOKU di dalam handler.
	})

	// Webhook endpoint di luar grup /v1 yang di-auth — diverifikasi oleh handler.
	r.Post("/v1/webhooks/doku", payments.WebhookDOKU)

	return r
}
