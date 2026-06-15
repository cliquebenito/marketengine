package pgliquidity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"marketengine/internal/domain"
	"marketengine/internal/liquidity"
	"marketengine/internal/storage"
)

type FeatureRepo struct{ pool *storage.Pool }

func NewFeatureRepo(pool *storage.Pool) *FeatureRepo { return &FeatureRepo{pool: pool} }

var _ liquidity.FeatureRepo = (*FeatureRepo)(nil)

func (r *FeatureRepo) Save(ctx context.Context, f domain.Feature) error {
	if err := f.Validate(); err != nil {
		return fmt.Errorf("invalid feature: %w", err)
	}
	return r.pool.InTx(ctx, func(tx pgx.Tx) error {
		return storage.InsertLiquidityFeature(ctx, tx, storage.LiquidityFeature{
			ValueDate:         f.ValueDate,
			Asset:             string(f.Asset),
			Timeframe:         f.Timeframe,
			FeatureName:       f.Key.Name,
			FeatureVersion:    f.Key.Version,
			Value:             f.Value,
			SourceRawVersions: f.SourceRawVersions,
			CodeSHA:           f.CodeSHA,
		})
	})
}

func (r *FeatureRepo) GetLatest(ctx context.Context, key domain.FeatureKey, asset domain.Asset, valueDate, cutoff time.Time) (float64, error) {
	var v float64
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		x, err := storage.GetLatestLiquidityFeature(ctx, tx, key.Name, key.Version, string(asset), valueDate, cutoff)
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

func (r *FeatureRepo) GetSeries(ctx context.Context, key domain.FeatureKey, asset domain.Asset, from, to, cutoff time.Time) ([]float64, error) {
	var out []float64
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		s, err := storage.GetLiquidityFeatureSeries(ctx, tx, key.Name, key.Version, string(asset), from, to, cutoff)
		if err != nil {
			return err
		}
		out = s
		return nil
	})
	return out, err
}
