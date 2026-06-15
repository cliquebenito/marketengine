package pgregime

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"marketengine/internal/domain"
	"marketengine/internal/regime"
	"marketengine/internal/storage"
)

type RegimeRepo struct{ pool *storage.Pool }

func NewRegimeRepo(pool *storage.Pool) *RegimeRepo { return &RegimeRepo{pool: pool} }

var _ regime.RegimeRepo = (*RegimeRepo)(nil)

func (r *RegimeRepo) Save(ctx context.Context, s domain.RegimeState) error {
	if err := s.Validate(); err != nil {
		return fmt.Errorf("invalid regime state: %w", err)
	}
	return r.pool.InTx(ctx, func(tx pgx.Tx) error {
		return storage.InsertRegimeState(ctx, tx, toStorageRegimeState(s))
	})
}

func (r *RegimeRepo) GetLatest(ctx context.Context, asset domain.Asset) (domain.RegimeState, error) {
	s, err := storage.GetLatestRegimeState(ctx, r.pool, string(asset))
	if err != nil {
		return domain.RegimeState{}, err
	}
	if s == nil {
		return domain.RegimeState{}, domain.ErrNotFound
	}
	return fromStorageRegimeState(*s), nil
}

func (r *RegimeRepo) GetByDate(ctx context.Context, asset domain.Asset, valueDate time.Time) (domain.RegimeState, error) {
	s, err := storage.GetRegimeStateByDate(ctx, r.pool, string(asset), valueDate)
	if err != nil {
		return domain.RegimeState{}, err
	}
	if s == nil {
		return domain.RegimeState{}, domain.ErrNotFound
	}
	return fromStorageRegimeState(*s), nil
}

func (r *RegimeRepo) GetHistory(ctx context.Context, asset domain.Asset, from, to time.Time) ([]domain.RegimeState, error) {
	rows, err := storage.GetRegimeStateHistory(ctx, r.pool, string(asset), from, to)
	if err != nil {
		return nil, err
	}
	out := make([]domain.RegimeState, 0, len(rows))
	for _, s := range rows {
		out = append(out, fromStorageRegimeState(s))
	}
	return out, nil
}

func (r *RegimeRepo) GetIndicatorRawSeries(ctx context.Context, asset domain.Asset, from, to time.Time) ([]domain.IndicatorPoint, error) {
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

func (r *RegimeRepo) GetSmoothingTrailing(ctx context.Context, asset domain.Asset, from, to, cutoff time.Time) ([]regime.SmoothingTrailingPoint, error) {
	var out []regime.SmoothingTrailingPoint
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		pts, terr := storage.GetRegimeSmoothingTrailing(ctx, tx, string(asset), from, to, cutoff)
		if terr != nil {
			return terr
		}
		out = make([]regime.SmoothingTrailingPoint, 0, len(pts))
		for _, p := range pts {
			out = append(out, regime.SmoothingTrailingPoint{
				ValueDate:       p.ValueDate,
				RegimeIndicator: p.RegimeIndicator,
				TransitionRisk:  p.TransitionRisk,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func toStorageRegimeState(s domain.RegimeState) storage.RegimeState {
	contrib := make(map[string]float64, len(s.DomainContributions))
	for d, v := range s.DomainContributions {
		contrib[string(d)] = v
	}
	weights := make(map[string]float64, len(s.EffectiveWeights))
	for d, v := range s.EffectiveWeights {
		weights[string(d)] = v
	}
	cov := make(map[string]bool, len(s.FeatureCoverageFlag))
	for d, v := range s.FeatureCoverageFlag {
		cov[string(d)] = v
	}
	drivers := make([]storage.TopDriver, 0, len(s.TopDrivers))
	for _, d := range s.TopDrivers {
		drivers = append(drivers, storage.TopDriver{
			Domain:       string(d.Domain),
			Contribution: d.Contribution,
			Share:        d.Share,
		})
	}
	flags := s.InteractionFlags
	if flags == nil {
		flags = []string{}
	}
	return storage.RegimeState{
		Asset:               string(s.Asset),
		ValueDate:           s.ValueDate,
		RegimeIndicator:     s.RegimeIndicator,
		RegimeIndicatorRaw:  s.RegimeIndicatorRaw,
		RiskOnProbability:   s.RiskOnProbability,
		RiskOffProbability:  s.RiskOffProbability,
		TransitionRisk:      s.TransitionRisk,
		ModelVersion:        s.ModelVersion,
		ConfigVersion:       s.ConfigVersion,
		CodeSHA:             s.CodeSHA,
		DomainContributions: contrib,
		TopDrivers:          drivers,
		EffectiveWeights:    weights,
		FeatureCoverageFlag: cov,
		InteractionFlags:    flags,
	}
}

func fromStorageRegimeState(s storage.RegimeState) domain.RegimeState {
	contrib := make(map[domain.DomainCode]float64, len(s.DomainContributions))
	for d, v := range s.DomainContributions {
		contrib[domain.DomainCode(d)] = v
	}
	weights := make(map[domain.DomainCode]float64, len(s.EffectiveWeights))
	for d, v := range s.EffectiveWeights {
		weights[domain.DomainCode(d)] = v
	}
	cov := make(map[domain.DomainCode]bool, len(s.FeatureCoverageFlag))
	for d, v := range s.FeatureCoverageFlag {
		cov[domain.DomainCode(d)] = v
	}
	drivers := make([]domain.TopDriver, 0, len(s.TopDrivers))
	for _, d := range s.TopDrivers {
		drivers = append(drivers, domain.TopDriver{
			Domain:       domain.DomainCode(d.Domain),
			Contribution: d.Contribution,
			Share:        d.Share,
		})
	}
	return domain.RegimeState{
		Asset:               domain.Asset(s.Asset),
		ValueDate:           s.ValueDate,
		RegimeIndicator:     s.RegimeIndicator,
		RegimeIndicatorRaw:  s.RegimeIndicatorRaw,
		RiskOnProbability:   s.RiskOnProbability,
		RiskOffProbability:  s.RiskOffProbability,
		TransitionRisk:      s.TransitionRisk,
		ModelVersion:        s.ModelVersion,
		ConfigVersion:       s.ConfigVersion,
		CodeSHA:             s.CodeSHA,
		DomainContributions: contrib,
		TopDrivers:          drivers,
		EffectiveWeights:    weights,
		FeatureCoverageFlag: cov,
		InteractionFlags:    s.InteractionFlags,
	}
}
