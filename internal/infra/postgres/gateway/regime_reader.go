package pggateway

import (
	"context"
	"time"

	"marketengine/internal/api/http/gateway"
	"marketengine/internal/domain"
	"marketengine/internal/storage"
)

type RegimeReader struct{ pool *storage.Pool }

func NewRegimeReader(pool *storage.Pool) *RegimeReader { return &RegimeReader{pool: pool} }

var _ gateway.RegimeReader = (*RegimeReader)(nil)

func (r *RegimeReader) GetLatest(ctx context.Context, asset domain.Asset) (*domain.RegimeState, error) {
	s, err := storage.GetLatestRegimeState(ctx, r.pool, string(asset))
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, domain.ErrNotFound
	}
	out := toDomainRegimeState(s)
	return &out, nil
}

func (r *RegimeReader) GetHistory(ctx context.Context, asset domain.Asset, from, to time.Time) ([]domain.RegimeState, error) {
	rows, err := storage.GetRegimeStateHistory(ctx, r.pool, string(asset), from, to)
	if err != nil {
		return nil, err
	}
	out := make([]domain.RegimeState, len(rows))
	for i := range rows {
		out[i] = toDomainRegimeState(&rows[i])
	}
	return out, nil
}

func (r *RegimeReader) GetByDate(ctx context.Context, asset domain.Asset, date time.Time) (*domain.RegimeState, error) {
	s, err := storage.GetRegimeStateByDate(ctx, r.pool, string(asset), date)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, domain.ErrNotFound
	}
	out := toDomainRegimeState(s)
	return &out, nil
}

func toDomainRegimeState(s *storage.RegimeState) domain.RegimeState {
	contributions := make(map[domain.DomainCode]float64, len(s.DomainContributions))
	for k, v := range s.DomainContributions {
		contributions[domain.DomainCode(k)] = v
	}
	weights := make(map[domain.DomainCode]float64, len(s.EffectiveWeights))
	for k, v := range s.EffectiveWeights {
		weights[domain.DomainCode(k)] = v
	}
	coverage := make(map[domain.DomainCode]bool, len(s.FeatureCoverageFlag))
	for k, v := range s.FeatureCoverageFlag {
		coverage[domain.DomainCode(k)] = v
	}
	drivers := make([]domain.TopDriver, len(s.TopDrivers))
	for i, d := range s.TopDrivers {
		drivers[i] = domain.TopDriver{
			Domain:       domain.DomainCode(d.Domain),
			Contribution: d.Contribution,
			Share:        d.Share,
		}
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
		DomainContributions: contributions,
		TopDrivers:          drivers,
		EffectiveWeights:    weights,
		FeatureCoverageFlag: coverage,
		InteractionFlags:    s.InteractionFlags,
	}
}
