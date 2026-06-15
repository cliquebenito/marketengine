-- +goose Up
-- Minimal schema for Liquidity vertical slice.
-- Per D-001: up-only migrations; no Down block.
-- Per D-002: every computed row carries model_version + config_version + code_sha.
-- Per D-005: v1 emits regime_states for BTC/ETH only (asset_code enum allows future expansion).

-- TimescaleDB extension (idempotent).
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- Dedicated schema for all regime-engine tables.
CREATE SCHEMA IF NOT EXISTS regime;
SET search_path TO regime, public;

-- Enums.
CREATE TYPE asset_code AS ENUM ('BTC', 'ETH', 'SOL', 'BNB', 'XRP', 'ADA', 'DOGE', 'TOTAL', 'GLOBAL');
CREATE TYPE domain_code AS ENUM ('LIQUIDITY', 'LEVERAGE', 'MARKET_STRESS', 'CAPITAL_FLOWS', 'VOLATILITY_REGIME');

-- Model configs — PK is SHA256 content hash of the YAML body (D-002).
-- Must be created before tables that FK to it.
CREATE TABLE regime.model_configs (
  config_version   TEXT PRIMARY KEY
                   CHECK (config_version ~ '^sha256:[0-9a-f]{16,64}$'),
  scope            TEXT NOT NULL,              -- e.g. 'liquidity', 'engine'
  model_version    TEXT NOT NULL
                   CHECK (model_version ~ '^[a-z][a-z0-9_]*_v[0-9]+\.[0-9]+\.[0-9]+(_[a-z0-9]+)?$'),
  yaml_body        TEXT NOT NULL,
  yaml_parsed      JSONB NOT NULL,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by       TEXT
);
CREATE INDEX ON regime.model_configs (scope, model_version, created_at DESC);

-- Raw: DefiLlama stablecoin supply.
-- Bitemporal (value_date, ingested_at). payload_hash for idempotent ingest.
CREATE TABLE regime.raw_defillama_stablecoin_supply (
  value_date      TIMESTAMPTZ NOT NULL,
  ingested_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  stablecoin      TEXT NOT NULL,               -- 'USDT', 'USDC', 'DAI', etc.
  metric          TEXT NOT NULL,               -- 'circulating_supply_usd'
  value           NUMERIC NOT NULL,
  source_version  TEXT NOT NULL DEFAULT 'defillama_v1',
  payload_hash    TEXT NOT NULL,
  PRIMARY KEY (value_date, stablecoin, metric, ingested_at)
);
SELECT create_hypertable('regime.raw_defillama_stablecoin_supply', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');
CREATE INDEX ON regime.raw_defillama_stablecoin_supply (stablecoin, value_date DESC, ingested_at DESC);
-- Idempotency target for ingest: same payload → same row (ON CONFLICT DO NOTHING).
-- Different payload (vendor corrected a value) inserts a new bitemporal row.
-- Note: include value_date in the unique constraint because TimescaleDB requires
-- any unique index on a hypertable to include the partitioning column.
CREATE UNIQUE INDEX raw_defillama_stablecoin_supply_dedupe
  ON regime.raw_defillama_stablecoin_supply (value_date, stablecoin, metric, payload_hash);

-- Features (long-format for Liquidity).
CREATE TABLE regime.features_liquidity (
  value_date          TIMESTAMPTZ NOT NULL,
  ingested_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  asset               asset_code NOT NULL,
  timeframe           TEXT NOT NULL,            -- '1d', '4h'
  feature_name        TEXT NOT NULL,            -- 'stablecoin_supply_total'
  feature_version     TEXT NOT NULL
                      CHECK (feature_version ~ '^[a-z][a-z0-9_]*_v[0-9]+\.[0-9]+\.[0-9]+(_[a-z0-9]+)?$'),
  value               DOUBLE PRECISION NOT NULL,
  source_raw_versions JSONB NOT NULL,           -- {"defillama_stablecoin_supply": "defillama_v1"}
  code_sha            TEXT NOT NULL,
  PRIMARY KEY (value_date, asset, timeframe, feature_name, feature_version, ingested_at)
);
SELECT create_hypertable('regime.features_liquidity', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');
CREATE INDEX ON regime.features_liquidity (feature_name, asset, value_date DESC, ingested_at DESC);

-- Domain scores (single table, all 5 domains).
CREATE TABLE regime.domain_scores (
  asset               asset_code NOT NULL,
  domain              domain_code NOT NULL,
  value_date          TIMESTAMPTZ NOT NULL,
  score               DOUBLE PRECISION NOT NULL CHECK (score BETWEEN -1 AND 1),
  components          JSONB NOT NULL,
  feature_codes_used  TEXT[] NOT NULL,
  model_version       TEXT NOT NULL
                      CHECK (model_version ~ '^[a-z][a-z0-9_]*_v[0-9]+\.[0-9]+\.[0-9]+(_[a-z0-9]+)?$'),
  config_version      TEXT NOT NULL REFERENCES regime.model_configs(config_version),
  code_sha            TEXT NOT NULL,
  source_raw_versions JSONB NOT NULL,
  data_quality        JSONB,
  ingested_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (asset, domain, value_date, model_version, config_version, ingested_at)
);
SELECT create_hypertable('regime.domain_scores', 'value_date',
                         chunk_time_interval => INTERVAL '30 days');
CREATE INDEX ON regime.domain_scores (asset, domain, value_date DESC, ingested_at DESC);

-- Outbox events (plain table, not hypertable — volume low, need FIFO).
CREATE TABLE regime.outbox_events (
  event_id      BIGSERIAL PRIMARY KEY,
  topic         TEXT NOT NULL,                  -- e.g. 'features.liquidity.completed.v1'
  aggregate_id  TEXT NOT NULL,                  -- e.g. "BTC:2026-04-15"
  payload       JSONB NOT NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  published_at  TIMESTAMPTZ
);
CREATE INDEX ON regime.outbox_events (topic, published_at NULLS FIRST, event_id);

-- Outbox cursors (per consumer, last event_id seen).
CREATE TABLE regime.outbox_cursors (
  consumer_name  TEXT PRIMARY KEY,
  last_event_id  BIGINT NOT NULL DEFAULT 0,
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
