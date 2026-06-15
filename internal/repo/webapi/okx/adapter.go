package okx

import (
	"context"
	"time"

	"marketengine/internal/domain"
	"marketengine/internal/leverage"
)

func instIDFor(asset domain.Asset) string {
	switch asset {
	case domain.AssetBTC:
		return "BTC-USDT-SWAP"
	case domain.AssetETH:
		return "ETH-USDT-SWAP"
	}
	return ""
}

func currencyFor(asset domain.Asset) string {
	switch asset {
	case domain.AssetBTC:
		return "BTC"
	case domain.AssetETH:
		return "ETH"
	}
	return ""
}

type OIAdapter struct{ client *Client }

func NewOIAdapter(c *Client) *OIAdapter { return &OIAdapter{client: c} }

var _ leverage.OIProvider = (*OIAdapter)(nil)

func (a *OIAdapter) FetchOpenInterest(ctx context.Context, asset domain.Asset, _, _ time.Time) ([]leverage.OIPoint, error) {
	ccy := currencyFor(asset)
	if ccy == "" {
		return nil, nil
	}
	pts, err := a.client.FetchOpenInterest(ctx, ccy)
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
	instID := instIDFor(asset)
	if instID == "" {
		return nil, nil
	}
	pts, err := a.client.FetchFundingRateHistory(ctx, instID, from, to)
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
