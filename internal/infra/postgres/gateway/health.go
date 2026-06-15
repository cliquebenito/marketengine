package pggateway

import (
	"context"

	"marketengine/internal/api/http/gateway"
	"marketengine/internal/storage"
)

type HealthChecker struct{ pool *storage.Pool }

func NewHealthChecker(pool *storage.Pool) *HealthChecker { return &HealthChecker{pool: pool} }

var _ gateway.HealthChecker = (*HealthChecker)(nil)

func (h *HealthChecker) Ping(ctx context.Context) error { return h.pool.Ping(ctx) }
