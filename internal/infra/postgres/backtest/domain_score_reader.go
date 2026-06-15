package pgbacktest

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"marketengine/internal/backtest"
	"marketengine/internal/domain"
	"marketengine/internal/storage"
)

type DomainScoreReader struct{ pool *storage.Pool }

func NewDomainScoreReader(pool *storage.Pool) *DomainScoreReader {
	return &DomainScoreReader{pool: pool}
}

var _ backtest.DomainScoreReader = (*DomainScoreReader)(nil)

func queryAssetFor(asset domain.Asset, dom domain.DomainCode) string {
	if dom == domain.DomainCapitalFlows {
		return string(domain.AssetGlobal)
	}
	return string(asset)
}

func (r *DomainScoreReader) GetLatestAll(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) (map[domain.DomainCode]float64, error) {
	out := make(map[domain.DomainCode]float64, len(domain.AllDomains()))
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		assetMap, err := storage.GetLatestDomainScores(ctx, tx, string(asset), valueDate, cutoff)
		if err != nil {
			return err
		}
		for k, v := range assetMap {
			d := domain.DomainCode(k)
			if d == domain.DomainCapitalFlows {
				continue
			}
			out[d] = v
		}
		globalMap, err := storage.GetLatestDomainScores(ctx, tx, string(domain.AssetGlobal), valueDate, cutoff)
		if err != nil {
			return err
		}
		if v, ok := globalMap[string(domain.DomainCapitalFlows)]; ok {
			out[domain.DomainCapitalFlows] = v
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *DomainScoreReader) GetByDate(ctx context.Context, asset domain.Asset, dom domain.DomainCode, valueDate, cutoff time.Time) (float64, bool, error) {
	var score float64
	var found bool
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		v, ok, err := storage.GetDomainScoreOnDate(ctx, tx, queryAssetFor(asset, dom), string(dom), valueDate, cutoff)
		if err != nil {
			return err
		}
		score, found = v, ok
		return nil
	})
	if err != nil {
		return 0, false, err
	}
	return score, found, nil
}

func (r *DomainScoreReader) GetHistory(ctx context.Context, asset domain.Asset, dom domain.DomainCode, lookbackDays int) ([]float64, error) {
	var out []float64
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		s, err := storage.GetDomainScoreHistory(ctx, tx, queryAssetFor(asset, dom), string(dom), lookbackDays)
		if err != nil {
			return err
		}
		out = s
		return nil
	})
	return out, err
}
