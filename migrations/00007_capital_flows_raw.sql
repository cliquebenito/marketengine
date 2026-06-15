-- +goose Up
-- Raw tables for the Capital Flows domain.
-- Sources: CoinGlass V4 (ETF flows), Glassnode (LTH supply, miner flow — future).

-- ETF flows — BTC and ETH spot ETF net flows from CoinGlass.
CREATE TABLE regime.raw_coinglass_etf_flows (
  value_date      TIMESTAMPTZ NOT NULL,
  ingested_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  flow_type       TEXT NOT NULL,            -- 'BTC' or 'ETH'
  total_flow_usd  DOUBLE PRECISION NOT NULL,
  price_usd       DOUBLE PRECISION NOT NULL,
  source_version  TEXT NOT NULL DEFAULT 'coinglass_v4',
  payload_hash    TEXT NOT NULL,
  PRIMARY KEY (value_date, flow_type, ingested_at)
);
SELECT create_hypertable('regime.raw_coinglass_etf_flows', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');
CREATE UNIQUE INDEX raw_coinglass_etf_flows_dedupe
  ON regime.raw_coinglass_etf_flows (value_date, flow_type, payload_hash);
CREATE INDEX ON regime.raw_coinglass_etf_flows (flow_type, value_date DESC, ingested_at DESC);

-- Glassnode LTH supply — placeholder for future provider.
CREATE TABLE regime.raw_glassnode_lth_supply (
  value_date      TIMESTAMPTZ NOT NULL,
  ingested_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  asset           asset_code NOT NULL,
  lth_supply      DOUBLE PRECISION NOT NULL,
  source_version  TEXT NOT NULL DEFAULT 'glassnode_v1',
  payload_hash    TEXT NOT NULL,
  PRIMARY KEY (value_date, asset, ingested_at)
);
SELECT create_hypertable('regime.raw_glassnode_lth_supply', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');
CREATE UNIQUE INDEX raw_glassnode_lth_supply_dedupe
  ON regime.raw_glassnode_lth_supply (value_date, asset, payload_hash);

-- Glassnode miner flow — placeholder for future provider.
CREATE TABLE regime.raw_glassnode_miner_flow (
  value_date      TIMESTAMPTZ NOT NULL,
  ingested_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  asset           asset_code NOT NULL,
  miner_flow_usd  DOUBLE PRECISION NOT NULL,
  source_version  TEXT NOT NULL DEFAULT 'glassnode_v1',
  payload_hash    TEXT NOT NULL,
  PRIMARY KEY (value_date, asset, ingested_at)
);
SELECT create_hypertable('regime.raw_glassnode_miner_flow', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');
CREATE UNIQUE INDEX raw_glassnode_miner_flow_dedupe
  ON regime.raw_glassnode_miner_flow (value_date, asset, payload_hash);

-- Feature table for Capital Flows (long-format, same pattern as features_leverage).
CREATE TABLE regime.features_capital_flows (
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
SELECT create_hypertable('regime.features_capital_flows', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');
CREATE INDEX ON regime.features_capital_flows (feature_name, asset, value_date DESC, ingested_at DESC);

-- +goose Down
DROP TABLE IF EXISTS regime.features_capital_flows;
DROP TABLE IF EXISTS regime.raw_glassnode_miner_flow;
DROP TABLE IF EXISTS regime.raw_glassnode_lth_supply;
DROP TABLE IF EXISTS regime.raw_coinglass_etf_flows;
