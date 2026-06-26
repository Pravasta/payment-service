package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Pravasta/payment-service/internal/adapter/http/apperror"
	"github.com/Pravasta/payment-service/internal/outbox"
)

// AdminHandler menyediakan operasi operasional (mis. replay dead-letter outbox).
type AdminHandler struct {
	outbox outbox.Store
}

func NewAdminHandler(s outbox.Store) *AdminHandler {
	return &AdminHandler{outbox: s}
}

// ReplayOutbox — POST /v1/admin/outbox/{id}/replay: kembalikan pesan dead ke
// antrean pending (detailed-design §6.2).
func (h *AdminHandler) ReplayOutbox(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, apperror.BadRequest("id outbox tidak valid"))
		return
	}
	if err := h.outbox.Replay(r.Context(), id); err != nil {
		if errors.Is(err, outbox.ErrNotReplayable) {
			writeError(w, r, apperror.NotFound("pesan dead-letter tidak ditemukan"))
			return
		}
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "requeued"})
}
