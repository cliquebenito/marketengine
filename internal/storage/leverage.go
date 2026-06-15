package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type RawExchangeOI struct {
	ValueDate     time.Time
	Asset         string
	Exchange      string
	OIUSD         float64
	SourceVersion string
	PayloadHash   string
}

func InsertRawExchangeOI(ctx context.Context, tx pgx.Tx, r RawExchangeOI) error {
	_, err := tx.Exec(ctx, `
INSERT INTO raw_exchange_oi (value_date, asset, exchange, oi_usd, source_version, payload_hash)
VALUES ($1, $2::asset_code, $3, $4, $5, $6)
ON CONFLICT DO NOTHING`,
		r.ValueDate, r.Asset, r.Exchange, r.OIUSD, r.SourceVersion, r.PayloadHash)
	return err
}

func AggregatedOIAsOf(ctx context.Context, tx pgx.Tx,
	asset string, valueDate, cutoff time.Time,
) (float64, bool, error) {
	rows, err := tx.Query(ctx, `
SELECT DISTINCT ON (exchange) oi_usd
FROM raw_exchange_oi
WHERE asset = $1::asset_code AND value_date = $2 AND ingested_at <= $3
  AND exchange <> 'coinglass_aggregated'
ORDER BY exchange, ingested_at DESC`,
		asset, valueDate, cutoff)
	if err != nil {
		return 0, false, err
	}
	defer rows.Close()
	var total float64
	n := 0
	for rows.Next() {
		var v float64
		if err := rows.Scan(&v); err != nil {
			return 0, false, err
		}
		total += v
		n++
	}
	return total, n > 0, rows.Err()
}

func CoinglassAggregatedOIAsOf(ctx context.Context, tx pgx.Tx,
	asset string, valueDate, cutoff time.Time,
) (float64, bool, error) {
	var v float64
	err := tx.QueryRow(ctx, `
SELECT oi_usd FROM raw_exchange_oi
WHERE asset = $1::asset_code AND exchange = 'coinglass_aggregated'
  AND value_date = $2 AND ingested_at <= $3
ORDER BY ingested_at DESC LIMIT 1`,
		asset, valueDate, cutoff).Scan(&v)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, err
	}
	return v, true, nil
}

type RawExchangeFunding struct {
	FundingTime   time.Time
	Asset         string
	Exchange      string
	Rate          float64
	SourceVersion string
	PayloadHash   string
}

func InsertRawExchangeFunding(ctx context.Context, tx pgx.Tx, r RawExchangeFunding) error {
	_, err := tx.Exec(ctx, `
INSERT INTO raw_exchange_funding (funding_time, asset, exchange, rate, source_version, payload_hash)
VALUES ($1, $2::asset_code, $3, $4, $5, $6)
ON CONFLICT DO NOTHING`,
		r.FundingTime, r.Asset, r.Exchange, r.Rate, r.SourceVersion, r.PayloadHash)
	return err
}

func DailyAvgFundingAsOf(ctx context.Context, tx pgx.Tx,
	asset string, valueDate, cutoff time.Time,
) (float64, bool, error) {
	nextDay := valueDate.AddDate(0, 0, 1)
	var avg float64
	err := tx.QueryRow(ctx, `
SELECT avg(rate) FROM (
  SELECT DISTINCT ON (exchange, funding_time) rate
  FROM raw_exchange_funding
  WHERE asset = $1::asset_code
    AND funding_time >= $2 AND funding_time < $3
    AND ingested_at <= $4
  ORDER BY exchange, funding_time, ingested_at DESC
) sub`,
		asset, valueDate, nextDay, cutoff).Scan(&avg)
	if err != nil {
		return 0, false, nil
	}
	return avg, true, nil
}

type RawExchangeLiquidations struct {
	ValueDate     time.Time
	Asset         string
	Exchange      string
	LongLiqsUSD   float64
	ShortLiqsUSD  float64
	SourceVersion string
	PayloadHash   string
}

func InsertRawExchangeLiquidations(ctx context.Context, tx pgx.Tx, r RawExchangeLiquidations) error {
	_, err := tx.Exec(ctx, `
INSERT INTO raw_exchange_liquidations
  (value_date, asset, exchange, long_liqs_usd, short_liqs_usd, source_version, payload_hash)
VALUES ($1, $2::asset_code, $3, $4, $5, $6, $7)
ON CONFLICT DO NOTHING`,
		r.ValueDate, r.Asset, r.Exchange, r.LongLiqsUSD, r.ShortLiqsUSD, r.SourceVersion, r.PayloadHash)
	return err
}

func DailyTotalLiquidationsAsOf(ctx context.Context, tx pgx.Tx,
	asset string, valueDate, cutoff time.Time,
) (float64, bool, error) {
	rows, err := tx.Query(ctx, `
SELECT DISTINCT ON (exchange) long_liqs_usd + short_liqs_usd AS total
FROM raw_exchange_liquidations
WHERE asset = $1::asset_code AND value_date = $2 AND ingested_at <= $3
  AND exchange <> 'coinglass_aggregated'
ORDER BY exchange, ingested_at DESC`,
		asset, valueDate, cutoff)
	if err != nil {
		return 0, false, err
	}
	defer rows.Close()
	var sum float64
	n := 0
	for rows.Next() {
		var v float64
		if err := rows.Scan(&v); err != nil {
			return 0, false, err
		}
		sum += v
		n++
	}
	return sum, n > 0, rows.Err()
}

func CoinglassAggregatedLiquidationsAsOf(ctx context.Context, tx pgx.Tx,
	asset string, valueDate, cutoff time.Time,
) (float64, bool, error) {
	var v float64
	err := tx.QueryRow(ctx, `
SELECT long_liqs_usd + short_liqs_usd FROM raw_exchange_liquidations
WHERE asset = $1::asset_code AND exchange = 'coinglass_aggregated'
  AND value_date = $2 AND ingested_at <= $3
ORDER BY ingested_at DESC LIMIT 1`,
		asset, valueDate, cutoff).Scan(&v)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, err
	}
	return v, true, nil
}

type RawCoinglassFuturesBasis struct {
	ValueDate          time.Time
	Symbol             string
	Exchange           string
	AnnualizedBasisPct float64
	CloseBasis         float64
	SourceVersion      string
	PayloadHash        string
}

func InsertRawCoinglassFuturesBasis(ctx context.Context, tx pgx.Tx, r RawCoinglassFuturesBasis) error {
	_, err := tx.Exec(ctx, `
INSERT INTO raw_coinglass_futures_basis
  (value_date, symbol, exchange, annualized_basis_pct, close_basis, source_version, payload_hash)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT DO NOTHING`,
		r.ValueDate, r.Symbol, r.Exchange, r.AnnualizedBasisPct, r.CloseBasis, r.SourceVersion, r.PayloadHash)
	return err
}

func GetCoinglassFuturesBasisAsOf(ctx context.Context, tx pgx.Tx,
	symbol, exchange string, valueDate, cutoff time.Time,
) (float64, bool, error) {
	var v float64
	err := tx.QueryRow(ctx, `
SELECT annualized_basis_pct FROM raw_coinglass_futures_basis
WHERE symbol = $1 AND exchange = $2 AND value_date = $3 AND ingested_at <= $4
ORDER BY ingested_at DESC LIMIT 1`, symbol, exchange, valueDate, cutoff).Scan(&v)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, err
	}
	return v, true, nil
}

type RawCoinglassLongShortRatio struct {
	ValueDate     time.Time
	Symbol        string
	Exchange      string
	LongPercent   float64
	ShortPercent  float64
	Ratio         float64
	SourceVersion string
	PayloadHash   string
}

type LSRatioKind string

const (
	LSGlobal      LSRatioKind = "global"
	LSTopAccount  LSRatioKind = "top_account"
	LSTopPosition LSRatioKind = "top_position"
)

func (k LSRatioKind) table() string {
	switch k {
	case LSGlobal:
		return "raw_coinglass_long_short_global"
	case LSTopAccount:
		return "raw_coinglass_long_short_top_account"
	case LSTopPosition:
		return "raw_coinglass_long_short_top_position"
	}
	return ""
}

func InsertRawCoinglassLongShortRatio(ctx context.Context, tx pgx.Tx, kind LSRatioKind, r RawCoinglassLongShortRatio) error {
	tbl := kind.table()
	if tbl == "" {
		return fmt.Errorf("unknown LSRatioKind %q", kind)
	}
	q := fmt.Sprintf(`
INSERT INTO %s
  (value_date, symbol, exchange, long_percent, short_percent, ratio, source_version, payload_hash)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT DO NOTHING`, tbl)
	_, err := tx.Exec(ctx, q,
		r.ValueDate, r.Symbol, r.Exchange,
		r.LongPercent, r.ShortPercent, r.Ratio,
		r.SourceVersion, r.PayloadHash)
	return err
}

func GetCoinglassLSRatioAvgAsOf(ctx context.Context, tx pgx.Tx,
	kind LSRatioKind, symbol string, valueDate, cutoff time.Time,
) (float64, bool, error) {
	tbl := kind.table()
	if tbl == "" {
		return 0, false, fmt.Errorf("unknown LSRatioKind %q", kind)
	}
	q := fmt.Sprintf(`
SELECT AVG(ratio) FROM (
  SELECT DISTINCT ON (exchange) exchange, ratio
  FROM %s
  WHERE symbol = $1 AND value_date = $2 AND ingested_at <= $3
  ORDER BY exchange, ingested_at DESC
) t`, tbl)
	var v *float64
	if err := tx.QueryRow(ctx, q, symbol, valueDate, cutoff).Scan(&v); err != nil {
		if err == pgx.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, err
	}
	if v == nil {
		return 0, false, nil
	}
	return *v, true, nil
}

type RawCoinglassTakerVolume struct {
	ValueDate     time.Time
	Symbol        string
	BuyVolumeUSD  float64
	SellVolumeUSD float64
	SourceVersion string
	PayloadHash   string
}

func InsertRawCoinglassTakerVolume(ctx context.Context, tx pgx.Tx, r RawCoinglassTakerVolume) error {
	_, err := tx.Exec(ctx, `
INSERT INTO raw_coinglass_taker_volume_aggregated
  (value_date, symbol, buy_volume_usd, sell_volume_usd, source_version, payload_hash)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT DO NOTHING`,
		r.ValueDate, r.Symbol, r.BuyVolumeUSD, r.SellVolumeUSD, r.SourceVersion, r.PayloadHash)
	return err
}

func GetCoinglassTakerVolumeAsOf(ctx context.Context, tx pgx.Tx,
	symbol string, valueDate, cutoff time.Time,
) (buy, sell float64, ok bool, err error) {
	err = tx.QueryRow(ctx, `
SELECT buy_volume_usd, sell_volume_usd
FROM raw_coinglass_taker_volume_aggregated
WHERE symbol = $1 AND value_date = $2 AND ingested_at <= $3
ORDER BY ingested_at DESC LIMIT 1`, symbol, valueDate, cutoff).Scan(&buy, &sell)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, 0, false, nil
		}
		return 0, 0, false, err
	}
	return buy, sell, true, nil
}

type RawCoinglassBorrowRate struct {
	ValueDate     time.Time
	Symbol        string
	Exchange      string
	InterestRate  float64
	SourceVersion string
	PayloadHash   string
}

func InsertRawCoinglassBorrowRate(ctx context.Context, tx pgx.Tx, r RawCoinglassBorrowRate) error {
	_, err := tx.Exec(ctx, `
INSERT INTO raw_coinglass_borrow_rate
  (value_date, symbol, exchange, interest_rate, source_version, payload_hash)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT DO NOTHING`,
		r.ValueDate, r.Symbol, r.Exchange, r.InterestRate, r.SourceVersion, r.PayloadHash)
	return err
}

type RawDeribitBasis struct {
	ValueDate       time.Time
	Asset           string
	InstrumentName  string
	FuturesPrice    float64
	SpotPrice       float64
	AnnualizedBasis float64
	DaysToExpiry    int
	SourceVersion   string
	PayloadHash     string
}

func InsertRawDeribitBasis(ctx context.Context, tx pgx.Tx, r RawDeribitBasis) error {
	_, err := tx.Exec(ctx, `
INSERT INTO raw_deribit_basis
  (value_date, asset, instrument_name, futures_price, spot_price, annualized_basis,
   days_to_expiry, source_version, payload_hash)
VALUES ($1, $2::asset_code, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT DO NOTHING`,
		r.ValueDate, r.Asset, r.InstrumentName, r.FuturesPrice, r.SpotPrice,
		r.AnnualizedBasis, r.DaysToExpiry, r.SourceVersion, r.PayloadHash)
	return err
}

func GetDeribitBasisAsOf(ctx context.Context, tx pgx.Tx,
	asset string, valueDate, cutoff time.Time,
) (float64, bool, error) {
	var v float64
	err := tx.QueryRow(ctx, `
SELECT annualized_basis FROM raw_deribit_basis
WHERE asset = $1::asset_code AND value_date = $2 AND ingested_at <= $3
ORDER BY ingested_at DESC LIMIT 1`,
		asset, valueDate, cutoff).Scan(&v)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, err
	}
	return v, true, nil
}

type LeverageFeature struct {
	ValueDate         time.Time
	Asset             string
	Timeframe         string
	FeatureName       string
	FeatureVersion    string
	Value             float64
	SourceRawVersions map[string]string
	CodeSHA           string
}

func InsertLeverageFeature(ctx context.Context, tx pgx.Tx, f LeverageFeature) error {
	_, err := tx.Exec(ctx, `
INSERT INTO features_leverage
  (value_date, asset, timeframe, feature_name, feature_version,
   value, source_raw_versions, code_sha)
VALUES ($1, $2::asset_code, $3, $4, $5, $6, $7, $8)
ON CONFLICT DO NOTHING`,
		f.ValueDate, f.Asset, f.Timeframe, f.FeatureName, f.FeatureVersion,
		f.Value, f.SourceRawVersions, f.CodeSHA)
	return err
}

func GetLatestLeverageFeature(ctx context.Context, tx pgx.Tx,
	featureName, featureVersion, asset string, valueDate, cutoff time.Time,
) (float64, error) {
	var v float64
	err := tx.QueryRow(ctx, `
SELECT value FROM features_leverage
WHERE feature_name = $1 AND feature_version = $2
  AND asset = $3::asset_code AND value_date = $4
  AND ingested_at <= $5
ORDER BY ingested_at DESC LIMIT 1`,
		featureName, featureVersion, asset, valueDate, cutoff).Scan(&v)
	return v, err
}

func GetLatestLeverageFeatureAnyVersion(ctx context.Context, tx pgx.Tx,
	featureName, asset string, valueDate, cutoff time.Time,
) (float64, error) {
	var v float64
	err := tx.QueryRow(ctx, `
SELECT value FROM features_leverage
WHERE feature_name = $1
  AND asset = $2::asset_code AND value_date = $3
  AND ingested_at <= $4
ORDER BY ingested_at DESC LIMIT 1`,
		featureName, asset, valueDate, cutoff).Scan(&v)
	return v, err
}

func GetLeverageFeatureSeries(ctx context.Context, tx pgx.Tx,
	featureName, featureVersion, asset string, from, to, cutoff time.Time,
) ([]float64, error) {
	rows, err := tx.Query(ctx, `
SELECT DISTINCT ON (value_date) value
FROM features_leverage
WHERE feature_name = $1 AND feature_version = $2
  AND asset = $3::asset_code
  AND value_date BETWEEN $4 AND $5
  AND ingested_at <= $6
ORDER BY value_date, ingested_at DESC`,
		featureName, featureVersion, asset, from, to, cutoff)
	if err != nil {
		return nil, err
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
