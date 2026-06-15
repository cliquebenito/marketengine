package coinmetrics

import (
	"context"
	"time"

	"marketengine/internal/domain"
	"marketengine/internal/liquidity"
)

type Adapter struct{ client *Client }

func NewAdapter(c *Client) *Adapter { return &Adapter{client: c} }

var _ liquidity.ExchangeNetflowProvider = (*Adapter)(nil)

func (a *Adapter) FetchExchangeNetflow(ctx context.Context, assets []domain.Asset, start, end time.Time) ([]liquidity.ExchangeNetflowPoint, error) {
	syms := make([]string, 0, len(assets))
	for _, x := range assets {
		syms = append(syms, string(x))
	}
	pts, err := a.client.FetchExchangeNetflow(ctx, syms, start, end)
	if err != nil {
		return nil, err
	}
	out := make([]liquidity.ExchangeNetflowPoint, 0, len(pts))
	for _, p := range pts {
		out = append(out, liquidity.ExchangeNetflowPoint{
			Date:        p.Date,
			Asset:       domain.Asset(p.Asset),
			InflowUSD:   p.InflowUSD,
			OutflowUSD:  p.OutflowUSD,
			NetflowUSD:  p.NetflowUSD,
			PayloadHash: p.RawPayloadHash,
		})
	}
	return out, nil
}
