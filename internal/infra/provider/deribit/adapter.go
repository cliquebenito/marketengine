package deribit

import (
	"context"
	"time"

	"marketengine/internal/domain"
	"marketengine/internal/leverage"
	"marketengine/internal/providers/deribit"
)

type BasisAdapter struct{ client *deribit.Client }

func NewBasisAdapter(c *deribit.Client) *BasisAdapter { return &BasisAdapter{client: c} }

var _ leverage.BasisProvider = (*BasisAdapter)(nil)

func (a *BasisAdapter) FetchBasis3mSnapshot(ctx context.Context, asset domain.Asset) (leverage.BasisSnapshotPoint, error) {
	snap, err := a.client.Basis3mSnapshot(ctx, string(asset))
	if err != nil {
		return leverage.BasisSnapshotPoint{}, err
	}
	return leverage.BasisSnapshotPoint{
		Date:            snap.Date,
		Asset:           asset,
		InstrumentName:  snap.InstrumentName,
		FuturesPrice:    snap.FuturesPrice,
		SpotPrice:       snap.SpotPrice,
		AnnualizedBasis: snap.AnnualizedBasisPct,
		DaysToExpiry:    snap.DaysToExpiry,
		PayloadHash:     snap.PayloadHash,
	}, nil
}

func New(baseURL string, timeout time.Duration) *deribit.Client {
	return deribit.New(baseURL, timeout)
}
