package gateway

import (
	"net/http"
)

type Handler struct {
	Regime   RegimeReader
	Scores   DomainScoreReader
	Backtest BacktestReader
	Health   HealthChecker
	GitSHA   string
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("GET /regime/latest", h.regimeLatest)
	mux.HandleFunc("GET /regime/history", h.regimeHistory)
	mux.HandleFunc("GET /regime/{date}/contributions", h.regimeContributions)
	mux.HandleFunc("GET /domains/{domain}/scores", h.domainScores)
	mux.HandleFunc("GET /backtest/runs", h.backtestRuns)
	mux.HandleFunc("GET /backtest/{run_id}/timeline", h.backtestTimeline)
	mux.HandleFunc("GET /backtest/{run_id}/events", h.backtestEvents)
	return logMiddleware(corsMiddleware(mux))
}
