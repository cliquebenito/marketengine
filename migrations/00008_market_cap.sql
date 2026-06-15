-- +goose Up
-- Raw market cap history from CoinGecko (ETH primary, BTC backup).

CREATE TABLE regime.raw_coingecko_market_cap (
  value_date      TIMESTAMPTZ NOT NULL,
  ingested_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  coin_id         TEXT NOT NULL,            -- 'bitcoin', 'ethereum'
  market_cap_usd  NUMERIC NOT NULL,
  price_usd       NUMERIC NOT NULL,
  source_version  TEXT NOT NULL DEFAULT 'coingecko_v1',
  payload_hash    TEXT NOT NULL,
  PRIMARY KEY (value_date, coin_id, ingested_at)
);
SELECT create_hypertable('regime.raw_coingecko_market_cap', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');
CREATE UNIQUE INDEX raw_coingecko_market_cap_dedupe
  ON regime.raw_coingecko_market_cap (value_date, coin_id, payload_hash);
CREATE INDEX ON regime.raw_coingecko_market_cap (coin_id, value_date DESC, ingested_at DESC);

-- +goose Down
DROP TABLE IF EXISTS regime.raw_coingecko_market_cap;
