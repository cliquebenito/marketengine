package pgvolatility

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"marketengine/internal/domain"
	"marketengine/internal/storage"
	"marketengine/internal/volatility"
)

type ScoreRepo struct{ pool *storage.Pool }

func NewScoreRepo(pool *storage.Pool) *ScoreRepo { return &ScoreRepo{pool: pool} }

var _ volatility.ScoreRepo = (*ScoreRepo)(nil)

func (r *ScoreRepo) Save(ctx context.Context, s domain.DomainScore) error {
	if err := s.Validate(); err != nil {
		return fmt.Errorf("invalid score: %w", err)
	}
	return r.pool.InTx(ctx, func(tx pgx.Tx) error {
		return storage.InsertDomainScore(ctx, tx, storage.DomainScore{
			Asset:             string(s.Asset),
			Domain:            string(s.Domain),
			ValueDate:         s.ValueDate,
			Score:             s.Score,
			Components:        s.Components,
			FeatureCodesUsed:  s.FeatureCodesUsed,
			ModelVersion:      s.ModelVersion,
			ConfigVersion:     s.ConfigVersion,
			CodeSHA:           s.CodeSHA,
			SourceRawVersions: s.SourceRawVersions,
			DataQuality:       s.DataQuality,
		})
	})
}
