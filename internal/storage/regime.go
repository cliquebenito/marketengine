package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type RegimeState struct {
	Asset               string
	ValueDate           time.Time
	RegimeIndicator     float64
	RegimeIndicatorRaw  float64
	RiskOnProbability   float64
	RiskOffProbability  float64
	TransitionRisk      float64
	ModelVersion        string
	ConfigVersion       string
	CodeSHA             string
	DomainContributions map[string]float64
	TopDrivers          []TopDriver
	EffectiveWeights    map[string]float64
	FeatureCoverageFlag map[string]bool
	InteractionFlags    []string
}

type TopDriver struct {
	Domain       string  `json:"domain"`
	Contribution float64 `json:"contribution"`
	Share        float64 `json:"share"`
}

func InsertRegimeState(ctx context.Context, tx pgx.Tx, s RegimeState) error {
	_, err := tx.Exec(ctx, `
INSERT INTO regime_states
  (asset, value_date, regime_indicator, regime_indicator_raw,
   risk_on_probability, risk_off_probability, transition_risk,
   model_version, config_version, code_sha,
   domain_contributions, top_drivers, effective_weights,
   feature_coverage_flag, interaction_flags)
VALUES ($1::asset_code, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`,
		s.Asset, s.ValueDate, s.RegimeIndicator, s.RegimeIndicatorRaw,
		s.RiskOnProbability, s.RiskOffProbability, s.TransitionRisk,
		s.ModelVersion, s.ConfigVersion, s.CodeSHA,
		s.DomainContributions, s.TopDrivers, s.EffectiveWeights,
		s.FeatureCoverageFlag, s.InteractionFlags)
	return err
}

func GetLatestDomainScores(ctx context.Context, tx pgx.Tx,
	asset string, valueDate, cutoff time.Time,
) (map[string]float64, error) {
	rows, err := tx.Query(ctx, `
SELECT DISTINCT ON (domain) domain, score
FROM domain_scores
WHERE asset = $1::asset_code AND value_date = $2
  AND ingested_at <= $3
ORDER BY domain, ingested_at DESC`,
		asset, valueDate, cutoff)
	if err != nil {
		return nil, fmt.Errorf("query domain scores: %w", err)
	}
	defer rows.Close()
	out := make(map[string]float64)
	for rows.Next() {
		var domain string
		var score float64
		if err := rows.Scan(&domain, &score); err != nil {
			return nil, err
		}
		out[domain] = score
	}
	return out, rows.Err()
}

func GetDomainScoreHistory(ctx context.Context, tx pgx.Tx,
	asset, domain string, lookbackDays int,
) ([]float64, error) {
	rows, err := tx.Query(ctx, `
SELECT score FROM (
  SELECT DISTINCT ON (value_date) value_date, score
  FROM domain_scores
  WHERE asset = $1::asset_code AND domain = $2
    AND value_date >= CURRENT_DATE - $3::int
  ORDER BY value_date, ingested_at DESC
) sub
ORDER BY value_date ASC`,
		asset, domain, lookbackDays)
	if err != nil {
		return nil, fmt.Errorf("query domain score history: %w", err)
	}
	defer rows.Close()
	var out []float64
	for rows.Next() {
		var v float64
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func GetDomainScoreOnDate(ctx context.Context, tx pgx.Tx,
	asset, domain string, valueDate, cutoff time.Time,
) (float64, bool, error) {
	var score float64
	err := tx.QueryRow(ctx, `
SELECT score FROM domain_scores
WHERE asset = $1::asset_code AND domain = $2
  AND value_date = $3 AND ingested_at <= $4
ORDER BY ingested_at DESC
LIMIT 1`,
		asset, domain, valueDate, cutoff).Scan(&score)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("query domain score on date: %w", err)
	}
	return score, true, nil
}

type RegimeIndicatorPoint struct {
	ValueDate       time.Time
	RegimeIndicator float64
}

type DomainScoreRow struct {
	Asset        string
	Domain       string
	ValueDate    time.Time
	Score        float64
	Components   map[string]float64
	DataQuality  map[string]any
	ModelVersion string
}

func scanRegimeState(row pgx.Row) (*RegimeState, error) {
	var s RegimeState
	if err := row.Scan(
		&s.Asset, &s.ValueDate, &s.RegimeIndicator, &s.RegimeIndicatorRaw,
		&s.RiskOnProbability, &s.RiskOffProbability, &s.TransitionRisk,
		&s.ModelVersion, &s.ConfigVersion, &s.CodeSHA,
		&s.DomainContributions, &s.TopDrivers, &s.EffectiveWeights,
		&s.FeatureCoverageFlag, &s.InteractionFlags,
	); err != nil {
		return nil, err
	}
	return &s, nil
}

const regimeStateCols = `asset, value_date, regime_indicator, regime_indicator_raw,
  risk_on_probability, risk_off_probability, transition_risk,
  model_version, config_version, code_sha,
  domain_contributions, top_drivers, effective_weights,
  feature_coverage_flag, interaction_flags`

func GetLatestRegimeState(ctx context.Context, pool *Pool, asset string) (*RegimeState, error) {
	row := pool.QueryRow(ctx, `
SELECT `+regimeStateCols+`
FROM regime_states
WHERE asset = $1::asset_code
ORDER BY value_date DESC, ingested_at DESC
LIMIT 1`, asset)
	s, err := scanRegimeState(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("latest regime state: %w", err)
	}
	return s, nil
}

func GetRegimeStateHistory(ctx context.Context, pool *Pool,
	asset string, from, to time.Time,
) ([]RegimeState, error) {
	rows, err := pool.Query(ctx, `
SELECT `+regimeStateCols+` FROM (
  SELECT DISTINCT ON (value_date) `+regimeStateCols+`
  FROM regime_states
  WHERE asset = $1::asset_code AND value_date BETWEEN $2 AND $3
  ORDER BY value_date, ingested_at DESC
) sub
ORDER BY value_date ASC`, asset, from, to)
	if err != nil {
		return nil, fmt.Errorf("history regime state: %w", err)
	}
	defer rows.Close()
	var out []RegimeState
	for rows.Next() {
		var s RegimeState
		if err := rows.Scan(
			&s.Asset, &s.ValueDate, &s.RegimeIndicator, &s.RegimeIndicatorRaw,
			&s.RiskOnProbability, &s.RiskOffProbability, &s.TransitionRisk,
			&s.ModelVersion, &s.ConfigVersion, &s.CodeSHA,
			&s.DomainContributions, &s.TopDrivers, &s.EffectiveWeights,
			&s.FeatureCoverageFlag, &s.InteractionFlags,
		); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func GetRegimeStateByDate(ctx context.Context, pool *Pool,
	asset string, date time.Time,
) (*RegimeState, error) {
	row := pool.QueryRow(ctx, `
SELECT `+regimeStateCols+`
FROM regime_states
WHERE asset = $1::asset_code AND value_date = $2
ORDER BY ingested_at DESC
LIMIT 1`, asset, date)
	s, err := scanRegimeState(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("regime state by date: %w", err)
	}
	return s, nil
}

func GetDomainScoreTimeline(ctx context.Context, pool *Pool,
	asset, domain string, from, to time.Time,
) ([]DomainScoreRow, error) {
	rows, err := pool.Query(ctx, `
SELECT asset, domain, value_date, score, components, data_quality, model_version FROM (
  SELECT DISTINCT ON (value_date)
    asset, domain, value_date, score, components, data_quality, model_version, ingested_at
  FROM domain_scores
  WHERE asset = $1::asset_code AND domain = $2::domain_code
    AND value_date BETWEEN $3 AND $4
  ORDER BY value_date, ingested_at DESC
) sub
ORDER BY value_date ASC`, asset, domain, from, to)
	if err != nil {
		return nil, fmt.Errorf("domain score timeline: %w", err)
	}
	defer rows.Close()
	var out []DomainScoreRow
	for rows.Next() {
		var r DomainScoreRow
		if err := rows.Scan(&r.Asset, &r.Domain, &r.ValueDate, &r.Score,
			&r.Components, &r.DataQuality, &r.ModelVersion); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func GetRegimeIndicatorSeries(ctx context.Context, tx pgx.Tx,
	asset string, from, to time.Time,
) ([]RegimeIndicatorPoint, error) {
	return queryRegimeSeries(ctx, tx, asset, from, to, "regime_indicator")
}

func GetRegimeIndicatorRawSeries(ctx context.Context, tx pgx.Tx,
	asset string, from, to time.Time,
) ([]RegimeIndicatorPoint, error) {
	return queryRegimeSeries(ctx, tx, asset, from, to, "regime_indicator_raw")
}

func queryRegimeSeries(ctx context.Context, tx pgx.Tx,
	asset string, from, to time.Time, column string,
) ([]RegimeIndicatorPoint, error) {

	sql := fmt.Sprintf(`
SELECT DISTINCT ON (value_date) value_date, %s
FROM regime_states
WHERE asset = $1::asset_code
  AND value_date BETWEEN $2 AND $3
ORDER BY value_date, ingested_at DESC`, column)
	rows, err := tx.Query(ctx, sql, asset, from, to)
	if err != nil {
		return nil, fmt.Errorf("query regime %s series: %w", column, err)
	}
	defer rows.Close()
	var out []RegimeIndicatorPoint
	for rows.Next() {
		var p RegimeIndicatorPoint
		if err := rows.Scan(&p.ValueDate, &p.RegimeIndicator); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

type SmoothingTrailingPoint struct {
	ValueDate       time.Time
	RegimeIndicator float64
	TransitionRisk  float64
}

func GetRegimeSmoothingTrailing(ctx context.Context, tx pgx.Tx,
	asset string, from, to, cutoff time.Time,
) ([]SmoothingTrailingPoint, error) {
	rows, err := tx.Query(ctx, `
SELECT value_date, regime_indicator, transition_risk FROM (
  SELECT DISTINCT ON (value_date)
    value_date, regime_indicator, transition_risk, ingested_at
  FROM regime_states
  WHERE asset = $1::asset_code
    AND value_date BETWEEN $2 AND $3
    AND ingested_at <= $4
  ORDER BY value_date, ingested_at DESC
) sub
ORDER BY value_date ASC`,
		asset, from, to, cutoff)
	if err != nil {
		return nil, fmt.Errorf("query regime smoothing trailing: %w", err)
	}
	defer rows.Close()
	var out []SmoothingTrailingPoint
	for rows.Next() {
		var p SmoothingTrailingPoint
		if err := rows.Scan(&p.ValueDate, &p.RegimeIndicator, &p.TransitionRisk); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
