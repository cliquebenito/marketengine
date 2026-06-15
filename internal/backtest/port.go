package backtest

import (
	"context"
	"time"

	"marketengine/internal/domain"
)

type RunID string

type BacktestRun struct {
	ID               RunID
	Mode             string
	PeriodStart      time.Time
	PeriodEnd        time.Time
	ModelVersion     string
	ConfigVersion    string
	ConfigYAML       string
	CodeSHA          string
	DataSnapshotHash string
	SLAOffsetMinutes int
	ParentRunID      *RunID
	HarnessVersion   string
	Status           string
	StartedAt        time.Time
	CompletedAt      *time.Time
	Metadata         map[string]any
}

type Metric struct {
	Name     string
	Scope    string
	Value    float64
	Metadata map[string]any
}

type PricePointAt struct {
	Date  time.Time
	Price float64
}

type RunRepo interface {
	Save(ctx context.Context, r BacktestRun) (RunID, error)
	Get(ctx context.Context, id RunID) (BacktestRun, error)
	UpdateStatus(ctx context.Context, id RunID, status string, completedAt *time.Time) error
}

type RegimeStateRepo interface {
	Save(ctx context.Context, runID RunID, state domain.RegimeState) error
	GetByRun(ctx context.Context, runID RunID) ([]domain.RegimeState, error)
}

type MetricsRepo interface {
	Save(ctx context.Context, runID RunID, m Metric) error
	GetByRun(ctx context.Context, runID RunID) ([]Metric, error)
}

type DomainScoreReader interface {
	GetByDate(ctx context.Context, asset domain.Asset, dom domain.DomainCode, valueDate, cutoff time.Time) (float64, bool, error)
	GetLatestAll(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) (map[domain.DomainCode]float64, error)
	GetHistory(ctx context.Context, asset domain.Asset, dom domain.DomainCode, lookbackDays int) ([]float64, error)
}

type IndicatorRawReader interface {
	GetIndicatorRawSeries(ctx context.Context, asset domain.Asset, from, to, cutoff time.Time) ([]domain.IndicatorPoint, error)
}

type PriceReader interface {
	GetPriceHistory(ctx context.Context, asset domain.Asset, from, to time.Time) ([]PricePointAt, error)
}

type Clock interface {
	Now() time.Time
}
