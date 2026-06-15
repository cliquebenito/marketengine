package defillama

import (
	"context"

	"marketengine/internal/liquidity"
)

type StablecoinAdapter struct{ client *Client }

func NewStablecoinAdapter(c *Client) *StablecoinAdapter {
	return &StablecoinAdapter{client: c}
}

var _ liquidity.StablecoinProvider = (*StablecoinAdapter)(nil)

func (a *StablecoinAdapter) FetchAllStablecoinsChart(ctx context.Context) ([]liquidity.StablecoinSupplyPoint, error) {
	pts, err := a.client.FetchAllStablecoinsChart(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]liquidity.StablecoinSupplyPoint, 0, len(pts))
	for _, p := range pts {
		out = append(out, liquidity.StablecoinSupplyPoint{
			Date:           p.Date,
			Symbol:         "AGGREGATE",
			CirculatingUSD: p.TotalCirculatingUSD,
			PayloadHash:    p.RawPayloadHash,
		})
	}
	return out, nil
}

func (a *StablecoinAdapter) FetchPerStablecoinSupply(ctx context.Context) ([]liquidity.StablecoinSupplyPoint, error) {
	pts, err := a.client.FetchPerStablecoinSupply(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]liquidity.StablecoinSupplyPoint, 0, len(pts))
	for _, p := range pts {
		out = append(out, liquidity.StablecoinSupplyPoint{
			Date:           p.Date,
			Symbol:         p.Symbol,
			CirculatingUSD: p.CirculatingUSD,
			PayloadHash:    p.RawPayloadHash,
		})
	}
	return out, nil
}

type ChainTVLAdapter struct{ client *Client }

func NewChainTVLAdapter(c *Client) *ChainTVLAdapter {
	return &ChainTVLAdapter{client: c}
}

var _ liquidity.ChainTVLProvider = (*ChainTVLAdapter)(nil)

func (a *ChainTVLAdapter) FetchChainTVL(ctx context.Context, chain string) ([]liquidity.ChainTVLPoint, error) {
	pts, err := a.client.FetchChainTVL(ctx, chain)
	if err != nil {
		return nil, err
	}
	out := make([]liquidity.ChainTVLPoint, 0, len(pts))
	for _, p := range pts {
		out = append(out, liquidity.ChainTVLPoint{
			Date:        p.Date,
			TVLUSD:      p.TVLUSD,
			PayloadHash: p.RawPayloadHash,
		})
	}
	return out, nil
}
