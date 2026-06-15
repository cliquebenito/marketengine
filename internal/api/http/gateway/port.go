package gateway

import (
	"context"
	"time"

	"marketengine/internal/domain"
)

type RegimeReader interface {
	GetLatest(ctx context.Context, asset domain.Asset) (*domain.RegimeState, error)
	GetHistory(ctx context.Context, asset domain.Asset, from, to time.Time) ([]domain.RegimeState, error)
	GetByDate(ctx context.Context, asset domain.Asset, date time.Time) (*domain.RegimeState, error)
}

type DomainScoreReader interface {
	GetTimeline(ctx context.Context, asset domain.Asset, dom domain.DomainCode, from, to time.Time) ([]domain.DomainScore, error)
}

type HealthChecker interface {
	Ping(ctx context.Context) error
}

type BacktestReader interface {
	ListRuns(ctx context.Context, limit int) ([]BacktestRun, error)
	GetTimeline(ctx context.Context, runID string, asset domain.Asset) ([]BacktestPoint, error)
	GetEvents(ctx context.Context, runID string, asset domain.Asset) ([]BacktestEvent, error)
}

type BacktestRun struct {
	RunID         string
	Mode          string
	PeriodStart   time.Time
	PeriodEnd     time.Time
	ModelVersion  string
	ConfigVersion string
	Status        string
	StartedAt     time.Time
	CompletedAt   *time.Time
	ParentRunID   *string
}

type BacktestPoint struct {
	ValueDate       time.Time
	RegimeIndicator float64
	RiskOnProb      float64
	RiskOffProb     float64
	TransitionRisk  float64
	Price           float64
	PriceOK         bool
}

type BacktestEvent struct {
	Name               string
	PeakDate           time.Time
	FirstRiskOffOffset int
	FirstTransOffset   int
	DataPresent        bool
}
