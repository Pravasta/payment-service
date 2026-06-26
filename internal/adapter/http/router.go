// Package http adalah delivery layer (interface adapter) — menerjemahkan HTTP
// ke pemanggilan use-case. Pakai chi sebagai router.
package http

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/Pravasta/payment-service/internal/adapter/http/handler"
	appmw "github.com/Pravasta/payment-service/internal/adapter/http/middleware"
	usecase "github.com/Pravasta/payment-service/internal/usecase/payment"
)

// NewRouter merakit seluruh route HTTP (detailed-design §4).
//
//   - ready: memeriksa kesiapan dependency (mis. ping DB) untuk /readyz.
//   - credRepo: dipakai oleh auth middleware untuk lookup API key.
//   - masterKey: kunci AES-GCM untuk dekripsi HMAC signing secret (boleh nil di dev).
//   - idemStore: penyimpanan idempotency untuk endpoint tulis (create/refund).
func NewRouter(
	paymentSvc *usecase.Service,
	ready func(ctx context.Context) error,
	credRepo appmw.AuthRepository,
	masterKey []byte,
	idemStore appmw.IdempotencyStore,
) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.Recoverer)
	r.Use(appmw.StripTrailingSlash)
	r.Use(appmw.RequestID)

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

		// Webhook DOKU: terima dari gateway — tidak perlu auth API key,
		// diverifikasi via HMAC signature DOKU di dalam handler.
	})

	// Webhook endpoint di luar grup /v1 yang di-auth — diverifikasi oleh handler.
	r.Post("/v1/webhooks/doku", payments.WebhookDOKU)

	return r
}
