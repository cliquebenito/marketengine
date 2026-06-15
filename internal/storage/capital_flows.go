package storage

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

type RawCoinglassETFFlow struct {
	ValueDate     time.Time
	FlowType      string
	TotalFlowUSD  float64
	PriceUSD      float64
	SourceVersion string
	PayloadHash   string
}

func InsertRawCoinglassETFFlow(ctx context.Context, tx pgx.Tx, r RawCoinglassETFFlow) error {
	_, err := tx.Exec(ctx, `
INSERT INTO raw_coinglass_etf_flows
  (value_date, flow_type, total_flow_usd, price_usd, source_version, payload_hash)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT DO NOTHING`,
		r.ValueDate, r.FlowType, r.TotalFlowUSD, r.PriceUSD, r.SourceVersion, r.PayloadHash)
	return err
}

func CombinedETFFlowAsOf(ctx context.Context, tx pgx.Tx,
	valueDate, cutoff time.Time,
) (float64, bool, error) {
	rows, err := tx.Query(ctx, `
SELECT DISTINCT ON (flow_type) total_flow_usd
FROM raw_coinglass_etf_flows
WHERE value_date = $1 AND ingested_at <= $2
ORDER BY flow_type, ingested_at DESC`,
		valueDate, cutoff)
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

type RawGlassnodeLTHSupply struct {
	ValueDate     time.Time
	Asset         string
	LTHSupply     float64
	SourceVersion string
	PayloadHash   string
}

func InsertRawGlassnodeLTHSupply(ctx context.Context, tx pgx.Tx, r RawGlassnodeLTHSupply) error {
	_, err := tx.Exec(ctx, `
INSERT INTO raw_glassnode_lth_supply
  (value_date, asset, lth_supply, source_version, payload_hash)
VALUES ($1, $2::asset_code, $3, $4, $5)
ON CONFLICT DO NOTHING`,
		r.ValueDate, r.Asset, r.LTHSupply, r.SourceVersion, r.PayloadHash)
	return err
}

func GetLTHSupplyAsOf(ctx context.Context, tx pgx.Tx,
	asset string, valueDate, cutoff time.Time,
) (float64, error) {
	var v float64
	err := tx.QueryRow(ctx, `
SELECT lth_supply FROM raw_glassnode_lth_supply
WHERE asset = $1::asset_code AND value_date = $2
  AND ingested_at <= $3
ORDER BY ingested_at DESC
LIMIT 1`, asset, valueDate, cutoff).Scan(&v)
	return v, err
}

func GetLTHSupplySeries(ctx context.Context, tx pgx.Tx,
	asset string, from, to, cutoff time.Time,
) ([]float64, error) {
	rows, err := tx.Query(ctx, `
SELECT DISTINCT ON (value_date) lth_supply
FROM raw_glassnode_lth_supply
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

type RawGlassnodeMinerFlow struct {
	ValueDate     time.Time
	Asset         string
	MinerFlowUSD  float64
	SourceVersion string
	PayloadHash   string
}

func InsertRawGlassnodeMinerFlow(ctx context.Context, tx pgx.Tx, r RawGlassnodeMinerFlow) error {
	_, err := tx.Exec(ctx, `
INSERT INTO raw_glassnode_miner_flow
  (value_date, asset, miner_flow_usd, source_version, payload_hash)
VALUES ($1, $2::asset_code, $3, $4, $5)
ON CONFLICT DO NOTHING`,
		r.ValueDate, r.Asset, r.MinerFlowUSD, r.SourceVersion, r.PayloadHash)
	return err
}

type CapitalFlowsFeature struct {
	ValueDate         time.Time
	Asset             string
	Timeframe         string
	FeatureName       string
	FeatureVersion    string
	Value             float64
	SourceRawVersions map[string]string
	CodeSHA           string
}

func InsertCapitalFlowsFeature(ctx context.Context, tx pgx.Tx, f CapitalFlowsFeature) error {
	_, err := tx.Exec(ctx, `
INSERT INTO features_capital_flows
  (value_date, asset, timeframe, feature_name, feature_version,
   value, source_raw_versions, code_sha)
VALUES ($1, $2::asset_code, $3, $4, $5, $6, $7, $8)
ON CONFLICT DO NOTHING`,
		f.ValueDate, f.Asset, f.Timeframe, f.FeatureName, f.FeatureVersion,
		f.Value, f.SourceRawVersions, f.CodeSHA)
	return err
}

func GetLatestCapitalFlowsFeature(ctx context.Context, tx pgx.Tx,
	featureName, featureVersion, asset string, valueDate, cutoff time.Time,
) (float64, error) {
	var v float64
	err := tx.QueryRow(ctx, `
SELECT value FROM features_capital_flows
WHERE feature_name = $1 AND feature_version = $2
  AND asset = $3::asset_code AND value_date = $4
  AND ingested_at <= $5
ORDER BY ingested_at DESC LIMIT 1`,
		featureName, featureVersion, asset, valueDate, cutoff).Scan(&v)
	return v, err
}

func GetCapitalFlowsFeatureSeries(ctx context.Context, tx pgx.Tx,
	featureName, featureVersion, asset string, from, to, cutoff time.Time,
) ([]float64, error) {
	rows, err := tx.Query(ctx, `
SELECT DISTINCT ON (value_date) value
FROM features_capital_flows
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

type RawCoinglassStablecoinMcap struct {
	ValueDate     time.Time
	MarketCap     float64
	PriceUSD      float64
	SourceVersion string
	PayloadHash   string
}

func InsertRawCoinglassStablecoinMcap(ctx context.Context, tx pgx.Tx, r RawCoinglassStablecoinMcap) error {
	_, err := tx.Exec(ctx, `
INSERT INTO raw_coinglass_stablecoin_mcap
  (value_date, market_cap, price_usd, source_version, payload_hash)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT DO NOTHING`,
		r.ValueDate, r.MarketCap, r.PriceUSD, r.SourceVersion, r.PayloadHash)
	return err
}

func GetCoinglassStablecoinMcapAsOf(ctx context.Context, tx pgx.Tx, valueDate, cutoff time.Time) (float64, bool, error) {
	var v float64
	err := tx.QueryRow(ctx, `
SELECT market_cap FROM raw_coinglass_stablecoin_mcap
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

func GetCoinglassStablecoinMcapSeries(ctx context.Context, tx pgx.Tx, from, to, cutoff time.Time) ([]float64, error) {
	rows, err := tx.Query(ctx, `
SELECT DISTINCT ON (value_date) market_cap
FROM raw_coinglass_stablecoin_mcap
WHERE value_date BETWEEN $1 AND $2 AND ingested_at <= $3
ORDER BY value_date, ingested_at DESC`, from, to, cutoff)
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

type RawCoinglassExchangeBalance struct {
	ValueDate           time.Time
	Symbol              string
	Exchange            string
	TotalBalance        float64
	BalanceChange1d     float64
	BalanceChange7d     float64
	BalanceChange30d    float64
	BalanceChangePct1d  float64
	BalanceChangePct7d  float64
	BalanceChangePct30d float64
	SourceVersion       string
	PayloadHash         string
}

func InsertRawCoinglassExchangeBalance(ctx context.Context, tx pgx.Tx, r RawCoinglassExchangeBalance) error {
	_, err := tx.Exec(ctx, `
INSERT INTO raw_coinglass_exchange_balance
  (value_date, symbol, exchange, total_balance,
   balance_change_1d, balance_change_7d, balance_change_30d,
   balance_change_pct_1d, balance_change_pct_7d, balance_change_pct_30d,
   source_version, payload_hash)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
ON CONFLICT DO NOTHING`,
		r.ValueDate, r.Symbol, r.Exchange, r.TotalBalance,
		r.BalanceChange1d, r.BalanceChange7d, r.BalanceChange30d,
		r.BalanceChangePct1d, r.BalanceChangePct7d, r.BalanceChangePct30d,
		r.SourceVersion, r.PayloadHash)
	return err
}

func GetCoinglassExchangeBalanceChange7dSumAsOf(ctx context.Context, tx pgx.Tx,
	symbol string, valueDate, cutoff time.Time,
) (float64, bool, error) {
	return getExchangeBalanceChangeSumAsOf(ctx, tx, symbol, valueDate, cutoff, "balance_change_7d")
}

func GetCoinglassExchangeBalanceChange30dSumAsOf(ctx context.Context, tx pgx.Tx,
	symbol string, valueDate, cutoff time.Time,
) (float64, bool, error) {
	return getExchangeBalanceChangeSumAsOf(ctx, tx, symbol, valueDate, cutoff, "balance_change_30d")
}

func getExchangeBalanceChangeSumAsOf(ctx context.Context, tx pgx.Tx,
	symbol string, valueDate, cutoff time.Time, column string,
) (float64, bool, error) {
	q := `
SELECT SUM(` + column + `) FROM (
  SELECT DISTINCT ON (exchange) exchange, ` + column + `
  FROM raw_coinglass_exchange_balance
  WHERE symbol = $1 AND value_date = $2 AND ingested_at <= $3
  ORDER BY exchange, ingested_at DESC
) t`
	var v *float64
	err := tx.QueryRow(ctx, q, symbol, valueDate, cutoff).Scan(&v)
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

type RawCoinglassBitfinexMargin struct {
	ValueDate     time.Time
	Symbol        string
	LongQty       float64
	ShortQty      float64
	SourceVersion string
	PayloadHash   string
}

func InsertRawCoinglassBitfinexMargin(ctx context.Context, tx pgx.Tx, r RawCoinglassBitfinexMargin) error {
	_, err := tx.Exec(ctx, `
INSERT INTO raw_coinglass_bitfinex_margin
  (value_date, symbol, long_qty, short_qty, source_version, payload_hash)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT DO NOTHING`,
		r.ValueDate, r.Symbol, r.LongQty, r.ShortQty, r.SourceVersion, r.PayloadHash)
	return err
}

func GetCoinglassBitfinexMarginAsOf(ctx context.Context, tx pgx.Tx,
	symbol string, valueDate, cutoff time.Time,
) (long, short float64, ok bool, err error) {
	err = tx.QueryRow(ctx, `
SELECT long_qty, short_qty FROM raw_coinglass_bitfinex_margin
WHERE symbol = $1 AND value_date = $2 AND ingested_at <= $3
ORDER BY ingested_at DESC LIMIT 1`, symbol, valueDate, cutoff).Scan(&long, &short)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, 0, false, nil
		}
		return 0, 0, false, err
	}
	return long, short, true, nil
}

type RawCoinglassETFListItem struct {
	ValueDate          time.Time
	Ticker             string
	FundName           string
	Region             string
	MarketStatus       string
	PrimaryExchange    string
	FundType           string
	SharesOutstanding  float64
	AUMUSD             float64
	ManagementFeePct   float64
	VolumeUSD          float64
	PriceChangePct     float64
	NetAssetValueUSD   float64
	PremiumDiscountPct float64
	HoldingQuantity    float64
	ChangePct24h       float64
	ChangeQty24h       float64
	ChangePct7d        float64
	ChangeQty7d        float64
	SourceVersion      string
	PayloadHash        string
}

func InsertRawCoinglassETFListItem(ctx context.Context, tx pgx.Tx, r RawCoinglassETFListItem) error {
	_, err := tx.Exec(ctx, `
INSERT INTO raw_coinglass_etf_list
  (value_date, ticker, fund_name, region, market_status, primary_exchange, fund_type,
   shares_outstanding, aum_usd, management_fee_pct, volume_usd, price_change_pct,
   net_asset_value_usd, premium_discount_pct, holding_quantity,
   change_pct_24h, change_qty_24h, change_pct_7d, change_qty_7d,
   source_version, payload_hash)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)
ON CONFLICT DO NOTHING`,
		r.ValueDate, r.Ticker, r.FundName, r.Region, r.MarketStatus, r.PrimaryExchange, r.FundType,
		r.SharesOutstanding, r.AUMUSD, r.ManagementFeePct, r.VolumeUSD, r.PriceChangePct,
		r.NetAssetValueUSD, r.PremiumDiscountPct, r.HoldingQuantity,
		r.ChangePct24h, r.ChangeQty24h, r.ChangePct7d, r.ChangeQty7d,
		r.SourceVersion, r.PayloadHash)
	return err
}

func GetCoinglassETFListAUMTotalAsOf(ctx context.Context, tx pgx.Tx, valueDate, cutoff time.Time) (float64, bool, error) {
	var v *float64
	err := tx.QueryRow(ctx, `
SELECT SUM(aum_usd) FROM (
  SELECT DISTINCT ON (ticker) ticker, aum_usd
  FROM raw_coinglass_etf_list
  WHERE value_date = $1 AND ingested_at <= $2
  ORDER BY ticker, ingested_at DESC
) t`, valueDate, cutoff).Scan(&v)
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

func GetCoinglassETFListConcentrationHHIAsOf(ctx context.Context, tx pgx.Tx, valueDate, cutoff time.Time) (float64, bool, error) {
	rows, err := tx.Query(ctx, `
SELECT DISTINCT ON (ticker) ticker, aum_usd
FROM raw_coinglass_etf_list
WHERE value_date = $1 AND ingested_at <= $2
ORDER BY ticker, ingested_at DESC`, valueDate, cutoff)
	if err != nil {
		return 0, false, err
	}
	defer rows.Close()
	var aums []float64
	var total float64
	for rows.Next() {
		var ticker string
		var a float64
		if err := rows.Scan(&ticker, &a); err != nil {
			return 0, false, err
		}
		if a > 0 {
			aums = append(aums, a)
			total += a
		}
	}
	if err := rows.Err(); err != nil {
		return 0, false, err
	}
	if total <= 0 || len(aums) == 0 {
		return 0, false, nil
	}
	var hhi float64
	for _, a := range aums {
		s := a / total
		hhi += s * s
	}
	return hhi, true, nil
}

type RawCoinglassETFAUMHistory struct {
	ValueDate     time.Time
	Ticker        string
	AUMUSD        float64
	SourceVersion string
	PayloadHash   string
}

func InsertRawCoinglassETFAUMHistory(ctx context.Context, tx pgx.Tx, r RawCoinglassETFAUMHistory) error {
	_, err := tx.Exec(ctx, `
INSERT INTO raw_coinglass_etf_aum_history
  (value_date, ticker, aum_usd, source_version, payload_hash)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT DO NOTHING`,
		r.ValueDate, r.Ticker, r.AUMUSD, r.SourceVersion, r.PayloadHash)
	return err
}

func GetCoinglassETFAUMHistoryTotalAsOf(ctx context.Context, tx pgx.Tx, valueDate, cutoff time.Time) (float64, bool, error) {
	var v *float64
	err := tx.QueryRow(ctx, `
SELECT SUM(aum_usd) FROM (
  SELECT DISTINCT ON (ticker) ticker, aum_usd
  FROM raw_coinglass_etf_aum_history
  WHERE value_date = $1 AND ingested_at <= $2
  ORDER BY ticker, ingested_at DESC
) t`, valueDate, cutoff).Scan(&v)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, err
	}
	if v == nil || *v <= 0 {
		return 0, false, nil
	}
	return *v, true, nil
}
