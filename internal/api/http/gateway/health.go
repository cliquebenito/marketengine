package gateway

import (
	"context"
	"net/http"
	"time"
)

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.Health.Ping(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "db unavailable", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"git_sha": h.GitSHA,
	})
}
