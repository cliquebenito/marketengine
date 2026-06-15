-- +goose Up
-- Wave 4 (Startup tier): CoinGlass V4 endpoint'ы для market_stress.
--   - raw_coinglass_orderbook_aggregated: daily стакан bid/ask в окне ±1%
--   - raw_coinglass_futures_spot_vol_ratio: daily futures/spot volume ratio
--
-- Bitemporal (value_date + ingested_at), идемпотентный insert через UNIQUE
-- INDEX по (value_date, …, payload_hash).

CREATE TABLE regime.raw_coinglass_orderbook_aggregated (
  value_date     TIMESTAMPTZ NOT NULL,
  ingested_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  symbol         TEXT NOT NULL,            -- coin (BTC/ETH)
  range_pct      TEXT NOT NULL DEFAULT '1', -- ±1% дефолт
  bids_usd       NUMERIC,
  bids_qty       NUMERIC,
  asks_usd       NUMERIC,
  asks_qty       NUMERIC,
  source_version TEXT NOT NULL DEFAULT 'coinglass_v4',
  payload_hash   TEXT NOT NULL,
  PRIMARY KEY (value_date, symbol, range_pct, ingested_at)
);
SELECT create_hypertable('regime.raw_coinglass_orderbook_aggregated', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');
CREATE UNIQUE INDEX ON regime.raw_coinglass_orderbook_aggregated
  (value_date, symbol, range_pct, payload_hash);

CREATE TABLE regime.raw_coinglass_futures_spot_vol_ratio (
  value_date         TIMESTAMPTZ NOT NULL,
  ingested_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  symbol             TEXT NOT NULL,
  futures_spot_ratio NUMERIC,
  futures_vol_usd    NUMERIC,
  spot_vol_usd       NUMERIC,
  source_version     TEXT NOT NULL DEFAULT 'coinglass_v4',
  payload_hash       TEXT NOT NULL,
  PRIMARY KEY (value_date, symbol, ingested_at)
);
SELECT create_hypertable('regime.raw_coinglass_futures_spot_vol_ratio', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');
CREATE UNIQUE INDEX ON regime.raw_coinglass_futures_spot_vol_ratio
  (value_date, symbol, payload_hash);

-- +goose Down
DROP TABLE IF EXISTS regime.raw_coinglass_futures_spot_vol_ratio;
DROP TABLE IF EXISTS regime.raw_coinglass_orderbook_aggregated;
