-- +goose Up
-- CoinGlass Coinbase Premium Index and Futures Basis History raw tables.

CREATE TABLE regime.raw_coinglass_coinbase_premium (
  value_date     TIMESTAMPTZ NOT NULL,
  ingested_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  premium_usd    NUMERIC,
  premium_rate   NUMERIC,
  coinbase_price NUMERIC,
  source_version TEXT NOT NULL DEFAULT 'coinglass_v4',
  payload_hash   TEXT NOT NULL,
  PRIMARY KEY (value_date, ingested_at)
);

SELECT create_hypertable('regime.raw_coinglass_coinbase_premium', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');

CREATE UNIQUE INDEX ON regime.raw_coinglass_coinbase_premium (value_date, payload_hash);

CREATE TABLE regime.raw_coinglass_futures_basis (
  value_date           TIMESTAMPTZ NOT NULL,
  ingested_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  symbol               TEXT NOT NULL,
  exchange             TEXT NOT NULL,
  annualized_basis_pct NUMERIC NOT NULL,
  close_basis          NUMERIC NOT NULL,
  source_version       TEXT NOT NULL DEFAULT 'coinglass_v4',
  payload_hash         TEXT NOT NULL,
  PRIMARY KEY (value_date, symbol, exchange, ingested_at)
);

SELECT create_hypertable('regime.raw_coinglass_futures_basis', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');

CREATE UNIQUE INDEX ON regime.raw_coinglass_futures_basis (value_date, symbol, exchange, payload_hash);
