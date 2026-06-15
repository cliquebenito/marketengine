package pgbacktest

import (
	"context"
	"fmt"
	"time"

	"marketengine/internal/backtest"
	"marketengine/internal/domain"
	"marketengine/internal/storage"
)

type PriceReader struct{ pool *storage.Pool }

func NewPriceReader(pool *storage.Pool) *PriceReader { return &PriceReader{pool: pool} }

var _ backtest.PriceReader = (*PriceReader)(nil)

func coinIDFor(asset domain.Asset) string {
	switch asset {
	case domain.AssetBTC:
		return "bitcoin"
	case domain.AssetETH:
		return "ethereum"
	}
	return ""
}

func (r *PriceReader) GetPriceHistory(ctx context.Context, asset domain.Asset, from, to time.Time) ([]backtest.PricePointAt, error) {
	coin := coinIDFor(asset)
	if coin == "" {
		return nil, fmt.Errorf("price reader: unsupported asset %s", asset)
	}
	rows, err := r.pool.Query(ctx, `
SELECT DISTINCT ON (value_date) value_date, price_usd
FROM regime.raw_coingecko_market_cap
WHERE coin_id = $1
  AND value_date BETWEEN $2 AND $3
  AND price_usd > 0
ORDER BY value_date ASC, ingested_at DESC`, coin, from, to)
	if err != nil {
		return nil, fmt.Errorf("query price history: %w", err)
	}
	defer rows.Close()
	var raw []backtest.PricePointAt
	for rows.Next() {
		var p backtest.PricePointAt
		if err := rows.Scan(&p.Date, &p.Price); err != nil {
			return nil, err
		}
		p.Date = domain.UTCDay(p.Date)
		raw = append(raw, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return forwardFillDaily(raw, from, to), nil
}

func forwardFillDaily(sparse []backtest.PricePointAt, from, to time.Time) []backtest.PricePointAt {
	if len(sparse) == 0 {
		return sparse
	}
	from = domain.UTCDay(from)
	to = domain.UTCDay(to)
	const maxGapDays = 7
	out := make([]backtest.PricePointAt, 0, int(to.Sub(from).Hours()/24)+1)
	idx := 0
	var lastPrice float64
	var lastDate time.Time
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		for idx < len(sparse) && !sparse[idx].Date.After(d) {
			lastPrice = sparse[idx].Price
			lastDate = sparse[idx].Date
			idx++
		}
		if lastPrice > 0 && int(d.Sub(lastDate).Hours()/24) <= maxGapDays {
			out = append(out, backtest.PricePointAt{Date: d, Price: lastPrice})
		}
	}
	return out
}
