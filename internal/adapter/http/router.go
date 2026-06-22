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
// ready memeriksa kesiapan dependency (mis. ping DB) untuk /readyz.
func NewRouter(paymentSvc *usecase.Service, ready func(ctx context.Context) error) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.Recoverer)
	r.Use(appmw.StripTrailingSlash) // /readyz/ -> /readyz (hindari 404 trailing slash)
	r.Use(appmw.RequestID)

	health := handler.NewHealthHandler(ready)
	r.Get("/healthz", health.Healthz)
	r.Get("/readyz", health.Readyz)

	payments := handler.NewPaymentHandler(paymentSvc)
	r.Route("/v1", func(r chi.Router) {
		// TODO: pasang middleware auth (API key/HMAC) + idempotency di sini.
		r.Post("/payments", payments.Create)
		r.Get("/payments/{id}", payments.Get)
		r.Post("/payments/{id}/sync", payments.Sync)
		r.Post("/payments/{id}/refunds", payments.Refund)

		// Webhook receiver DOKU (diverifikasi via signature).
		r.Post("/webhooks/doku", payments.WebhookDOKU)
	})

	return r
}
