package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// HealthHandler melayani liveness (/healthz) & readiness (/readyz).
type HealthHandler struct {
	// ready memeriksa dependency kritis (mis. ping DB). Nil = lewati cek.
	ready func(ctx context.Context) error
}

func NewHealthHandler(ready func(ctx context.Context) error) *HealthHandler {
	return &HealthHandler{ready: ready}
}

// Healthz: liveness — proses hidup (tidak mengecek dependency).
func (h *HealthHandler) Healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Readyz: readiness — siap melayani trafik hanya bila dependency (DB) sehat.
func (h *HealthHandler) Readyz(w http.ResponseWriter, r *http.Request) {
	if h.ready != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := h.ready(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "unavailable",
				"error":  err.Error(),
			})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
