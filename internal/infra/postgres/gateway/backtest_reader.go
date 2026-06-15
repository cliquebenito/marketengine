package pggateway

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"marketengine/internal/api/http/gateway"
	"marketengine/internal/domain"
	"marketengine/internal/storage"
)

type BacktestReader struct{ pool *storage.Pool }

func NewBacktestReader(pool *storage.Pool) *BacktestReader { return &BacktestReader{pool: pool} }

var _ gateway.BacktestReader = (*BacktestReader)(nil)

func (r *BacktestReader) ListRuns(ctx context.Context, limit int) ([]gateway.BacktestRun, error) {
	var out []gateway.BacktestRun
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		raws, err := storage.ListBacktestRuns(ctx, tx, limit)
		if err != nil {
			return err
		}
		for _, s := range raws {
			run := gateway.BacktestRun{
				RunID:         s.RunID,
				Mode:          s.Mode,
				PeriodStart:   s.PeriodStart,
				PeriodEnd:     s.PeriodEnd,
				ModelVersion:  s.ModelVersion,
				ConfigVersion: s.ConfigVersion,
				Status:        s.Status,
				StartedAt:     s.StartedAt,
				CompletedAt:   s.CompletedAt,
				ParentRunID:   s.ParentRunID,
			}
			out = append(out, run)
		}
		return nil
	})
	return out, err
}

func (r *BacktestReader) GetTimeline(ctx context.Context, runID string, asset domain.Asset) ([]gateway.BacktestPoint, error) {
	var out []gateway.BacktestPoint
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		pts, err := storage.GetBacktestTimeline(ctx, tx, runID, string(asset))
		if err != nil {
			return err
		}
		for _, p := range pts {
			out = append(out, gateway.BacktestPoint{
				ValueDate:       p.ValueDate,
				RegimeIndicator: p.RegimeIndicator,
				RiskOnProb:      p.RiskOnProb,
				RiskOffProb:     p.RiskOffProb,
				TransitionRisk:  p.TransitionRisk,
				Price:           p.Price,
				PriceOK:         p.PriceOK,
			})
		}
		return nil
	})
	return out, err
}

var calibrationEvents = []struct {
	Name string
	Peak time.Time
}{
	{"China ban / leverage flush", date("2021-05-19")},
	{"UST collapse", date("2022-05-11")},
	{"3AC collapse", date("2022-06-18")},
	{"FTX collapse", date("2022-11-09")},
	{"USDC depeg / SVB", date("2023-03-11")},
	{"Yen carry unwind", date("2024-08-05")},
}

func date(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t.UTC()
}

func (r *BacktestReader) GetEvents(ctx context.Context, runID string, asset domain.Asset) ([]gateway.BacktestEvent, error) {
	var out []gateway.BacktestEvent
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		evs := make([]struct {
			Name string
			Peak time.Time
		}, len(calibrationEvents))
		copy(evs, calibrationEvents)
		raws, err := storage.GetBacktestEventResults(ctx, tx, runID, string(asset), evs, 0.6)
		if err != nil {
			return err
		}
		for _, e := range raws {
			out = append(out, gateway.BacktestEvent{
				Name:               e.Name,
				PeakDate:           e.PeakDate,
				FirstRiskOffOffset: e.FirstRiskOffOffset,
				FirstTransOffset:   e.FirstTransOffset,
				DataPresent:        e.DataPresent,
			})
		}
		return nil
	})
	return out, err
}
