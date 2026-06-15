package regime

import (
	"context"
	"time"

	"marketengine/internal/domain"
)

type RegimeRepo interface {
	Save(ctx context.Context, s domain.RegimeState) error
	GetLatest(ctx context.Context, asset domain.Asset) (domain.RegimeState, error)
	GetByDate(ctx context.Context, asset domain.Asset, valueDate time.Time) (domain.RegimeState, error)
	GetHistory(ctx context.Context, asset domain.Asset, from, to time.Time) ([]domain.RegimeState, error)
	GetIndicatorRawSeries(ctx context.Context, asset domain.Asset, from, to time.Time) ([]domain.IndicatorPoint, error)
	GetSmoothingTrailing(ctx context.Context, asset domain.Asset, from, to, cutoff time.Time) ([]SmoothingTrailingPoint, error)
}

type DomainScoreReader interface {
	GetLatestAll(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) (map[domain.DomainCode]float64, error)

	GetByDate(ctx context.Context, asset domain.Asset, dom domain.DomainCode, valueDate, cutoff time.Time) (float64, bool, error)

	GetHistory(ctx context.Context, asset domain.Asset, dom domain.DomainCode, lookbackDays int) ([]float64, error)
}

type Publisher interface {
	Publish(ctx context.Context, ev domain.Event) error
}

type Clock interface {
	Now() time.Time
}

type SmoothingTrailingPoint struct {
	ValueDate       time.Time
	RegimeIndicator float64
	TransitionRisk  float64
}
