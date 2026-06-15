package coingecko

import (
	"context"

	"marketengine/internal/liquidity"
)

type Adapter struct{ client *Client }

func NewAdapter(c *Client) *Adapter { return &Adapter{client: c} }

var _ liquidity.MarketCapProvider = (*Adapter)(nil)

func (a *Adapter) FetchMarketCapHistory(ctx context.Context, coinID string) ([]liquidity.MarketCapPoint, error) {
	pts, err := a.client.FetchMarketCapHistory(ctx, coinID)
	if err != nil {
		return nil, err
	}
	out := make([]liquidity.MarketCapPoint, 0, len(pts))
	for _, p := range pts {
		out = append(out, liquidity.MarketCapPoint{
			Date:         p.Date,
			CoinID:       p.CoinID,
			MarketCapUSD: p.MarketCapUSD,
			PriceUSD:     p.PriceUSD,
			PayloadHash:  p.PayloadHash,
		})
	}
	return out, nil
}
