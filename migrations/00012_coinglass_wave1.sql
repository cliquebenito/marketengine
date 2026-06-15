-- +goose Up
-- Wave 1 (Startup tier): CoinGlass V4 endpoint'ы для leverage домена.
--   - 3 long/short ratio таблицы (global / top accounts / top positions)
--   - aggregated taker buy/sell volume
--   - borrow interest rate (margin lending; пока сырьё, в score не входит)
--
-- Все таблицы bitemporal (value_date + ingested_at), идемпотентный insert
-- через UNIQUE INDEX по (value_date, …, payload_hash).

CREATE TABLE regime.raw_coinglass_long_short_global (
  value_date     TIMESTAMPTZ NOT NULL,
  ingested_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  symbol         TEXT NOT NULL,
  exchange       TEXT NOT NULL,
  long_percent   NUMERIC,
  short_percent  NUMERIC,
  ratio          NUMERIC,
  source_version TEXT NOT NULL DEFAULT 'coinglass_v4',
  payload_hash   TEXT NOT NULL,
  PRIMARY KEY (value_date, symbol, exchange, ingested_at)
);
SELECT create_hypertable('regime.raw_coinglass_long_short_global', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');
CREATE UNIQUE INDEX ON regime.raw_coinglass_long_short_global
  (value_date, symbol, exchange, payload_hash);

CREATE TABLE regime.raw_coinglass_long_short_top_account (
  value_date     TIMESTAMPTZ NOT NULL,
  ingested_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  symbol         TEXT NOT NULL,
  exchange       TEXT NOT NULL,
  long_percent   NUMERIC,
  short_percent  NUMERIC,
  ratio          NUMERIC,
  source_version TEXT NOT NULL DEFAULT 'coinglass_v4',
  payload_hash   TEXT NOT NULL,
  PRIMARY KEY (value_date, symbol, exchange, ingested_at)
);
SELECT create_hypertable('regime.raw_coinglass_long_short_top_account', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');
CREATE UNIQUE INDEX ON regime.raw_coinglass_long_short_top_account
  (value_date, symbol, exchange, payload_hash);

CREATE TABLE regime.raw_coinglass_long_short_top_position (
  value_date     TIMESTAMPTZ NOT NULL,
  ingested_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  symbol         TEXT NOT NULL,
  exchange       TEXT NOT NULL,
  long_percent   NUMERIC,
  short_percent  NUMERIC,
  ratio          NUMERIC,
  source_version TEXT NOT NULL DEFAULT 'coinglass_v4',
  payload_hash   TEXT NOT NULL,
  PRIMARY KEY (value_date, symbol, exchange, ingested_at)
);
SELECT create_hypertable('regime.raw_coinglass_long_short_top_position', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');
CREATE UNIQUE INDEX ON regime.raw_coinglass_long_short_top_position
  (value_date, symbol, exchange, payload_hash);

CREATE TABLE regime.raw_coinglass_taker_volume_aggregated (
  value_date       TIMESTAMPTZ NOT NULL,
  ingested_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  symbol           TEXT NOT NULL,           -- coin (BTC/ETH)
  buy_volume_usd   NUMERIC,
  sell_volume_usd  NUMERIC,
  source_version   TEXT NOT NULL DEFAULT 'coinglass_v4',
  payload_hash     TEXT NOT NULL,
  PRIMARY KEY (value_date, symbol, ingested_at)
);
SELECT create_hypertable('regime.raw_coinglass_taker_volume_aggregated', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');
CREATE UNIQUE INDEX ON regime.raw_coinglass_taker_volume_aggregated
  (value_date, symbol, payload_hash);

CREATE TABLE regime.raw_coinglass_borrow_rate (
  value_date     TIMESTAMPTZ NOT NULL,
  ingested_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  symbol         TEXT NOT NULL,             -- USDT/BTC/ETH/...
  exchange       TEXT NOT NULL,             -- Binance/OKX/Bybit
  interest_rate  NUMERIC NOT NULL,
  source_version TEXT NOT NULL DEFAULT 'coinglass_v4',
  payload_hash   TEXT NOT NULL,
  PRIMARY KEY (value_date, symbol, exchange, ingested_at)
);
SELECT create_hypertable('regime.raw_coinglass_borrow_rate', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');
CREATE UNIQUE INDEX ON regime.raw_coinglass_borrow_rate
  (value_date, symbol, exchange, payload_hash);

-- +goose Down
DROP TABLE IF EXISTS regime.raw_coinglass_borrow_rate;
DROP TABLE IF EXISTS regime.raw_coinglass_taker_volume_aggregated;
DROP TABLE IF EXISTS regime.raw_coinglass_long_short_top_position;
DROP TABLE IF EXISTS regime.raw_coinglass_long_short_top_account;
DROP TABLE IF EXISTS regime.raw_coinglass_long_short_global;
