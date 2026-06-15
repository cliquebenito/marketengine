package bybit

import (
	"context"
	"time"

	"marketengine/internal/domain"
	"marketengine/internal/leverage"
)

func symbolFor(asset domain.Asset) string {
	switch asset {
	case domain.AssetBTC:
		return "BTCUSDT"
	case domain.AssetETH:
		return "ETHUSDT"
	}
	return ""
}

type OIAdapter struct{ client *Client }

func NewOIAdapter(c *Client) *OIAdapter { return &OIAdapter{client: c} }

var _ leverage.OIProvider = (*OIAdapter)(nil)

func (a *OIAdapter) FetchOpenInterest(ctx context.Context, asset domain.Asset, from, to time.Time) ([]leverage.OIPoint, error) {
	sym := symbolFor(asset)
	if sym == "" {
		return nil, nil
	}
	pts, err := a.client.FetchOpenInterest(ctx, sym, "1d", from, to)
	if err != nil {
		return nil, err
	}
	out := make([]leverage.OIPoint, 0, len(pts))
	for _, p := range pts {
		out = append(out, leverage.OIPoint{
			Date:        p.Date,
			Asset:       asset,
			OIUSD:       p.OpenInterestUSD,
			PayloadHash: p.PayloadHash,
		})
	}
	return out, nil
}

type FundingAdapter struct{ client *Client }

func NewFundingAdapter(c *Client) *FundingAdapter { return &FundingAdapter{client: c} }

var _ leverage.FundingProvider = (*FundingAdapter)(nil)

func (a *FundingAdapter) FetchFundingRateHistory(ctx context.Context, asset domain.Asset, from, to time.Time) ([]leverage.FundingPoint, error) {
	sym := symbolFor(asset)
	if sym == "" {
		return nil, nil
	}
	pts, err := a.client.FetchFundingRateHistory(ctx, sym, from, to)
	if err != nil {
		return nil, err
	}
	out := make([]leverage.FundingPoint, 0, len(pts))
	for _, p := range pts {
		out = append(out, leverage.FundingPoint{
			Timestamp:   p.Timestamp,
			Asset:       asset,
			Rate:        p.Rate,
			PayloadHash: p.PayloadHash,
		})
	}
	return out, nil
}
