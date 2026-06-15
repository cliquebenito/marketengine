package binance_spot

import (
	"context"
	"time"

	"marketengine/internal/marketstress"
	"marketengine/internal/providers/binance"
)

type Adapter struct{ client *binance.Client }

func NewAdapter(c *binance.Client) *Adapter { return &Adapter{client: c} }

var _ marketstress.BinanceSpotProvider = (*Adapter)(nil)

func (a *Adapter) FetchKlines(ctx context.Context, symbol, interval string, start, end time.Time) ([]marketstress.BinanceKlinePoint, error) {
	pts, err := a.client.FetchKlines(ctx, symbol, interval, start, end)
	if err != nil {
		return nil, err
	}
	out := make([]marketstress.BinanceKlinePoint, 0, len(pts))
	for _, p := range pts {
		out = append(out, marketstress.BinanceKlinePoint{
			OpenTime:    p.OpenTime,
			Close:       p.Close,
			PayloadHash: p.PayloadHash,
		})
	}
	return out, nil
}
