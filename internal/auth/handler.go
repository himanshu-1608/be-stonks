package auth

import (
	"net/http"

	"be-stonks/internal/provider"
)

// Handler exposes the generic broker login flow.
type Handler struct {
	p provider.Provider
}

// NewHandler builds an auth HTTP handler for the active provider.
func NewHandler(p provider.Provider) *Handler { return &Handler{p: p} }

// Login handles GET /auth/login by redirecting to the broker login URL.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, h.p.LoginURL(), http.StatusFound)
}

// Callback handles GET /auth/callback?request_token=... and persists the session.
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("request_token")
	if token == "" {
		http.Error(w, "missing request_token", http.StatusBadRequest)
		return
	}
	if err := h.p.ExchangeToken(token); err != nil {
		http.Error(w, "token exchange failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("login successful — session saved"))
}
