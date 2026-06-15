package coinbase

import (
	"context"
	"time"

	"marketengine/internal/marketstress"
)

type Adapter struct{ client *Client }

func NewAdapter(c *Client) *Adapter { return &Adapter{client: c} }

var _ marketstress.CoinbaseProvider = (*Adapter)(nil)

func (a *Adapter) FetchCandles(ctx context.Context, productID string, start, end time.Time, granularitySec int) ([]marketstress.CoinbaseCandlePoint, error) {
	pts, err := a.client.FetchCandles(ctx, productID, start, end, granularitySec)
	if err != nil {
		return nil, err
	}
	out := make([]marketstress.CoinbaseCandlePoint, 0, len(pts))
	for _, p := range pts {
		out = append(out, marketstress.CoinbaseCandlePoint{
			Date:        p.Date,
			ProductID:   p.ProductID,
			Close:       p.Close,
			Volume:      p.Volume,
			PayloadHash: p.PayloadHash,
		})
	}
	return out, nil
}
