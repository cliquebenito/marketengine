-- +goose Up
-- Regime states table: final aggregation output from the Regime Engine.
-- Per D-003: composite is named regime_indicator.
-- Per D-006: effective_weights + feature_coverage_flag for null-domain handling.

CREATE TABLE regime.regime_states (
  asset                  asset_code NOT NULL,
  value_date             TIMESTAMPTZ NOT NULL,
  regime_indicator       DOUBLE PRECISION NOT NULL,
  regime_indicator_raw   DOUBLE PRECISION NOT NULL,
  risk_on_probability    DOUBLE PRECISION NOT NULL,
  risk_off_probability   DOUBLE PRECISION NOT NULL,
  transition_risk        DOUBLE PRECISION NOT NULL,
  model_version          TEXT NOT NULL
                         CHECK (model_version ~ '^[a-z][a-z0-9_]*_v[0-9]+\.[0-9]+\.[0-9]+(_[a-z0-9]+)?$'),
  config_version         TEXT NOT NULL REFERENCES regime.model_configs(config_version),
  code_sha               TEXT NOT NULL,
  domain_contributions   JSONB NOT NULL,
  top_drivers            JSONB NOT NULL,
  effective_weights      JSONB NOT NULL,
  feature_coverage_flag  JSONB,
  interaction_flags      TEXT[],
  ingested_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (asset, value_date, model_version, config_version, ingested_at)
);

SELECT create_hypertable('regime.regime_states', 'value_date',
                         chunk_time_interval => INTERVAL '90 days');

CREATE INDEX ON regime.regime_states (asset, value_date DESC, ingested_at DESC);
CREATE INDEX ON regime.regime_states (asset, model_version, value_date DESC);
