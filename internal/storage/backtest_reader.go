package storage

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

type BacktestRunSummary struct {
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

func ListBacktestRuns(ctx context.Context, tx pgx.Tx, limit int) ([]BacktestRunSummary, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := tx.Query(ctx, `
SELECT run_id::text, mode, period_start, period_end, model_version, config_version,
       status, started_at, completed_at, parent_run_id::text
  FROM backtest_runs
 ORDER BY started_at DESC
 LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]BacktestRunSummary, 0, limit)
	for rows.Next() {
		var r BacktestRunSummary
		var completed *time.Time
		var parent *string
		if err := rows.Scan(&r.RunID, &r.Mode, &r.PeriodStart, &r.PeriodEnd,
			&r.ModelVersion, &r.ConfigVersion, &r.Status, &r.StartedAt,
			&completed, &parent); err != nil {
			return nil, err
		}
		r.CompletedAt = completed
		r.ParentRunID = parent
		out = append(out, r)
	}
	return out, rows.Err()
}

type BacktestTimelinePoint struct {
	ValueDate       time.Time
	RegimeIndicator float64
	RiskOnProb      float64
	RiskOffProb     float64
	TransitionRisk  float64
	Price           float64
	PriceOK         bool
}

func GetBacktestTimeline(ctx context.Context, tx pgx.Tx,
	runID, asset string,
) ([]BacktestTimelinePoint, error) {
	symbol := asset + "USDT"
	rows, err := tx.Query(ctx, `
SELECT brs.value_date,
       brs.regime_indicator,
       brs.risk_on_probability,
       brs.risk_off_probability,
       brs.transition_risk,
       k.close
  FROM backtest_regime_states brs
  LEFT JOIN LATERAL (
    SELECT close FROM raw_binance_klines
     WHERE symbol = $3 AND value_date = brs.value_date
     ORDER BY ingested_at DESC LIMIT 1
  ) k ON TRUE
 WHERE brs.run_id = $1::uuid AND brs.asset = $2::asset_code
 ORDER BY brs.value_date`,
		runID, asset, symbol)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BacktestTimelinePoint
	for rows.Next() {
		var p BacktestTimelinePoint
		var price *float64
		if err := rows.Scan(&p.ValueDate, &p.RegimeIndicator, &p.RiskOnProb,
			&p.RiskOffProb, &p.TransitionRisk, &price); err != nil {
			return nil, err
		}
		if price != nil {
			p.Price = *price
			p.PriceOK = true
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

type BacktestEventResult struct {
	Name               string
	PeakDate           time.Time
	FirstRiskOffOffset int
	FirstTransOffset   int
	DataPresent        bool
}

func GetBacktestEventResults(ctx context.Context, tx pgx.Tx,
	runID, asset string, events []struct {
		Name string
		Peak time.Time
	}, transitionThreshold float64,
) ([]BacktestEventResult, error) {
	out := make([]BacktestEventResult, 0, len(events))
	for _, ev := range events {
		from := ev.Peak.AddDate(0, 0, -14)
		to := ev.Peak.AddDate(0, 0, 3)
		rows, err := tx.Query(ctx, `
SELECT value_date, regime_indicator, transition_risk
  FROM backtest_regime_states
 WHERE run_id = $1::uuid AND asset = $2::asset_code
   AND value_date BETWEEN $3 AND $4
 ORDER BY value_date`, runID, asset, from, to)
		if err != nil {
			return nil, err
		}
		res := BacktestEventResult{
			Name: ev.Name, PeakDate: ev.Peak,
			FirstRiskOffOffset: -999, FirstTransOffset: -999,
		}
		for rows.Next() {
			var d time.Time
			var ri, tr float64
			if err := rows.Scan(&d, &ri, &tr); err != nil {
				rows.Close()
				return nil, err
			}
			res.DataPresent = true
			off := int(ev.Peak.Sub(d).Hours() / 24)
			if res.FirstRiskOffOffset == -999 && ri < 0 {
				res.FirstRiskOffOffset = off
			}
			if res.FirstTransOffset == -999 && tr > transitionThreshold {
				res.FirstTransOffset = off
			}
		}
		rows.Close()
		out = append(out, res)
	}
	return out, nil
}
