package gateway

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"marketengine/internal/domain"
)

var validAssets = map[domain.Asset]bool{
	domain.AssetBTC: true,
	domain.AssetETH: true,
}

var domainSlugs = map[string]domain.DomainCode{
	"liquidity":     domain.DomainLiquidity,
	"leverage":      domain.DomainLeverage,
	"market-stress": domain.DomainMarketStress,
	"capital-flows": domain.DomainCapitalFlows,
	"volatility":    domain.DomainVolatility,
}

func parseAsset(r *http.Request) (domain.Asset, error) {
	raw := strings.ToUpper(r.URL.Query().Get("asset"))
	if raw == "" {
		return "", fmt.Errorf("asset required")
	}
	a := domain.Asset(raw)
	if !validAssets[a] {
		return "", fmt.Errorf("invalid asset: %s", raw)
	}
	return a, nil
}

func parseFromTo(r *http.Request) (time.Time, time.Time, error) {
	fs := r.URL.Query().Get("from")
	ts := r.URL.Query().Get("to")
	if fs == "" || ts == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("from and to required (YYYY-MM-DD)")
	}
	from, err := time.Parse("2006-01-02", fs)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid from")
	}
	to, err := time.Parse("2006-01-02", ts)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid to")
	}
	if from.After(to) {
		return time.Time{}, time.Time{}, fmt.Errorf("from after to")
	}
	return from.UTC(), to.UTC(), nil
}

func parseDate(s string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date (expect YYYY-MM-DD)")
	}
	return t.UTC(), nil
}
