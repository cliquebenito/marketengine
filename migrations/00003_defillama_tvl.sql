-- +goose Up
-- Raw table for DefiLlama Ethereum-chain TVL history.
-- Scope is Ethereum-only (simplification vs design/regime-engine.md §9 which
-- specifies "EVM basket: Ethereum + Arbitrum + Optimism + Base + Polygon").
-- Rationale: one HTTP endpoint, one ingestor; expand to basket in a later
-- iteration by adding per-chain columns or rows.
CREATE TABLE regime.raw_defillama_tvl (
  value_date      TIMESTAMPTZ NOT NULL,
  ingested_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  chain           TEXT NOT NULL DEFAULT 'Ethereum',
  tvl_usd         NUMERIC NOT NULL,
  source_version  TEXT NOT NULL DEFAULT 'defillama_v1',
  payload_hash    TEXT NOT NULL,
  PRIMARY KEY (value_date, chain, ingested_at)
);
SELECT create_hypertable('regime.raw_defillama_tvl', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');
CREATE INDEX ON regime.raw_defillama_tvl (chain, value_date DESC, ingested_at DESC);

CREATE UNIQUE INDEX raw_defillama_tvl_dedupe
  ON regime.raw_defillama_tvl (value_date, chain, payload_hash);
