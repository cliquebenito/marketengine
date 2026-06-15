-- +goose Up
-- Raw table for Coin Metrics community API exchange flows.
-- Per-asset data (BTC, ETH) — unlike stablecoin supply which is GLOBAL.
--
-- We store inflow_usd, outflow_usd separately so downstream can reconstruct
-- netflow = inflow − outflow OR use the fields independently (e.g. different
-- features may care about inflow spikes specifically). netflow_usd is stored
-- redundantly for query convenience.
CREATE TABLE regime.raw_coinmetrics_exchange_netflow (
  value_date      TIMESTAMPTZ NOT NULL,
  ingested_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  asset           asset_code NOT NULL,
  inflow_usd      NUMERIC,
  outflow_usd     NUMERIC,
  netflow_usd     NUMERIC,
  source_version  TEXT NOT NULL DEFAULT 'coinmetrics_v1',
  payload_hash    TEXT NOT NULL,
  PRIMARY KEY (value_date, asset, ingested_at)
);
SELECT create_hypertable('regime.raw_coinmetrics_exchange_netflow', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');
CREATE INDEX ON regime.raw_coinmetrics_exchange_netflow (asset, value_date DESC, ingested_at DESC);

-- Idempotency: same asset+day+payload_hash → ON CONFLICT DO NOTHING.
CREATE UNIQUE INDEX raw_coinmetrics_exchange_netflow_dedupe
  ON regime.raw_coinmetrics_exchange_netflow (value_date, asset, payload_hash);
