package gateway

import (
	"net/http"

	"marketengine/internal/domain"
)

func (h *Handler) domainScores(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("domain")
	dom, ok := domainSlugs[slug]
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown domain", nil)
		return
	}
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

	queryAsset := asset
	if dom == domain.DomainCapitalFlows {
		queryAsset = domain.AssetGlobal
	}
	rows, err := h.Scores.GetTimeline(r.Context(), queryAsset, dom, from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", err)
		return
	}

	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, domainScoreRowToJSON(row))
	}
	writeJSON(w, http.StatusOK, out)
}
