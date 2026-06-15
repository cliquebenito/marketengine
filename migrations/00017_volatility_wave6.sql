-- +goose Up
-- Wave 6: настоящий Options Gamma Exposure (GEX) через Deribit options chain.
--
-- Снапшот всех активных option-instruments из Deribit с per-strike OI и IV.
-- Greeks (gamma) считаются локально из Black-Scholes (pkg/math/blackscholes.go),
-- что устраняет потребность в per-instrument ticker запросах (938+ вызовов
-- per ingest заменяется одним book_summary вызовом + локальной арифметикой).
--
-- Snapshot-only: Deribit не предоставляет history для options chain,
-- поэтому копится только going-forward через cron.

CREATE TABLE regime.raw_deribit_options_chain (
  value_date           TIMESTAMPTZ NOT NULL,
  ingested_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  asset                asset_code  NOT NULL,
  instrument_name      TEXT        NOT NULL,
  expiry_date          DATE        NOT NULL,
  strike_price         NUMERIC     NOT NULL,
  is_put               BOOLEAN     NOT NULL,
  open_interest        NUMERIC     NOT NULL,
  mark_iv_pct          NUMERIC     NOT NULL,
  underlying_price_usd NUMERIC     NOT NULL,
  source_version       TEXT        NOT NULL DEFAULT 'deribit_v1',
  payload_hash         TEXT        NOT NULL,
  PRIMARY KEY (value_date, asset, instrument_name, ingested_at)
);
SELECT create_hypertable('regime.raw_deribit_options_chain', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');
CREATE UNIQUE INDEX ON regime.raw_deribit_options_chain
  (value_date, asset, instrument_name, payload_hash);
CREATE INDEX ON regime.raw_deribit_options_chain
  (asset, value_date DESC, ingested_at DESC);

-- +goose Down
DROP TABLE IF EXISTS regime.raw_deribit_options_chain;
