package alerts

import (
	"encoding/json"
	"net/http"
)

// Handler exposes the alerts feature over HTTP.
type Handler struct {
	syncer *Syncer
}

// NewHandler builds an alerts HTTP handler.
func NewHandler(s *Syncer) *Handler { return &Handler{syncer: s} }

// Sync handles POST /alerts/sync.
func (h *Handler) Sync(w http.ResponseWriter, r *http.Request) {
	rep, status := h.syncer.Sync(r.Context())
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(rep)
}
