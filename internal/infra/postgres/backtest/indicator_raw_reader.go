package pgbacktest

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"marketengine/internal/backtest"
	"marketengine/internal/domain"
	"marketengine/internal/storage"
)

type IndicatorRawReader struct{ pool *storage.Pool }

func NewIndicatorRawReader(pool *storage.Pool) *IndicatorRawReader {
	return &IndicatorRawReader{pool: pool}
}

var _ backtest.IndicatorRawReader = (*IndicatorRawReader)(nil)

func (r *IndicatorRawReader) GetIndicatorRawSeries(ctx context.Context, asset domain.Asset, from, to, _ time.Time) ([]domain.IndicatorPoint, error) {
	var out []domain.IndicatorPoint
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		pts, terr := storage.GetRegimeIndicatorRawSeries(ctx, tx, string(asset), from, to)
		if terr != nil {
			return terr
		}
		out = make([]domain.IndicatorPoint, 0, len(pts))
		for _, p := range pts {
			out = append(out, domain.IndicatorPoint{
				ValueDate:       p.ValueDate,
				RegimeIndicator: p.RegimeIndicator,
			})
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return out, nil
		}
		return nil, err
	}
	return out, nil
}
