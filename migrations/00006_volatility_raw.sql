-- +goose Up
-- Raw tables for the Volatility Regime domain.
-- Source: Deribit DVOL (implied volatility index), Binance klines (realized vol).
-- LIMITATION: Deribit DVOL history starts ~March 2021 for BTC, later for ETH.
-- Pre-2021 dates will have no DVOL data; features gracefully skip.

-- Daily DVOL close from Deribit volatility index.
CREATE TABLE regime.raw_deribit_dvol (
  value_date      TIMESTAMPTZ NOT NULL,
  ingested_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  asset           asset_code NOT NULL,
  dvol_close      DOUBLE PRECISION NOT NULL,
  source_version  TEXT NOT NULL DEFAULT 'deribit_dvol_v1',
  payload_hash    TEXT NOT NULL,
  PRIMARY KEY (value_date, asset, ingested_at)
);
SELECT create_hypertable('regime.raw_deribit_dvol', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');
CREATE UNIQUE INDEX raw_deribit_dvol_dedupe
  ON regime.raw_deribit_dvol (value_date, asset, payload_hash);
CREATE INDEX ON regime.raw_deribit_dvol (asset, value_date DESC, ingested_at DESC);

-- Feature table for Volatility (long-format, same pattern as other domains).
CREATE TABLE regime.features_volatility (
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
SELECT create_hypertable('regime.features_volatility', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');
CREATE INDEX ON regime.features_volatility (feature_name, asset, value_date DESC, ingested_at DESC);

-- +goose Down
DROP TABLE IF EXISTS regime.features_volatility;
DROP TABLE IF EXISTS regime.raw_deribit_dvol;
