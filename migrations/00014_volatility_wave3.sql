-- +goose Up
-- Wave 3 (Startup tier): CoinGlass V4 options endpoint'ы для volatility.
--   - raw_coinglass_options_info: snapshot OI / volume per (symbol, exchange)
--   - raw_coinglass_options_oi_history: daily aggregated options OI per exchange
--   - raw_coinglass_options_max_pain: snapshot max-pain summary per expiry

CREATE TABLE regime.raw_coinglass_options_info (
  value_date              TIMESTAMPTZ NOT NULL,
  ingested_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
  symbol                  TEXT NOT NULL,
  exchange                TEXT NOT NULL,
  open_interest           NUMERIC,
  oi_market_share         NUMERIC,
  oi_change_24h           NUMERIC,
  open_interest_usd       NUMERIC,
  volume_usd_24h          NUMERIC,
  volume_change_pct       NUMERIC,
  call_open_interest      NUMERIC,
  put_open_interest       NUMERIC,
  source_version          TEXT NOT NULL DEFAULT 'coinglass_v4',
  payload_hash            TEXT NOT NULL,
  PRIMARY KEY (value_date, symbol, exchange, ingested_at)
);
SELECT create_hypertable('regime.raw_coinglass_options_info', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');
CREATE UNIQUE INDEX ON regime.raw_coinglass_options_info
  (value_date, symbol, exchange, payload_hash);

CREATE TABLE regime.raw_coinglass_options_oi_history (
  value_date     TIMESTAMPTZ NOT NULL,
  ingested_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  symbol         TEXT NOT NULL,
  exchange       TEXT NOT NULL,
  open_interest  NUMERIC NOT NULL,
  source_version TEXT NOT NULL DEFAULT 'coinglass_v4',
  payload_hash   TEXT NOT NULL,
  PRIMARY KEY (value_date, symbol, exchange, ingested_at)
);
SELECT create_hypertable('regime.raw_coinglass_options_oi_history', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');
CREATE UNIQUE INDEX ON regime.raw_coinglass_options_oi_history
  (value_date, symbol, exchange, payload_hash);

CREATE TABLE regime.raw_coinglass_options_max_pain (
  value_date              TIMESTAMPTZ NOT NULL,    -- день ингеста
  expiry_date             DATE NOT NULL,            -- expiry конкретного контракта
  ingested_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
  symbol                  TEXT NOT NULL,
  exchange                TEXT NOT NULL,
  max_pain_price          NUMERIC NOT NULL,
  call_oi_contracts       NUMERIC,
  put_oi_contracts        NUMERIC,
  call_oi_notional_usd    NUMERIC,
  put_oi_notional_usd     NUMERIC,
  call_market_value       NUMERIC,
  put_market_value        NUMERIC,
  source_version          TEXT NOT NULL DEFAULT 'coinglass_v4',
  payload_hash            TEXT NOT NULL,
  PRIMARY KEY (value_date, symbol, exchange, expiry_date, ingested_at)
);
SELECT create_hypertable('regime.raw_coinglass_options_max_pain', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');
CREATE UNIQUE INDEX ON regime.raw_coinglass_options_max_pain
  (value_date, symbol, exchange, expiry_date, payload_hash);

-- +goose Down
DROP TABLE IF EXISTS regime.raw_coinglass_options_max_pain;
DROP TABLE IF EXISTS regime.raw_coinglass_options_oi_history;
DROP TABLE IF EXISTS regime.raw_coinglass_options_info;
