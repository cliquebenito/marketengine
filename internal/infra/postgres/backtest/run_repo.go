package pgbacktest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"marketengine/internal/backtest"
	"marketengine/internal/domain"
	"marketengine/internal/storage"
)

type RunRepo struct{ pool *storage.Pool }

func NewRunRepo(pool *storage.Pool) *RunRepo { return &RunRepo{pool: pool} }

var _ backtest.RunRepo = (*RunRepo)(nil)

func (r *RunRepo) Save(ctx context.Context, run backtest.BacktestRun) (backtest.RunID, error) {
	var meta []byte
	if run.Metadata != nil {
		b, err := json.Marshal(run.Metadata)
		if err != nil {
			return "", fmt.Errorf("marshal metadata: %w", err)
		}
		meta = b
	}
	var parent any
	if run.ParentRunID != nil {
		parent = string(*run.ParentRunID)
	}
	var id string
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
INSERT INTO regime.backtest_runs
  (mode, period_start, period_end, model_version, config_version, config_yaml,
   code_sha, data_snapshot_hash, sla_offset_minutes, parent_run_id,
   harness_version, status, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING run_id::text`,
			run.Mode, run.PeriodStart, run.PeriodEnd, run.ModelVersion, run.ConfigVersion,
			run.ConfigYAML, run.CodeSHA, run.DataSnapshotHash, run.SLAOffsetMinutes,
			parent, run.HarnessVersion, run.Status, meta,
		).Scan(&id)
	})
	if err != nil {
		return "", fmt.Errorf("save backtest run: %w", err)
	}
	return backtest.RunID(id), nil
}

func (r *RunRepo) Get(ctx context.Context, id backtest.RunID) (backtest.BacktestRun, error) {
	var run backtest.BacktestRun
	var parent *string
	var completed *time.Time
	var meta []byte
	err := r.pool.QueryRow(ctx, `
SELECT run_id::text, mode, period_start, period_end, model_version, config_version,
       config_yaml, code_sha, data_snapshot_hash, sla_offset_minutes,
       parent_run_id::text, harness_version, status, started_at, completed_at, metadata
FROM regime.backtest_runs WHERE run_id = $1::uuid`, string(id)).
		Scan(
			(*string)(&run.ID), &run.Mode, &run.PeriodStart, &run.PeriodEnd,
			&run.ModelVersion, &run.ConfigVersion, &run.ConfigYAML, &run.CodeSHA,
			&run.DataSnapshotHash, &run.SLAOffsetMinutes, &parent, &run.HarnessVersion,
			&run.Status, &run.StartedAt, &completed, &meta,
		)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return backtest.BacktestRun{}, domain.ErrNotFound
		}
		return backtest.BacktestRun{}, fmt.Errorf("get backtest run: %w", err)
	}
	if parent != nil {
		p := backtest.RunID(*parent)
		run.ParentRunID = &p
	}
	if completed != nil {
		run.CompletedAt = completed
	}
	if len(meta) > 0 {
		_ = json.Unmarshal(meta, &run.Metadata)
	}
	return run, nil
}

func (r *RunRepo) UpdateStatus(ctx context.Context, id backtest.RunID, status string, completedAt *time.Time) error {
	_, err := r.pool.Exec(ctx, `
UPDATE regime.backtest_runs
SET status = $2, completed_at = $3
WHERE run_id = $1::uuid`, string(id), status, completedAt)
	if err != nil {
		return fmt.Errorf("update backtest run status: %w", err)
	}
	return nil
}
