-- +goose Up
-- Raw tables for the Market Stress domain.
-- Sources: Kraken (stablecoin peg), Coinbase (premium), Binance spot klines (correlation).

-- Kraken OHLC — USDTUSD, USDCUSD fiat pairs for peg deviation monitoring.
CREATE TABLE regime.raw_kraken_ohlc (
  value_date      TIMESTAMPTZ NOT NULL,
  ingested_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  pair            TEXT NOT NULL,           -- 'USDTUSD', 'USDCUSD'
  open            DOUBLE PRECISION NOT NULL,
  high            DOUBLE PRECISION NOT NULL,
  low             DOUBLE PRECISION NOT NULL,
  close           DOUBLE PRECISION NOT NULL,
  volume          DOUBLE PRECISION NOT NULL,
  source_version  TEXT NOT NULL DEFAULT 'kraken_v1',
  payload_hash    TEXT NOT NULL,
  PRIMARY KEY (value_date, pair, ingested_at)
);
SELECT create_hypertable('regime.raw_kraken_ohlc', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');
CREATE UNIQUE INDEX raw_kraken_ohlc_dedupe
  ON regime.raw_kraken_ohlc (value_date, pair, payload_hash);
CREATE INDEX ON regime.raw_kraken_ohlc (pair, value_date DESC, ingested_at DESC);

-- Coinbase candles — BTC-USD for Coinbase Premium computation.
CREATE TABLE regime.raw_coinbase_candles (
  value_date      TIMESTAMPTZ NOT NULL,
  ingested_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  product_id      TEXT NOT NULL,           -- 'BTC-USD'
  close           DOUBLE PRECISION NOT NULL,
  volume          DOUBLE PRECISION NOT NULL,
  source_version  TEXT NOT NULL DEFAULT 'coinbase_v1',
  payload_hash    TEXT NOT NULL,
  PRIMARY KEY (value_date, product_id, ingested_at)
);
SELECT create_hypertable('regime.raw_coinbase_candles', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');
CREATE UNIQUE INDEX raw_coinbase_candles_dedupe
  ON regime.raw_coinbase_candles (value_date, product_id, payload_hash);
CREATE INDEX ON regime.raw_coinbase_candles (product_id, value_date DESC, ingested_at DESC);

-- Binance spot klines — BTCUSDT + 6 alt pairs for correlation computation.
CREATE TABLE regime.raw_binance_klines (
  value_date      TIMESTAMPTZ NOT NULL,
  ingested_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  symbol          TEXT NOT NULL,           -- 'BTCUSDT', 'ETHUSDT', 'SOLUSDT', etc.
  close           DOUBLE PRECISION NOT NULL,
  volume          DOUBLE PRECISION NOT NULL,
  source_version  TEXT NOT NULL DEFAULT 'binance_spot_v1',
  payload_hash    TEXT NOT NULL,
  PRIMARY KEY (value_date, symbol, ingested_at)
);
SELECT create_hypertable('regime.raw_binance_klines', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');
CREATE UNIQUE INDEX raw_binance_klines_dedupe
  ON regime.raw_binance_klines (value_date, symbol, payload_hash);
CREATE INDEX ON regime.raw_binance_klines (symbol, value_date DESC, ingested_at DESC);

-- Feature table for Market Stress (long-format, same pattern as features_liquidity/leverage).
CREATE TABLE regime.features_market_stress (
  value_date          TIMESTAMPTZ NOT NULL,
  ingested_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  asset               asset_code NOT NULL,
  timeframe           TEXT NOT NULL,
  feature_name        TEXT NOT NULL,
  feature_version     TEXT NOT NULL
                      CHECK (feature_version ~ '^[a-z][a-z0-9_]*_v[0-9]+\.[0-9]+\.[0-9]+(_[a-z0-9]+)?$'),
  value               DOUBLE PRECISION NOT NULL,
  source_raw_versions JSONB NOT NULL,
  code_sha            TEXT NOT NULL,
  PRIMARY KEY (value_date, asset, timeframe, feature_name, feature_version, ingested_at)
);
SELECT create_hypertable('regime.features_market_stress', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');
CREATE INDEX ON regime.features_market_stress (feature_name, asset, value_date DESC, ingested_at DESC);
