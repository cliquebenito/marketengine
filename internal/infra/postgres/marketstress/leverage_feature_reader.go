package pgmarketstress

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"marketengine/internal/domain"
	"marketengine/internal/marketstress"
	"marketengine/internal/storage"
)

type LeverageFeatureReader struct {
	pool *storage.Pool
}

func NewLeverageFeatureReader(pool *storage.Pool) *LeverageFeatureReader {
	return &LeverageFeatureReader{pool: pool}
}

var _ marketstress.LeverageFeatureReader = (*LeverageFeatureReader)(nil)

func (r *LeverageFeatureReader) GetBasis3mDailyAnyVersion(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) (float64, error) {
	var v float64
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		x, err := storage.GetLatestLeverageFeatureAnyVersion(ctx, tx, "basis_3m_daily", string(asset), valueDate, cutoff)
		if err != nil {
			return err
		}
		v = x
		return nil
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, domain.ErrNotFound
		}
		return 0, err
	}
	return v, nil
}
