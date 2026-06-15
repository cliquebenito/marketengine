package pgbacktest

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"

	"marketengine/internal/backtest"
	"marketengine/internal/storage"
)

type MetricsRepo struct{ pool *storage.Pool }

func NewMetricsRepo(pool *storage.Pool) *MetricsRepo { return &MetricsRepo{pool: pool} }

var _ backtest.MetricsRepo = (*MetricsRepo)(nil)

func (r *MetricsRepo) Save(ctx context.Context, runID backtest.RunID, m backtest.Metric) error {
	scope := m.Scope
	if scope == "" {
		scope = "overall"
	}
	var meta []byte
	if m.Metadata != nil {
		b, err := json.Marshal(m.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metric metadata: %w", err)
		}
		meta = b
	}
	return r.pool.InTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
INSERT INTO regime.backtest_metrics (run_id, metric_name, metric_scope, value, metadata)
VALUES ($1::uuid, $2, $3, $4, $5)
ON CONFLICT (run_id, metric_name, metric_scope) DO UPDATE SET
  value    = EXCLUDED.value,
  metadata = EXCLUDED.metadata`,
			string(runID), m.Name, scope, m.Value, meta)
		return err
	})
}

func (r *MetricsRepo) GetByRun(ctx context.Context, runID backtest.RunID) ([]backtest.Metric, error) {
	rows, err := r.pool.Query(ctx, `
SELECT metric_name, metric_scope, value, metadata
FROM regime.backtest_metrics
WHERE run_id = $1::uuid
ORDER BY metric_name, metric_scope`, string(runID))
	if err != nil {
		return nil, fmt.Errorf("query backtest metrics: %w", err)
	}
	defer rows.Close()
	var out []backtest.Metric
	for rows.Next() {
		var m backtest.Metric
		var meta []byte
		if err := rows.Scan(&m.Name, &m.Scope, &m.Value, &meta); err != nil {
			return nil, err
		}
		if len(meta) > 0 {
			_ = json.Unmarshal(meta, &m.Metadata)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

var _ = pgx.ErrNoRows
