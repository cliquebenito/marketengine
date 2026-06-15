package kraken

import (
	"context"

	"marketengine/internal/marketstress"
)

type Adapter struct{ client *Client }

func NewAdapter(c *Client) *Adapter { return &Adapter{client: c} }

var _ marketstress.KrakenProvider = (*Adapter)(nil)

func (a *Adapter) FetchOHLC(ctx context.Context, pair string, intervalMinutes int, since int64) ([]marketstress.KrakenOHLCPoint, error) {
	pts, err := a.client.FetchOHLC(ctx, pair, intervalMinutes, since)
	if err != nil {
		return nil, err
	}
	out := make([]marketstress.KrakenOHLCPoint, 0, len(pts))
	for _, p := range pts {
		out = append(out, marketstress.KrakenOHLCPoint{
			Date:        p.Date,
			Open:        p.Open,
			High:        p.High,
			Low:         p.Low,
			Close:       p.Close,
			Volume:      p.Volume,
			PayloadHash: p.PayloadHash,
		})
	}
	return out, nil
}
