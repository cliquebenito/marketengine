package storage

import (
	"context"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
)

type RawDeribitDVOL struct {
	ValueDate     time.Time
	Asset         string
	DVOLClose     float64
	SourceVersion string
	PayloadHash   string
}

func InsertRawDeribitDVOL(ctx context.Context, tx pgx.Tx, r RawDeribitDVOL) error {
	_, err := tx.Exec(ctx, `
INSERT INTO raw_deribit_dvol (value_date, asset, dvol_close, source_version, payload_hash)
VALUES ($1, $2::asset_code, $3, $4, $5)
ON CONFLICT DO NOTHING`,
		r.ValueDate, r.Asset, r.DVOLClose, r.SourceVersion, r.PayloadHash)
	return err
}

func GetDVOLCloseAsOf(ctx context.Context, tx pgx.Tx,
	asset string, valueDate, cutoff time.Time,
) (float64, bool, error) {
	var v float64
	err := tx.QueryRow(ctx, `
SELECT dvol_close FROM raw_deribit_dvol
WHERE asset = $1::asset_code AND value_date = $2 AND ingested_at <= $3
ORDER BY ingested_at DESC LIMIT 1`, asset, valueDate, cutoff).Scan(&v)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, err
	}
	return v, true, nil
}

func GetDVOLCloseSeries(ctx context.Context, tx pgx.Tx,
	asset string, from, to, cutoff time.Time,
) ([]float64, error) {
	rows, err := tx.Query(ctx, `
SELECT DISTINCT ON (value_date) dvol_close
FROM raw_deribit_dvol
WHERE asset = $1::asset_code
  AND value_date BETWEEN $2 AND $3
  AND ingested_at <= $4
ORDER BY value_date, ingested_at DESC`,
		asset, from, to, cutoff)
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

func RealizedVol30d(ctx context.Context, tx pgx.Tx,
	symbol string, valueDate, cutoff time.Time,
) (float64, bool, error) {

	from := valueDate.AddDate(0, 0, -30)
	closes, err := GetBinanceKlineCloseSeries(ctx, tx, symbol, from, valueDate, cutoff)
	if err != nil {
		return 0, false, err
	}
	if len(closes) < 20 {
		return 0, false, nil
	}

	logReturns := make([]float64, 0, len(closes)-1)
	for i := 1; i < len(closes); i++ {
		if closes[i-1] <= 0 {
			continue
		}
		logReturns = append(logReturns, math.Log(closes[i]/closes[i-1]))
	}
	if len(logReturns) < 10 {
		return 0, false, nil
	}

	mean := 0.0
	for _, r := range logReturns {
		mean += r
	}
	mean /= float64(len(logReturns))

	var sumSq float64
	for _, r := range logReturns {
		d := r - mean
		sumSq += d * d
	}
	std := math.Sqrt(sumSq / float64(len(logReturns)-1))

	rv := std * math.Sqrt(365) * 100
	return rv, true, nil
}

type VolatilityFeature struct {
	ValueDate         time.Time
	Asset             string
	Timeframe         string
	FeatureName       string
	FeatureVersion    string
	Value             float64
	SourceRawVersions map[string]string
	CodeSHA           string
}

func InsertVolatilityFeature(ctx context.Context, tx pgx.Tx, f VolatilityFeature) error {
	_, err := tx.Exec(ctx, `
INSERT INTO features_volatility
  (value_date, asset, timeframe, feature_name, feature_version,
   value, source_raw_versions, code_sha)
VALUES ($1, $2::asset_code, $3, $4, $5, $6, $7, $8)
ON CONFLICT DO NOTHING`,
		f.ValueDate, f.Asset, f.Timeframe, f.FeatureName, f.FeatureVersion,
		f.Value, f.SourceRawVersions, f.CodeSHA)
	return err
}

func GetLatestVolatilityFeature(ctx context.Context, tx pgx.Tx,
	featureName, featureVersion, asset string, valueDate, cutoff time.Time,
) (float64, error) {
	var v float64
	err := tx.QueryRow(ctx, `
SELECT value FROM features_volatility
WHERE feature_name = $1 AND feature_version = $2
  AND asset = $3::asset_code AND value_date = $4
  AND ingested_at <= $5
ORDER BY ingested_at DESC LIMIT 1`,
		featureName, featureVersion, asset, valueDate, cutoff).Scan(&v)
	return v, err
}

func GetVolatilityFeatureSeries(ctx context.Context, tx pgx.Tx,
	featureName, featureVersion, asset string, from, to, cutoff time.Time,
) ([]float64, error) {
	rows, err := tx.Query(ctx, `
SELECT DISTINCT ON (value_date) value
FROM features_volatility
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

type RawCoinglassOptionsInfo struct {
	ValueDate        time.Time
	Symbol           string
	Exchange         string
	OpenInterest     float64
	OIMarketShare    float64
	OIChange24h      float64
	OpenInterestUSD  float64
	VolumeUSD24h     float64
	VolumeChangePct  float64
	CallOpenInterest float64
	PutOpenInterest  float64
	SourceVersion    string
	PayloadHash      string
}

func InsertRawCoinglassOptionsInfo(ctx context.Context, tx pgx.Tx, r RawCoinglassOptionsInfo) error {
	_, err := tx.Exec(ctx, `
INSERT INTO raw_coinglass_options_info
  (value_date, symbol, exchange, open_interest, oi_market_share, oi_change_24h,
   open_interest_usd, volume_usd_24h, volume_change_pct,
   call_open_interest, put_open_interest, source_version, payload_hash)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
ON CONFLICT DO NOTHING`,
		r.ValueDate, r.Symbol, r.Exchange, r.OpenInterest, r.OIMarketShare, r.OIChange24h,
		r.OpenInterestUSD, r.VolumeUSD24h, r.VolumeChangePct,
		r.CallOpenInterest, r.PutOpenInterest, r.SourceVersion, r.PayloadHash)
	return err
}

func GetCoinglassOptionsPutCallRatioAsOf(ctx context.Context, tx pgx.Tx,
	symbol string, valueDate, cutoff time.Time,
) (float64, bool, error) {
	var callOI, putOI *float64
	err := tx.QueryRow(ctx, `
SELECT SUM(call_open_interest), SUM(put_open_interest) FROM (
  SELECT DISTINCT ON (exchange) exchange, call_open_interest, put_open_interest
  FROM raw_coinglass_options_info
  WHERE symbol = $1 AND value_date = $2 AND ingested_at <= $3
  ORDER BY exchange, ingested_at DESC
) t`, symbol, valueDate, cutoff).Scan(&callOI, &putOI)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, err
	}
	if callOI == nil || putOI == nil || *callOI <= 0 {
		return 0, false, nil
	}
	return *putOI / *callOI, true, nil
}

type RawCoinglassOptionsOIHistory struct {
	ValueDate     time.Time
	Symbol        string
	Exchange      string
	OpenInterest  float64
	SourceVersion string
	PayloadHash   string
}

func InsertRawCoinglassOptionsOIHistory(ctx context.Context, tx pgx.Tx, r RawCoinglassOptionsOIHistory) error {
	_, err := tx.Exec(ctx, `
INSERT INTO raw_coinglass_options_oi_history
  (value_date, symbol, exchange, open_interest, source_version, payload_hash)
VALUES ($1,$2,$3,$4,$5,$6)
ON CONFLICT DO NOTHING`,
		r.ValueDate, r.Symbol, r.Exchange, r.OpenInterest, r.SourceVersion, r.PayloadHash)
	return err
}

func GetCoinglassOptionsAggregatedOIAsOf(ctx context.Context, tx pgx.Tx,
	symbol string, valueDate, cutoff time.Time,
) (float64, bool, error) {
	var v *float64
	err := tx.QueryRow(ctx, `
SELECT SUM(open_interest) FROM (
  SELECT DISTINCT ON (exchange) exchange, open_interest
  FROM raw_coinglass_options_oi_history
  WHERE symbol = $1 AND value_date = $2 AND ingested_at <= $3
  ORDER BY exchange, ingested_at DESC
) t`, symbol, valueDate, cutoff).Scan(&v)
	if err != nil {
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

type RawCoinglassOptionsMaxPain struct {
	ValueDate         time.Time
	ExpiryDate        time.Time
	Symbol            string
	Exchange          string
	MaxPainPrice      float64
	CallOIContracts   float64
	PutOIContracts    float64
	CallOINotionalUSD float64
	PutOINotionalUSD  float64
	CallMarketValue   float64
	PutMarketValue    float64
	SourceVersion     string
	PayloadHash       string
}

func InsertRawCoinglassOptionsMaxPain(ctx context.Context, tx pgx.Tx, r RawCoinglassOptionsMaxPain) error {
	_, err := tx.Exec(ctx, `
INSERT INTO raw_coinglass_options_max_pain
  (value_date, expiry_date, symbol, exchange, max_pain_price,
   call_oi_contracts, put_oi_contracts, call_oi_notional_usd, put_oi_notional_usd,
   call_market_value, put_market_value, source_version, payload_hash)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
ON CONFLICT DO NOTHING`,
		r.ValueDate, r.ExpiryDate, r.Symbol, r.Exchange, r.MaxPainPrice,
		r.CallOIContracts, r.PutOIContracts, r.CallOINotionalUSD, r.PutOINotionalUSD,
		r.CallMarketValue, r.PutMarketValue, r.SourceVersion, r.PayloadHash)
	return err
}

func GetCoinglassOptionsMaxPainNearestAsOf(ctx context.Context, tx pgx.Tx,
	symbol, exchange string, valueDate, cutoff time.Time,
) (float64, bool, error) {
	var v float64
	err := tx.QueryRow(ctx, `
SELECT max_pain_price FROM raw_coinglass_options_max_pain
WHERE symbol = $1 AND exchange = $2 AND value_date = $3
  AND expiry_date >= $3 AND ingested_at <= $4
ORDER BY expiry_date ASC, ingested_at DESC LIMIT 1`,
		symbol, exchange, valueDate, cutoff).Scan(&v)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, err
	}
	return v, true, nil
}

func GetCoinglassOptionsDealerSkewProxyAsOf(ctx context.Context, tx pgx.Tx,
	symbol, exchange string, valueDate, cutoff time.Time,
) (float64, bool, error) {
	var callN, putN float64
	err := tx.QueryRow(ctx, `
SELECT call_oi_notional_usd, put_oi_notional_usd
FROM raw_coinglass_options_max_pain
WHERE symbol = $1 AND exchange = $2 AND value_date = $3
  AND expiry_date >= $3 AND ingested_at <= $4
ORDER BY expiry_date ASC, ingested_at DESC LIMIT 1`,
		symbol, exchange, valueDate, cutoff).Scan(&callN, &putN)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, err
	}
	denom := callN + putN
	if denom <= 0 {
		return 0, false, nil
	}
	return (callN - putN) / denom, true, nil
}

type RawDeribitOptionsChain struct {
	ValueDate          time.Time
	Asset              string
	InstrumentName     string
	ExpiryDate         time.Time
	StrikePrice        float64
	IsPut              bool
	OpenInterest       float64
	MarkIVPct          float64
	UnderlyingPriceUSD float64
	SourceVersion      string
	PayloadHash        string
}

func InsertRawDeribitOptionsChain(ctx context.Context, tx pgx.Tx, r RawDeribitOptionsChain) error {
	_, err := tx.Exec(ctx, `
INSERT INTO raw_deribit_options_chain
  (value_date, asset, instrument_name, expiry_date, strike_price, is_put,
   open_interest, mark_iv_pct, underlying_price_usd, source_version, payload_hash)
VALUES ($1,$2::asset_code,$3,$4,$5,$6,$7,$8,$9,$10,$11)
ON CONFLICT DO NOTHING`,
		r.ValueDate, r.Asset, r.InstrumentName, r.ExpiryDate, r.StrikePrice, r.IsPut,
		r.OpenInterest, r.MarkIVPct, r.UnderlyingPriceUSD, r.SourceVersion, r.PayloadHash)
	return err
}

type DeribitOptionsChainRow struct {
	ExpiryDate         time.Time
	StrikePrice        float64
	IsPut              bool
	OpenInterest       float64
	MarkIVPct          float64
	UnderlyingPriceUSD float64
}

func GetDeribitOptionsChainAsOf(ctx context.Context, tx pgx.Tx,
	asset string, valueDate, cutoff time.Time,
) ([]DeribitOptionsChainRow, error) {
	rows, err := tx.Query(ctx, `
SELECT DISTINCT ON (instrument_name)
  expiry_date, strike_price, is_put, open_interest, mark_iv_pct, underlying_price_usd
FROM raw_deribit_options_chain
WHERE asset = $1::asset_code AND value_date = $2 AND ingested_at <= $3
ORDER BY instrument_name, ingested_at DESC`, asset, valueDate, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DeribitOptionsChainRow
	for rows.Next() {
		var r DeribitOptionsChainRow
		if err := rows.Scan(&r.ExpiryDate, &r.StrikePrice, &r.IsPut, &r.OpenInterest, &r.MarkIVPct, &r.UnderlyingPriceUSD); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
