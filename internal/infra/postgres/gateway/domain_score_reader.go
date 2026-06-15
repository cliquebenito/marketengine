package pggateway

import (
	"context"
	"time"

	"marketengine/internal/api/http/gateway"
	"marketengine/internal/domain"
	"marketengine/internal/storage"
)

type DomainScoreReader struct{ pool *storage.Pool }

func NewDomainScoreReader(pool *storage.Pool) *DomainScoreReader {
	return &DomainScoreReader{pool: pool}
}

var _ gateway.DomainScoreReader = (*DomainScoreReader)(nil)

func (r *DomainScoreReader) GetTimeline(ctx context.Context,
	asset domain.Asset, dom domain.DomainCode, from, to time.Time,
) ([]domain.DomainScore, error) {
	rows, err := storage.GetDomainScoreTimeline(ctx, r.pool, string(asset), string(dom), from, to)
	if err != nil {
		return nil, err
	}
	out := make([]domain.DomainScore, len(rows))
	for i, row := range rows {
		out[i] = domain.DomainScore{
			Asset:        domain.Asset(row.Asset),
			Domain:       domain.DomainCode(row.Domain),
			ValueDate:    row.ValueDate,
			Score:        row.Score,
			Components:   row.Components,
			ModelVersion: row.ModelVersion,
			DataQuality:  row.DataQuality,
		}
	}
	return out, nil
}
