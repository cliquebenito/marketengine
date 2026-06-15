-- +goose Up
-- Wave 5 (institutional positioning): BTC ETF list snapshots + AUM history.
--   - raw_coinglass_etf_list: snapshot всех BTC ETF (~20 на сегодня) с
--     AUM, NAV, holdings, %-changes per ETF.
--   - raw_coinglass_etf_aum_history: daily AUM per ticker (полная история).
--
-- ETF list — snapshot (value_date = today). AUM history — daily per ticker.

CREATE TABLE regime.raw_coinglass_etf_list (
  value_date              TIMESTAMPTZ NOT NULL,
  ingested_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
  ticker                  TEXT NOT NULL,
  fund_name               TEXT,
  region                  TEXT,
  market_status           TEXT,
  primary_exchange        TEXT,
  fund_type               TEXT,
  shares_outstanding      NUMERIC,
  aum_usd                 NUMERIC,
  management_fee_pct      NUMERIC,
  volume_usd              NUMERIC,
  price_change_pct        NUMERIC,
  net_asset_value_usd     NUMERIC,
  premium_discount_pct    NUMERIC,
  holding_quantity        NUMERIC,        -- BTC holdings
  change_pct_24h          NUMERIC,
  change_qty_24h          NUMERIC,
  change_pct_7d           NUMERIC,
  change_qty_7d           NUMERIC,
  source_version          TEXT NOT NULL DEFAULT 'coinglass_v4',
  payload_hash            TEXT NOT NULL,
  PRIMARY KEY (value_date, ticker, ingested_at)
);
SELECT create_hypertable('regime.raw_coinglass_etf_list', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');
CREATE UNIQUE INDEX ON regime.raw_coinglass_etf_list
  (value_date, ticker, payload_hash);

CREATE TABLE regime.raw_coinglass_etf_aum_history (
  value_date     TIMESTAMPTZ NOT NULL,
  ingested_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  ticker         TEXT NOT NULL,
  aum_usd        NUMERIC NOT NULL,
  source_version TEXT NOT NULL DEFAULT 'coinglass_v4',
  payload_hash   TEXT NOT NULL,
  PRIMARY KEY (value_date, ticker, ingested_at)
);
SELECT create_hypertable('regime.raw_coinglass_etf_aum_history', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');
CREATE UNIQUE INDEX ON regime.raw_coinglass_etf_aum_history
  (value_date, ticker, payload_hash);

-- +goose Down
DROP TABLE IF EXISTS regime.raw_coinglass_etf_aum_history;
DROP TABLE IF EXISTS regime.raw_coinglass_etf_list;
