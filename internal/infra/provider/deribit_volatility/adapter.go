package deribit_volatility

import (
	"context"
	"fmt"
	"time"

	"marketengine/internal/domain"
	"marketengine/internal/providers/deribit"
	"marketengine/internal/volatility"
)

type DvolAdapter struct{ client *deribit.Client }

func NewDvolAdapter(c *deribit.Client) *DvolAdapter { return &DvolAdapter{client: c} }

var _ volatility.DvolProvider = (*DvolAdapter)(nil)

func (a *DvolAdapter) FetchDVOL(ctx context.Context, asset domain.Asset, start, end time.Time) ([]volatility.DVOLPoint, error) {
	pts, err := a.client.FetchDVOL(ctx, string(asset), start, end)
	if err != nil {
		return nil, err
	}
	out := make([]volatility.DVOLPoint, 0, len(pts))
	for _, p := range pts {
		out = append(out, volatility.DVOLPoint{
			Date:        p.Date,
			Asset:       domain.Asset(p.Currency),
			Close:       p.Close,
			PayloadHash: p.PayloadHash,
		})
	}
	return out, nil
}

type OptionsAdapter struct{ client *deribit.Client }

func NewOptionsAdapter(c *deribit.Client) *OptionsAdapter { return &OptionsAdapter{client: c} }

var _ volatility.OptionsChainProvider = (*OptionsAdapter)(nil)

func (a *OptionsAdapter) FetchOptionsSnapshot(ctx context.Context, asset domain.Asset) (volatility.OptionsSnapshot, error) {
	opts, err := a.client.FetchOptionsSummary(ctx, string(asset))
	if err != nil {
		return volatility.OptionsSnapshot{}, err
	}
	snap := volatility.OptionsSnapshot{
		Asset:      asset,
		NumOptions: len(opts),
	}
	if slope, ok := deribit.ComputeIVTermSlope(opts); ok {
		snap.TermSlope, snap.HasTermSlope = slope, true
	}
	if skew, ok := deribit.ComputeIVSkew(opts); ok {
		snap.Skew, snap.HasSkew = skew, true
	}
	return snap, nil
}

func New(baseURL string, timeout time.Duration) *deribit.Client {
	return deribit.New(baseURL, timeout)
}

type ChainAdapter struct{ client *deribit.Client }

func NewChainAdapter(c *deribit.Client) *ChainAdapter { return &ChainAdapter{client: c} }

var _ volatility.DeribitChainProvider = (*ChainAdapter)(nil)

func (a *ChainAdapter) FetchOptionsChain(ctx context.Context, asset domain.Asset) ([]volatility.DeribitOptionsChainSnapshot, error) {
	pts, err := a.client.FetchOptionsSummary(ctx, string(asset))
	if err != nil {
		return nil, err
	}
	out := make([]volatility.DeribitOptionsChainSnapshot, 0, len(pts))
	for _, p := range pts {
		expiry := time.UnixMilli(p.ExpiryTimestamp).UTC().Truncate(24 * time.Hour)
		out = append(out, volatility.DeribitOptionsChainSnapshot{
			InstrumentName:     p.InstrumentName,
			ExpiryDate:         expiry,
			StrikePrice:        p.StrikePrice,
			IsPut:              p.IsPut,
			OpenInterest:       p.OpenInterest,
			MarkIVPct:          p.MarkIV,
			UnderlyingPriceUSD: p.UnderlyingPrice,
			PayloadHash:        sha256Short(p.InstrumentName, p.MarkIV, p.OpenInterest, p.UnderlyingPrice),
		})
	}
	return out, nil
}

func sha256Short(name string, iv, oi, spot float64) string {

	return fmt.Sprintf("chain:%s:%.4f:%.4f:%.2f", name, iv, oi, spot)
}
