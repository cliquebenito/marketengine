package storage

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

type RawKrakenOHLC struct {
	ValueDate     time.Time
	Pair          string
	Open          float64
	High          float64
	Low           float64
	Close         float64
	Volume        float64
	SourceVersion string
	PayloadHash   string
}

func InsertRawKrakenOHLC(ctx context.Context, tx pgx.Tx, r RawKrakenOHLC) error {
	_, err := tx.Exec(ctx, `
INSERT INTO raw_kraken_ohlc (value_date, pair, open, high, low, close, volume, source_version, payload_hash)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT DO NOTHING`,
		r.ValueDate, r.Pair, r.Open, r.High, r.Low, r.Close, r.Volume, r.SourceVersion, r.PayloadHash)
	return err
}

func GetKrakenCloseAsOf(ctx context.Context, tx pgx.Tx,
	pair string, valueDate, cutoff time.Time,
) (float64, bool, error) {
	var v float64
	err := tx.QueryRow(ctx, `
SELECT close FROM raw_kraken_ohlc
WHERE pair = $1 AND value_date = $2 AND ingested_at <= $3
ORDER BY ingested_at DESC LIMIT 1`, pair, valueDate, cutoff).Scan(&v)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, err
	}
	return v, true, nil
}

type RawCoinbaseCandle struct {
	ValueDate     time.Time
	ProductID     string
	Close         float64
	Volume        float64
	SourceVersion string
	PayloadHash   string
}

func InsertRawCoinbaseCandle(ctx context.Context, tx pgx.Tx, r RawCoinbaseCandle) error {
	_, err := tx.Exec(ctx, `
INSERT INTO raw_coinbase_candles (value_date, product_id, close, volume, source_version, payload_hash)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT DO NOTHING`,
		r.ValueDate, r.ProductID, r.Close, r.Volume, r.SourceVersion, r.PayloadHash)
	return err
}

func GetCoinbaseCloseAsOf(ctx context.Context, tx pgx.Tx,
	productID string, valueDate, cutoff time.Time,
) (float64, bool, error) {
	var v float64
	err := tx.QueryRow(ctx, `
SELECT close FROM raw_coinbase_candles
WHERE product_id = $1 AND value_date = $2 AND ingested_at <= $3
ORDER BY ingested_at DESC LIMIT 1`, productID, valueDate, cutoff).Scan(&v)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, err
	}
	return v, true, nil
}

type RawBinanceKline struct {
	ValueDate     time.Time
	Symbol        string
	Close         float64
	Volume        float64
	SourceVersion string
	PayloadHash   string
}

func InsertRawBinanceKline(ctx context.Context, tx pgx.Tx, r RawBinanceKline) error {
	_, err := tx.Exec(ctx, `
INSERT INTO raw_binance_klines (value_date, symbol, close, volume, source_version, payload_hash)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT DO NOTHING`,
		r.ValueDate, r.Symbol, r.Close, r.Volume, r.SourceVersion, r.PayloadHash)
	return err
}

func GetBinanceKlineCloseAsOf(ctx context.Context, tx pgx.Tx,
	symbol string, valueDate, cutoff time.Time,
) (float64, bool, error) {
	var v float64
	err := tx.QueryRow(ctx, `
SELECT close FROM raw_binance_klines
WHERE symbol = $1 AND value_date = $2 AND ingested_at <= $3
ORDER BY ingested_at DESC LIMIT 1`, symbol, valueDate, cutoff).Scan(&v)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, err
	}
	return v, true, nil
}

func GetBinanceKlineCloseSeries(ctx context.Context, tx pgx.Tx,
	symbol string, from, to, cutoff time.Time,
) ([]float64, error) {
	rows, err := tx.Query(ctx, `
SELECT DISTINCT ON (value_date) close
FROM raw_binance_klines
WHERE symbol = $1
  AND value_date BETWEEN $2 AND $3
  AND ingested_at <= $4
ORDER BY value_date, ingested_at DESC`,
		symbol, from, to, cutoff)
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

type RawCoinglassCoinbasePremium struct {
	ValueDate     time.Time
	PremiumUSD    float64
	PremiumRate   float64
	CoinbasePrice float64
	SourceVersion string
	PayloadHash   string
}

func InsertRawCoinglassCoinbasePremium(ctx context.Context, tx pgx.Tx, r RawCoinglassCoinbasePremium) error {
	_, err := tx.Exec(ctx, `
INSERT INTO raw_coinglass_coinbase_premium (value_date, premium_usd, premium_rate, coinbase_price, source_version, payload_hash)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT DO NOTHING`,
		r.ValueDate, r.PremiumUSD, r.PremiumRate, r.CoinbasePrice, r.SourceVersion, r.PayloadHash)
	return err
}

func GetCoinglassCoinbasePremiumRateAsOf(ctx context.Context, tx pgx.Tx,
	valueDate, cutoff time.Time,
) (float64, bool, error) {
	var v float64
	err := tx.QueryRow(ctx, `
SELECT premium_rate FROM raw_coinglass_coinbase_premium
WHERE value_date = $1 AND ingested_at <= $2
ORDER BY ingested_at DESC LIMIT 1`, valueDate, cutoff).Scan(&v)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, err
	}
	return v, true, nil
}

type MarketStressFeature struct {
	ValueDate         time.Time
	Asset             string
	Timeframe         string
	FeatureName       string
	FeatureVersion    string
	Value             float64
	SourceRawVersions map[string]string
	CodeSHA           string
}

func InsertMarketStressFeature(ctx context.Context, tx pgx.Tx, f MarketStressFeature) error {
	_, err := tx.Exec(ctx, `
INSERT INTO features_market_stress
  (value_date, asset, timeframe, feature_name, feature_version,
   value, source_raw_versions, code_sha)
VALUES ($1, $2::asset_code, $3, $4, $5, $6, $7, $8)
ON CONFLICT DO NOTHING`,
		f.ValueDate, f.Asset, f.Timeframe, f.FeatureName, f.FeatureVersion,
		f.Value, f.SourceRawVersions, f.CodeSHA)
	return err
}

func GetLatestMarketStressFeature(ctx context.Context, tx pgx.Tx,
	featureName, featureVersion, asset string, valueDate, cutoff time.Time,
) (float64, error) {
	var v float64
	err := tx.QueryRow(ctx, `
SELECT value FROM features_market_stress
WHERE feature_name = $1 AND feature_version = $2
  AND asset = $3::asset_code AND value_date = $4
  AND ingested_at <= $5
ORDER BY ingested_at DESC LIMIT 1`,
		featureName, featureVersion, asset, valueDate, cutoff).Scan(&v)
	return v, err
}

func GetMarketStressFeatureSeries(ctx context.Context, tx pgx.Tx,
	featureName, featureVersion, asset string, from, to, cutoff time.Time,
) ([]float64, error) {
	rows, err := tx.Query(ctx, `
SELECT DISTINCT ON (value_date) value
FROM features_market_stress
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

type RawCoinglassOrderbookAggregated struct {
	ValueDate     time.Time
	Symbol        string
	RangePct      string
	BidsUSD       float64
	BidsQty       float64
	AsksUSD       float64
	AsksQty       float64
	SourceVersion string
	PayloadHash   string
}

func InsertRawCoinglassOrderbookAggregated(ctx context.Context, tx pgx.Tx, r RawCoinglassOrderbookAggregated) error {
	if r.RangePct == "" {
		r.RangePct = "1"
	}
	_, err := tx.Exec(ctx, `
INSERT INTO raw_coinglass_orderbook_aggregated
  (value_date, symbol, range_pct, bids_usd, bids_qty, asks_usd, asks_qty, source_version, payload_hash)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
ON CONFLICT DO NOTHING`,
		r.ValueDate, r.Symbol, r.RangePct, r.BidsUSD, r.BidsQty, r.AsksUSD, r.AsksQty, r.SourceVersion, r.PayloadHash)
	return err
}

func GetCoinglassOrderbookImbalanceAsOf(ctx context.Context, tx pgx.Tx,
	symbol string, valueDate, cutoff time.Time,
) (float64, bool, error) {
	var bids, asks float64
	err := tx.QueryRow(ctx, `
SELECT bids_usd, asks_usd FROM raw_coinglass_orderbook_aggregated
WHERE symbol = $1 AND range_pct = '1' AND value_date = $2 AND ingested_at <= $3
ORDER BY ingested_at DESC LIMIT 1`, symbol, valueDate, cutoff).Scan(&bids, &asks)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, err
	}
	denom := bids + asks
	if denom <= 0 {
		return 0, false, nil
	}
	return (bids - asks) / denom, true, nil
}

type RawCoinglassFuturesSpotVolRatio struct {
	ValueDate        time.Time
	Symbol           string
	FuturesSpotRatio float64
	FuturesVolUSD    float64
	SpotVolUSD       float64
	SourceVersion    string
	PayloadHash      string
}

func InsertRawCoinglassFuturesSpotVolRatio(ctx context.Context, tx pgx.Tx, r RawCoinglassFuturesSpotVolRatio) error {
	_, err := tx.Exec(ctx, `
INSERT INTO raw_coinglass_futures_spot_vol_ratio
  (value_date, symbol, futures_spot_ratio, futures_vol_usd, spot_vol_usd, source_version, payload_hash)
VALUES ($1,$2,$3,$4,$5,$6,$7)
ON CONFLICT DO NOTHING`,
		r.ValueDate, r.Symbol, r.FuturesSpotRatio, r.FuturesVolUSD, r.SpotVolUSD, r.SourceVersion, r.PayloadHash)
	return err
}

func GetCoinglassFuturesSpotRatioAsOf(ctx context.Context, tx pgx.Tx,
	symbol string, valueDate, cutoff time.Time,
) (float64, bool, error) {
	var v float64
	err := tx.QueryRow(ctx, `
SELECT futures_spot_ratio FROM raw_coinglass_futures_spot_vol_ratio
WHERE symbol = $1 AND value_date = $2 AND ingested_at <= $3
ORDER BY ingested_at DESC LIMIT 1`, symbol, valueDate, cutoff).Scan(&v)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, err
	}
	return v, true, nil
}
