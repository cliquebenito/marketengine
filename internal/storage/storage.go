package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Pool struct {
	*pgxpool.Pool
}

func Open(ctx context.Context, url string) (*Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse db url: %w", err)
	}

	cfg.ConnConfig.RuntimeParams["search_path"] = "regime,public"
	p, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("new pool: %w", err)
	}
	if err := p.Ping(ctx); err != nil {
		p.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Pool{Pool: p}, nil
}

func (p *Pool) InTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := p.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {

		_ = tx.Rollback(ctx)
	}()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type RawStablecoinSupply struct {
	ValueDate     time.Time
	Stablecoin    string
	Metric        string
	Value         float64
	SourceVersion string
	PayloadHash   string
}

type LiquidityFeature struct {
	ValueDate         time.Time
	Asset             string
	Timeframe         string
	FeatureName       string
	FeatureVersion    string
	Value             float64
	SourceRawVersions map[string]string
	CodeSHA           string
}

type DomainScore struct {
	Asset             string
	Domain            string
	ValueDate         time.Time
	Score             float64
	Components        map[string]float64
	FeatureCodesUsed  []string
	ModelVersion      string
	ConfigVersion     string
	CodeSHA           string
	SourceRawVersions map[string]string
	DataQuality       map[string]any
}

func InsertRawStablecoinSupply(ctx context.Context, tx pgx.Tx, r RawStablecoinSupply) error {
	_, err := tx.Exec(ctx, `
INSERT INTO raw_defillama_stablecoin_supply
  (value_date, stablecoin, metric, value, source_version, payload_hash)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (value_date, stablecoin, metric, payload_hash) DO NOTHING`,
		r.ValueDate, r.Stablecoin, r.Metric, r.Value, r.SourceVersion, r.PayloadHash)
	return err
}

func GetRawStablecoinSupplyAsOf(ctx context.Context, tx pgx.Tx,
	stablecoin string, valueDate time.Time, cutoff time.Time,
) (float64, error) {
	var v float64
	err := tx.QueryRow(ctx, `
SELECT value FROM raw_defillama_stablecoin_supply
WHERE value_date = $1 AND stablecoin = $2 AND metric = 'circulating_supply_usd'
  AND ingested_at <= $3
ORDER BY ingested_at DESC
LIMIT 1`, valueDate, stablecoin, cutoff).Scan(&v)
	if err != nil {
		return 0, err
	}
	return v, nil
}

func SumRawStablecoinSupplyAsOf(ctx context.Context, tx pgx.Tx,
	symbols []string, valueDate, cutoff time.Time,
) (float64, int, error) {
	var total float64
	found := 0
	for _, sym := range symbols {
		v, err := GetRawStablecoinSupplyAsOf(ctx, tx, sym, valueDate, cutoff)
		if err != nil {
			if err == pgx.ErrNoRows {
				continue
			}
			return 0, 0, fmt.Errorf("sum stablecoin %s: %w", sym, err)
		}
		total += v
		found++
	}
	return total, found, nil
}

type RawChainTVL struct {
	ValueDate     time.Time
	Chain         string
	TVLUSD        float64
	SourceVersion string
	PayloadHash   string
}

func InsertRawChainTVL(ctx context.Context, tx pgx.Tx, r RawChainTVL) error {
	_, err := tx.Exec(ctx, `
INSERT INTO raw_defillama_tvl
  (value_date, chain, tvl_usd, source_version, payload_hash)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (value_date, chain, payload_hash) DO NOTHING`,
		r.ValueDate, r.Chain, r.TVLUSD, r.SourceVersion, r.PayloadHash)
	return err
}

func GetRawChainTVLAsOf(ctx context.Context, tx pgx.Tx,
	chain string, valueDate, cutoff time.Time,
) (float64, error) {
	var v float64
	err := tx.QueryRow(ctx, `
SELECT tvl_usd FROM raw_defillama_tvl
WHERE chain = $1 AND value_date = $2
  AND ingested_at <= $3
ORDER BY ingested_at DESC
LIMIT 1`, chain, valueDate, cutoff).Scan(&v)
	return v, err
}

type RawExchangeNetflow struct {
	ValueDate     time.Time
	Asset         string
	InflowUSD     *float64
	OutflowUSD    *float64
	NetflowUSD    *float64
	SourceVersion string
	PayloadHash   string
}

func InsertRawExchangeNetflow(ctx context.Context, tx pgx.Tx, r RawExchangeNetflow) error {
	_, err := tx.Exec(ctx, `
INSERT INTO raw_coinmetrics_exchange_netflow
  (value_date, asset, inflow_usd, outflow_usd, netflow_usd, source_version, payload_hash)
VALUES ($1, $2::asset_code, $3, $4, $5, $6, $7)
ON CONFLICT (value_date, asset, payload_hash) DO NOTHING`,
		r.ValueDate, r.Asset, r.InflowUSD, r.OutflowUSD, r.NetflowUSD, r.SourceVersion, r.PayloadHash)
	return err
}

func Sum7dNetflow(ctx context.Context, tx pgx.Tx, asset string, valueDate, cutoff time.Time) (float64, bool, error) {
	from := valueDate.AddDate(0, 0, -6)
	rows, err := tx.Query(ctx, `
SELECT DISTINCT ON (value_date) netflow_usd
FROM raw_coinmetrics_exchange_netflow
WHERE asset = $1::asset_code
  AND value_date BETWEEN $2 AND $3
  AND ingested_at <= $4
ORDER BY value_date, ingested_at DESC`,
		asset, from, valueDate, cutoff)
	if err != nil {
		return 0, false, err
	}
	defer rows.Close()
	var sum float64
	n := 0
	for rows.Next() {
		var v *float64
		if err := rows.Scan(&v); err != nil {
			return 0, false, err
		}
		if v != nil {
			sum += *v
			n++
		}
	}
	if err := rows.Err(); err != nil {
		return 0, false, err
	}
	if n < 7 {
		return 0, false, nil
	}
	return sum, true, nil
}

func InsertLiquidityFeature(ctx context.Context, tx pgx.Tx, f LiquidityFeature) error {
	_, err := tx.Exec(ctx, `
INSERT INTO features_liquidity
  (value_date, asset, timeframe, feature_name, feature_version,
   value, source_raw_versions, code_sha)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		f.ValueDate, f.Asset, f.Timeframe, f.FeatureName, f.FeatureVersion,
		f.Value, f.SourceRawVersions, f.CodeSHA)
	return err
}

func GetLatestLiquidityFeature(ctx context.Context, tx pgx.Tx,
	featureName, featureVersion, asset string, valueDate, cutoff time.Time,
) (float64, error) {
	var v float64
	err := tx.QueryRow(ctx, `
SELECT value FROM features_liquidity
WHERE feature_name = $1 AND feature_version = $2
  AND asset = $3::asset_code AND value_date = $4
  AND ingested_at <= $5
ORDER BY ingested_at DESC
LIMIT 1`, featureName, featureVersion, asset, valueDate, cutoff).Scan(&v)
	if err != nil {
		return 0, err
	}
	return v, nil
}

func GetLiquidityFeatureSeries(ctx context.Context, tx pgx.Tx,
	featureName, featureVersion, asset string, from, to, cutoff time.Time,
) ([]float64, error) {
	rows, err := tx.Query(ctx, `
SELECT DISTINCT ON (value_date) value
FROM features_liquidity
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

func InsertDomainScore(ctx context.Context, tx pgx.Tx, s DomainScore) error {
	_, err := tx.Exec(ctx, `
INSERT INTO domain_scores
  (asset, domain, value_date, score, components, feature_codes_used,
   model_version, config_version, code_sha, source_raw_versions, data_quality)
VALUES ($1::asset_code, $2::domain_code, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		s.Asset, s.Domain, s.ValueDate, s.Score, s.Components, s.FeatureCodesUsed,
		s.ModelVersion, s.ConfigVersion, s.CodeSHA, s.SourceRawVersions, s.DataQuality)
	return err
}

type RawMarketCap struct {
	ValueDate     time.Time
	CoinID        string
	MarketCapUSD  float64
	PriceUSD      float64
	SourceVersion string
	PayloadHash   string
}

func InsertRawMarketCap(ctx context.Context, tx pgx.Tx, r RawMarketCap) error {
	_, err := tx.Exec(ctx, `
INSERT INTO raw_coingecko_market_cap
  (value_date, coin_id, market_cap_usd, price_usd, source_version, payload_hash)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (value_date, coin_id, payload_hash) DO NOTHING`,
		r.ValueDate, r.CoinID, r.MarketCapUSD, r.PriceUSD, r.SourceVersion, r.PayloadHash)
	return err
}

func GetMarketCapAsOf(ctx context.Context, tx pgx.Tx,
	coinID string, valueDate, cutoff time.Time,
) (float64, error) {
	var v float64
	err := tx.QueryRow(ctx, `
SELECT market_cap_usd FROM raw_coingecko_market_cap
WHERE coin_id = $1 AND value_date = $2
  AND ingested_at <= $3
ORDER BY ingested_at DESC
LIMIT 1`, coinID, valueDate, cutoff).Scan(&v)
	if err != nil {
		return 0, err
	}
	return v, nil
}

func UpsertModelConfig(ctx context.Context, tx pgx.Tx,
	configVersion, scope, modelVersion string, yamlBody []byte, yamlParsed map[string]any,
) error {
	_, err := tx.Exec(ctx, `
INSERT INTO model_configs (config_version, scope, model_version, yaml_body, yaml_parsed)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (config_version) DO NOTHING`,
		configVersion, scope, modelVersion, string(yamlBody), yamlParsed)
	return err
}
