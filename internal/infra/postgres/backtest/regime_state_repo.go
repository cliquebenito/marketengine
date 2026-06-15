package pgbacktest

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"

	"marketengine/internal/backtest"
	"marketengine/internal/domain"
	"marketengine/internal/storage"
)

type RegimeStateRepo struct{ pool *storage.Pool }

func NewRegimeStateRepo(pool *storage.Pool) *RegimeStateRepo {
	return &RegimeStateRepo{pool: pool}
}

var _ backtest.RegimeStateRepo = (*RegimeStateRepo)(nil)

func (r *RegimeStateRepo) Save(ctx context.Context, runID backtest.RunID, st domain.RegimeState) error {
	contrib, err := jsonifyDomainMap(st.DomainContributions)
	if err != nil {
		return err
	}
	weights, err := jsonifyDomainMap(st.EffectiveWeights)
	if err != nil {
		return err
	}
	cov, err := jsonifyCoverage(st.FeatureCoverageFlag)
	if err != nil {
		return err
	}
	drivers, err := jsonifyDrivers(st.TopDrivers)
	if err != nil {
		return err
	}
	flags := st.InteractionFlags
	if flags == nil {
		flags = []string{}
	}
	return r.pool.InTx(ctx, func(tx pgx.Tx) error {
		_, ierr := tx.Exec(ctx, `
INSERT INTO regime.backtest_regime_states
  (run_id, asset, value_date, regime_indicator, regime_indicator_raw,
   risk_on_probability, risk_off_probability, transition_risk,
   domain_contributions, top_drivers, effective_weights,
   feature_coverage_flag, interaction_flags)
VALUES ($1::uuid, $2::asset_code, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
ON CONFLICT (run_id, asset, value_date) DO UPDATE SET
  regime_indicator      = EXCLUDED.regime_indicator,
  regime_indicator_raw  = EXCLUDED.regime_indicator_raw,
  risk_on_probability   = EXCLUDED.risk_on_probability,
  risk_off_probability  = EXCLUDED.risk_off_probability,
  transition_risk       = EXCLUDED.transition_risk,
  domain_contributions  = EXCLUDED.domain_contributions,
  top_drivers           = EXCLUDED.top_drivers,
  effective_weights     = EXCLUDED.effective_weights,
  feature_coverage_flag = EXCLUDED.feature_coverage_flag,
  interaction_flags     = EXCLUDED.interaction_flags`,
			string(runID), string(st.Asset), st.ValueDate,
			st.RegimeIndicator, st.RegimeIndicatorRaw,
			st.RiskOnProbability, st.RiskOffProbability, st.TransitionRisk,
			contrib, drivers, weights, cov, flags,
		)
		return ierr
	})
}

func (r *RegimeStateRepo) GetByRun(ctx context.Context, runID backtest.RunID) ([]domain.RegimeState, error) {
	rows, err := r.pool.Query(ctx, `
SELECT asset::text, value_date, regime_indicator, regime_indicator_raw,
       risk_on_probability, risk_off_probability, transition_risk,
       domain_contributions, top_drivers, effective_weights,
       feature_coverage_flag, interaction_flags
FROM regime.backtest_regime_states
WHERE run_id = $1::uuid
ORDER BY asset, value_date ASC`, string(runID))
	if err != nil {
		return nil, fmt.Errorf("query backtest regime states: %w", err)
	}
	defer rows.Close()
	var out []domain.RegimeState
	for rows.Next() {
		var st domain.RegimeState
		var asset string
		var contrib, drivers, weights, cov []byte
		var flags []string
		if err := rows.Scan(
			&asset, &st.ValueDate, &st.RegimeIndicator, &st.RegimeIndicatorRaw,
			&st.RiskOnProbability, &st.RiskOffProbability, &st.TransitionRisk,
			&contrib, &drivers, &weights, &cov, &flags,
		); err != nil {
			return nil, err
		}
		st.Asset = domain.Asset(asset)
		st.DomainContributions, _ = parseDomainMap(contrib)
		st.EffectiveWeights, _ = parseDomainMap(weights)
		st.FeatureCoverageFlag, _ = parseCoverage(cov)
		st.TopDrivers, _ = parseDrivers(drivers)
		st.InteractionFlags = flags
		out = append(out, st)
	}
	return out, rows.Err()
}

func jsonifyDomainMap(m map[domain.DomainCode]float64) ([]byte, error) {
	str := make(map[string]float64, len(m))
	for k, v := range m {
		str[string(k)] = v
	}
	return json.Marshal(str)
}

func parseDomainMap(b []byte) (map[domain.DomainCode]float64, error) {
	if len(b) == 0 {
		return map[domain.DomainCode]float64{}, nil
	}
	var raw map[string]float64
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	out := make(map[domain.DomainCode]float64, len(raw))
	for k, v := range raw {
		out[domain.DomainCode(k)] = v
	}
	return out, nil
}

func jsonifyCoverage(m map[domain.DomainCode]bool) ([]byte, error) {
	str := make(map[string]bool, len(m))
	for k, v := range m {
		str[string(k)] = v
	}
	return json.Marshal(str)
}

func parseCoverage(b []byte) (map[domain.DomainCode]bool, error) {
	if len(b) == 0 {
		return map[domain.DomainCode]bool{}, nil
	}
	var raw map[string]bool
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	out := make(map[domain.DomainCode]bool, len(raw))
	for k, v := range raw {
		out[domain.DomainCode(k)] = v
	}
	return out, nil
}

func jsonifyDrivers(d []domain.TopDriver) ([]byte, error) {
	type wire struct {
		Domain       string  `json:"domain"`
		Contribution float64 `json:"contribution"`
		Share        float64 `json:"share"`
	}
	out := make([]wire, 0, len(d))
	for _, x := range d {
		out = append(out, wire{string(x.Domain), x.Contribution, x.Share})
	}
	return json.Marshal(out)
}

func parseDrivers(b []byte) ([]domain.TopDriver, error) {
	if len(b) == 0 {
		return nil, nil
	}
	type wire struct {
		Domain       string  `json:"domain"`
		Contribution float64 `json:"contribution"`
		Share        float64 `json:"share"`
	}
	var raw []wire
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	out := make([]domain.TopDriver, 0, len(raw))
	for _, x := range raw {
		out = append(out, domain.TopDriver{
			Domain: domain.DomainCode(x.Domain), Contribution: x.Contribution, Share: x.Share,
		})
	}
	return out, nil
}
