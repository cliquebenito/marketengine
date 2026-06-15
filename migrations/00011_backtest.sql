-- +goose Up
-- Backtest Replay Runner schema. Three tables, all under the regime schema.
-- Logical layout matches design/backtest-harness.md §6.3.
--
-- backtest_runs   — one row per replay / sweep / walk-forward / sensitivity invocation.
-- backtest_regime_states — per-day, per-asset replay output (mirrors regime_states).
-- backtest_metrics — per-run aggregate metrics (§3.1, §3.2, §3.5, §3.6 + bootstrap CIs).
--
-- The runner writes ONLY to these tables; live regime_states is sacred.

CREATE TABLE regime.backtest_runs (
  run_id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  mode                TEXT NOT NULL CHECK (mode IN ('replay','compare','walk_forward','sensitivity')),
  period_start        DATE NOT NULL,
  period_end          DATE NOT NULL,
  model_version       TEXT NOT NULL
                       CHECK (model_version ~ '^[a-z][a-z0-9_]*_v[0-9]+\.[0-9]+\.[0-9]+(_[a-z0-9]+)?$'),
  config_version      TEXT NOT NULL,
  config_yaml         TEXT NOT NULL,
  code_sha            TEXT NOT NULL,
  data_snapshot_hash  TEXT NOT NULL,
  sla_offset_minutes  INTEGER NOT NULL,
  parent_run_id       UUID REFERENCES regime.backtest_runs(run_id),
  harness_version     TEXT NOT NULL,
  status              TEXT NOT NULL DEFAULT 'running'
                       CHECK (status IN ('running','completed','failed')),
  started_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at        TIMESTAMPTZ,
  metadata            JSONB
);
CREATE INDEX backtest_runs_period_idx ON regime.backtest_runs (period_start, period_end);
CREATE INDEX backtest_runs_model_idx  ON regime.backtest_runs (model_version, config_version);
CREATE INDEX backtest_runs_parent_idx ON regime.backtest_runs (parent_run_id);

CREATE TABLE regime.backtest_regime_states (
  run_id                UUID NOT NULL REFERENCES regime.backtest_runs(run_id) ON DELETE CASCADE,
  asset                 asset_code NOT NULL,
  value_date            DATE NOT NULL,
  regime_indicator      DOUBLE PRECISION NOT NULL,
  regime_indicator_raw  DOUBLE PRECISION NOT NULL,
  risk_on_probability   DOUBLE PRECISION NOT NULL,
  risk_off_probability  DOUBLE PRECISION NOT NULL,
  transition_risk       DOUBLE PRECISION NOT NULL,
  domain_contributions  JSONB NOT NULL,
  top_drivers           JSONB NOT NULL,
  effective_weights     JSONB NOT NULL,
  feature_coverage_flag JSONB NOT NULL,
  interaction_flags     TEXT[] NOT NULL DEFAULT '{}',
  PRIMARY KEY (run_id, asset, value_date)
);
CREATE INDEX backtest_regime_states_run_idx ON regime.backtest_regime_states (run_id, value_date);

CREATE TABLE regime.backtest_metrics (
  run_id        UUID NOT NULL REFERENCES regime.backtest_runs(run_id) ON DELETE CASCADE,
  metric_name   TEXT NOT NULL,
  metric_scope  TEXT NOT NULL DEFAULT 'overall',
  value         DOUBLE PRECISION NOT NULL,
  metadata      JSONB,
  PRIMARY KEY (run_id, metric_name, metric_scope)
);

-- +goose Down
DROP TABLE IF EXISTS regime.backtest_metrics;
DROP TABLE IF EXISTS regime.backtest_regime_states;
DROP TABLE IF EXISTS regime.backtest_runs;
