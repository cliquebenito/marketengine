package gateway

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Warn("encode json", "err", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string, cause error) {
	if cause != nil {
		slog.Error("handler error", "status", status, "msg", msg, "err", cause)
	}
	writeJSON(w, status, map[string]any{"error": msg})
}
