package coinglass

import (
	"context"

	"marketengine/internal/volatility"
)

type VolatilityOptionsAdapter struct{ client *Client }

func NewVolatilityOptionsAdapter(c *Client) *VolatilityOptionsAdapter {
	return &VolatilityOptionsAdapter{client: c}
}

var _ volatility.CoinglassOptionsProvider = (*VolatilityOptionsAdapter)(nil)

func (a *VolatilityOptionsAdapter) FetchOptionsInfo(ctx context.Context, symbol string) ([]volatility.CoinglassOptionsInfoPoint, error) {
	pts, err := a.client.FetchOptionsInfo(ctx, symbol)
	if err != nil {
		return nil, err
	}
	out := make([]volatility.CoinglassOptionsInfoPoint, 0, len(pts))
	for _, p := range pts {
		out = append(out, volatility.CoinglassOptionsInfoPoint{
			Exchange:         p.Exchange,
			Symbol:           p.Symbol,
			OpenInterest:     p.OpenInterest,
			OIMarketShare:    p.OIMarketShare,
			OIChange24h:      p.OIChange24h,
			OpenInterestUSD:  p.OpenInterestUSD,
			VolumeUSD24h:     p.VolumeUSD24h,
			VolumeChangePct:  p.VolumeChangePct,
			CallOpenInterest: p.CallOpenInterest,
			PutOpenInterest:  p.PutOpenInterest,
			PayloadHash:      p.PayloadHash,
		})
	}
	return out, nil
}

func (a *VolatilityOptionsAdapter) FetchOptionsOIHistory(ctx context.Context, symbol string) ([]volatility.CoinglassOptionsOIHistoryPoint, error) {
	pts, err := a.client.FetchOptionsExchangeOIHistory(ctx, symbol, "USD", "all")
	if err != nil {
		return nil, err
	}
	out := make([]volatility.CoinglassOptionsOIHistoryPoint, 0, len(pts))
	for _, p := range pts {
		out = append(out, volatility.CoinglassOptionsOIHistoryPoint{
			Date:         p.Date,
			Exchange:     p.Exchange,
			Symbol:       p.Symbol,
			OpenInterest: p.OpenInterest,
			PayloadHash:  p.PayloadHash,
		})
	}
	return out, nil
}

func (a *VolatilityOptionsAdapter) FetchOptionsMaxPain(ctx context.Context, symbol, exchange string) ([]volatility.CoinglassOptionsMaxPainPoint, error) {
	pts, err := a.client.FetchOptionsMaxPain(ctx, symbol, exchange)
	if err != nil {
		return nil, err
	}
	out := make([]volatility.CoinglassOptionsMaxPainPoint, 0, len(pts))
	for _, p := range pts {
		out = append(out, volatility.CoinglassOptionsMaxPainPoint{
			Date:              p.Date,
			Symbol:            p.Symbol,
			Exchange:          p.Exchange,
			MaxPainPrice:      p.MaxPainPrice,
			CallOIContracts:   p.CallOIContracts,
			PutOIContracts:    p.PutOIContracts,
			CallOINotionalUSD: p.CallOINotionalUSD,
			PutOINotionalUSD:  p.PutOINotionalUSD,
			CallMarketValue:   p.CallMarketValue,
			PutMarketValue:    p.PutMarketValue,
			PayloadHash:       p.PayloadHash,
		})
	}
	return out, nil
}
