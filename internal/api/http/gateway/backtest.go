package gateway

import (
	"net/http"
	"strconv"
)

func (h *Handler) backtestRuns(w http.ResponseWriter, r *http.Request) {
	if h.Backtest == nil {
		writeError(w, http.StatusServiceUnavailable, "backtest reader not configured", nil)
		return
	}
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	runs, err := h.Backtest.ListRuns(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	out := make([]map[string]any, 0, len(runs))
	for _, run := range runs {
		row := map[string]any{
			"run_id":         run.RunID,
			"mode":           run.Mode,
			"period_start":   run.PeriodStart.Format("2006-01-02"),
			"period_end":     run.PeriodEnd.Format("2006-01-02"),
			"model_version":  run.ModelVersion,
			"config_version": run.ConfigVersion,
			"status":         run.Status,
			"started_at":     run.StartedAt.UTC().Format("2006-01-02T15:04:05Z"),
		}
		if run.CompletedAt != nil {
			row["completed_at"] = run.CompletedAt.UTC().Format("2006-01-02T15:04:05Z")
		}
		if run.ParentRunID != nil {
			row["parent_run_id"] = *run.ParentRunID
		}
		out = append(out, row)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) backtestTimeline(w http.ResponseWriter, r *http.Request) {
	if h.Backtest == nil {
		writeError(w, http.StatusServiceUnavailable, "backtest reader not configured", nil)
		return
	}
	runID := r.PathValue("run_id")
	if runID == "" {
		writeError(w, http.StatusBadRequest, "run_id required", nil)
		return
	}
	asset, err := parseAsset(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	pts, err := h.Backtest.GetTimeline(r.Context(), runID, asset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	out := make([]map[string]any, 0, len(pts))
	for _, p := range pts {
		row := map[string]any{
			"value_date":       p.ValueDate.Format("2006-01-02"),
			"regime_indicator": p.RegimeIndicator,
			"risk_on_prob":     p.RiskOnProb,
			"risk_off_prob":    p.RiskOffProb,
			"transition_risk":  p.TransitionRisk,
		}
		if p.PriceOK {
			row["price"] = p.Price
		}
		out = append(out, row)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"run_id": runID,
		"asset":  string(asset),
		"points": out,
	})
}

func (h *Handler) backtestEvents(w http.ResponseWriter, r *http.Request) {
	if h.Backtest == nil {
		writeError(w, http.StatusServiceUnavailable, "backtest reader not configured", nil)
		return
	}
	runID := r.PathValue("run_id")
	if runID == "" {
		writeError(w, http.StatusBadRequest, "run_id required", nil)
		return
	}
	asset, err := parseAsset(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	events, err := h.Backtest.GetEvents(r.Context(), runID, asset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	out := make([]map[string]any, 0, len(events))
	for _, e := range events {
		out = append(out, map[string]any{
			"name":                  e.Name,
			"peak_date":             e.PeakDate.Format("2006-01-02"),
			"first_risk_off_offset": e.FirstRiskOffOffset,
			"first_trans_offset":    e.FirstTransOffset,
			"data_present":          e.DataPresent,
		})
	}
	writeJSON(w, http.StatusOK, out)
}
