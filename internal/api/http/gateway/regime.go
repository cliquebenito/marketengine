package gateway

import (
	"errors"
	"net/http"

	"marketengine/internal/domain"
)

func (h *Handler) regimeLatest(w http.ResponseWriter, r *http.Request) {
	asset, err := parseAsset(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	s, err := h.Regime.GetLatest(r.Context(), asset)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "no regime state for asset", nil)
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error", err)
		return
	}
	writeJSON(w, http.StatusOK, regimeStateToJSON(s))
}

func (h *Handler) regimeHistory(w http.ResponseWriter, r *http.Request) {
	asset, err := parseAsset(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	from, to, err := parseFromTo(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	rows, err := h.Regime.GetHistory(r.Context(), asset, from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", err)
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for i := range rows {
		out = append(out, regimeStateToJSON(&rows[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) regimeContributions(w http.ResponseWriter, r *http.Request) {
	asset, err := parseAsset(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	date, err := parseDate(r.PathValue("date"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	s, err := h.Regime.GetByDate(r.Context(), asset, date)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "no regime state for asset+date", nil)
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error", err)
		return
	}

	writeJSON(w, http.StatusOK, regimeStateToJSON(s))
}
