package coinglass

import (
	"context"
	"time"

	"marketengine/internal/marketstress"
)

type MarketStressPremiumAdapter struct{ client *Client }

func NewMarketStressPremiumAdapter(c *Client) *MarketStressPremiumAdapter {
	return &MarketStressPremiumAdapter{client: c}
}

var _ marketstress.CoinglassProvider = (*MarketStressPremiumAdapter)(nil)

func (a *MarketStressPremiumAdapter) FetchCoinbasePremiumHistory(ctx context.Context, limit int) ([]marketstress.CoinbasePremiumPoint, error) {
	pts, err := a.client.FetchCoinbasePremiumHistory(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]marketstress.CoinbasePremiumPoint, 0, len(pts))
	for _, p := range pts {
		out = append(out, marketstress.CoinbasePremiumPoint{
			Date:          p.Date,
			PremiumUSD:    p.PremiumUSD,
			PremiumRate:   p.PremiumRate,
			CoinbasePrice: p.CoinbasePrice,
			PayloadHash:   p.PayloadHash,
		})
	}
	return out, nil
}

type MarketStressMicroAdapter struct{ client *Client }

func NewMarketStressMicroAdapter(c *Client) *MarketStressMicroAdapter {
	return &MarketStressMicroAdapter{client: c}
}

var _ marketstress.CoinglassMicroProvider = (*MarketStressMicroAdapter)(nil)

func (a *MarketStressMicroAdapter) FetchOrderbookBidAsk(ctx context.Context, coinSymbol string, limit int, start, end time.Time) ([]marketstress.CoinglassOrderbookPoint, error) {
	pts, err := a.client.FetchAggregatedOrderbookBidAsk(ctx, coinSymbol, "", "1", limit, start, end)
	if err != nil {
		return nil, err
	}
	out := make([]marketstress.CoinglassOrderbookPoint, 0, len(pts))
	for _, p := range pts {
		out = append(out, marketstress.CoinglassOrderbookPoint{
			Date:        p.Date,
			Symbol:      p.Symbol,
			BidsUSD:     p.BidsUSD,
			BidsQty:     p.BidsQty,
			AsksUSD:     p.AsksUSD,
			AsksQty:     p.AsksQty,
			PayloadHash: p.PayloadHash,
		})
	}
	return out, nil
}

func (a *MarketStressMicroAdapter) FetchFuturesSpotVolRatio(ctx context.Context, coinSymbol string, limit int, start, end time.Time) ([]marketstress.CoinglassFuturesSpotVolRatioPoint, error) {
	pts, err := a.client.FetchFuturesSpotVolumeRatio(ctx, coinSymbol, "", limit, start, end)
	if err != nil {
		return nil, err
	}
	out := make([]marketstress.CoinglassFuturesSpotVolRatioPoint, 0, len(pts))
	for _, p := range pts {
		out = append(out, marketstress.CoinglassFuturesSpotVolRatioPoint{
			Date:             p.Date,
			Symbol:           p.Symbol,
			FuturesSpotRatio: p.FuturesSpotRatio,
			FuturesVolUSD:    p.FuturesVolUSD,
			SpotVolUSD:       p.SpotVolUSD,
			PayloadHash:      p.PayloadHash,
		})
	}
	return out, nil
}
