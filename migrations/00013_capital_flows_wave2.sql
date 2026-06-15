-- +goose Up
-- Wave 2 (Startup tier): CoinGlass V4 endpoint'ы для capital_flows домена.
--   - raw_coinglass_stablecoin_mcap: общая капитализация стейблкоинов (snapshot)
--   - raw_coinglass_exchange_balance: snapshot балансов BTC/ETH по биржам
--   - raw_coinglass_bitfinex_margin: daily margin long/short Bitfinex
--
-- Все таблицы bitemporal (value_date + ingested_at), идемпотентный insert
-- через UNIQUE INDEX по (value_date, …, payload_hash).

CREATE TABLE regime.raw_coinglass_stablecoin_mcap (
  value_date     TIMESTAMPTZ NOT NULL,
  ingested_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  market_cap     NUMERIC NOT NULL,
  price_usd      NUMERIC,                  -- BTC reference price из price_list
  source_version TEXT NOT NULL DEFAULT 'coinglass_v4',
  payload_hash   TEXT NOT NULL,
  PRIMARY KEY (value_date, ingested_at)
);
SELECT create_hypertable('regime.raw_coinglass_stablecoin_mcap', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');
CREATE UNIQUE INDEX ON regime.raw_coinglass_stablecoin_mcap (value_date, payload_hash);

CREATE TABLE regime.raw_coinglass_exchange_balance (
  value_date              TIMESTAMPTZ NOT NULL,
  ingested_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
  symbol                  TEXT NOT NULL,         -- BTC / ETH / USDT(ETH) / ...
  exchange                TEXT NOT NULL,
  total_balance           NUMERIC,
  balance_change_1d       NUMERIC,
  balance_change_7d       NUMERIC,
  balance_change_30d      NUMERIC,
  balance_change_pct_1d   NUMERIC,
  balance_change_pct_7d   NUMERIC,
  balance_change_pct_30d  NUMERIC,
  source_version          TEXT NOT NULL DEFAULT 'coinglass_v4',
  payload_hash            TEXT NOT NULL,
  PRIMARY KEY (value_date, symbol, exchange, ingested_at)
);
SELECT create_hypertable('regime.raw_coinglass_exchange_balance', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');
CREATE UNIQUE INDEX ON regime.raw_coinglass_exchange_balance
  (value_date, symbol, exchange, payload_hash);

CREATE TABLE regime.raw_coinglass_bitfinex_margin (
  value_date     TIMESTAMPTZ NOT NULL,
  ingested_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  symbol         TEXT NOT NULL,                  -- BTC / ETH
  long_qty       NUMERIC,
  short_qty      NUMERIC,
  source_version TEXT NOT NULL DEFAULT 'coinglass_v4',
  payload_hash   TEXT NOT NULL,
  PRIMARY KEY (value_date, symbol, ingested_at)
);
SELECT create_hypertable('regime.raw_coinglass_bitfinex_margin', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');
CREATE UNIQUE INDEX ON regime.raw_coinglass_bitfinex_margin
  (value_date, symbol, payload_hash);

-- +goose Down
DROP TABLE IF EXISTS regime.raw_coinglass_bitfinex_margin;
DROP TABLE IF EXISTS regime.raw_coinglass_exchange_balance;
DROP TABLE IF EXISTS regime.raw_coinglass_stablecoin_mcap;
