-- +goose Up
-- Raw tables for the Leverage domain. Self-aggregated from Binance, Bybit, OKX, Deribit.
-- Per-asset data (BTC, ETH).

-- Open Interest — aggregated across exchanges. One row per (date, asset, exchange).
CREATE TABLE regime.raw_exchange_oi (
  value_date      TIMESTAMPTZ NOT NULL,
  ingested_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  asset           asset_code NOT NULL,
  exchange        TEXT NOT NULL,         -- 'binance', 'bybit', 'okx'
  oi_usd          NUMERIC NOT NULL,
  source_version  TEXT NOT NULL DEFAULT 'exchange_v1',
  payload_hash    TEXT NOT NULL,
  PRIMARY KEY (value_date, asset, exchange, ingested_at)
);
SELECT create_hypertable('regime.raw_exchange_oi', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');
CREATE UNIQUE INDEX raw_exchange_oi_dedupe
  ON regime.raw_exchange_oi (value_date, asset, exchange, payload_hash);
CREATE INDEX ON regime.raw_exchange_oi (asset, value_date DESC, ingested_at DESC);

-- Funding rates — per exchange, per asset. 8h resolution stored; will aggregate to daily.
CREATE TABLE regime.raw_exchange_funding (
  funding_time    TIMESTAMPTZ NOT NULL,
  ingested_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  asset           asset_code NOT NULL,
  exchange        TEXT NOT NULL,
  rate            DOUBLE PRECISION NOT NULL,
  source_version  TEXT NOT NULL DEFAULT 'exchange_v1',
  payload_hash    TEXT NOT NULL,
  PRIMARY KEY (funding_time, asset, exchange, ingested_at)
);
SELECT create_hypertable('regime.raw_exchange_funding', 'funding_time',
                         chunk_time_interval => INTERVAL '90 days');
CREATE UNIQUE INDEX raw_exchange_funding_dedupe
  ON regime.raw_exchange_funding (funding_time, asset, exchange, payload_hash);
CREATE INDEX ON regime.raw_exchange_funding (asset, funding_time DESC, ingested_at DESC);

-- Liquidations — daily aggregated per asset per exchange.
-- NOTE: Binance caps liq feed since May 2021 (~1/sec). Values are structural
-- underestimates for Binance. Flagged in data_quality.
CREATE TABLE regime.raw_exchange_liquidations (
  value_date      TIMESTAMPTZ NOT NULL,
  ingested_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  asset           asset_code NOT NULL,
  exchange        TEXT NOT NULL,
  long_liqs_usd   NUMERIC NOT NULL DEFAULT 0,
  short_liqs_usd  NUMERIC NOT NULL DEFAULT 0,
  source_version  TEXT NOT NULL DEFAULT 'exchange_v1',
  payload_hash    TEXT NOT NULL,
  PRIMARY KEY (value_date, asset, exchange, ingested_at)
);
SELECT create_hypertable('regime.raw_exchange_liquidations', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');
CREATE UNIQUE INDEX raw_exchange_liquidations_dedupe
  ON regime.raw_exchange_liquidations (value_date, asset, exchange, payload_hash);

-- Deribit basis snapshots — daily per asset.
CREATE TABLE regime.raw_deribit_basis (
  value_date        TIMESTAMPTZ NOT NULL,
  ingested_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  asset             asset_code NOT NULL,
  instrument_name   TEXT NOT NULL,
  futures_price     DOUBLE PRECISION NOT NULL,
  spot_price        DOUBLE PRECISION NOT NULL,
  annualized_basis  DOUBLE PRECISION NOT NULL,
  days_to_expiry    INT NOT NULL,
  source_version    TEXT NOT NULL DEFAULT 'deribit_v1',
  payload_hash      TEXT NOT NULL,
  PRIMARY KEY (value_date, asset, ingested_at)
);
SELECT create_hypertable('regime.raw_deribit_basis', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');
CREATE UNIQUE INDEX raw_deribit_basis_dedupe
  ON regime.raw_deribit_basis (value_date, asset, payload_hash);

-- Feature table for Leverage (long-format, same pattern as features_liquidity).
CREATE TABLE regime.features_leverage (
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
SELECT create_hypertable('regime.features_leverage', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');
CREATE INDEX ON regime.features_leverage (feature_name, asset, value_date DESC, ingested_at DESC);
