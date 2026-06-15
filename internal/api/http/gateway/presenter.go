package gateway

import (
	"marketengine/internal/domain"
)

func regimeStateToJSON(s *domain.RegimeState) map[string]any {
	drivers := make([]map[string]any, 0, len(s.TopDrivers))
	for _, d := range s.TopDrivers {
		direction := "neutral"
		if d.Contribution > 0 {
			direction = "risk_on"
		} else if d.Contribution < 0 {
			direction = "risk_off"
		}
		drivers = append(drivers, map[string]any{
			"domain":         string(d.Domain),
			"domain_display": domain.DisplayName(string(d.Domain)),
			"contribution":   d.Contribution,
			"share":          d.Share,
			"direction":      direction,
		})
	}
	contributions := make(map[string]float64, len(s.DomainContributions))
	displayNames := make(map[string]string, len(s.DomainContributions))
	for code, v := range s.DomainContributions {
		k := string(code)
		contributions[k] = v
		displayNames[k] = domain.DisplayName(k)
	}
	weights := make(map[string]float64, len(s.EffectiveWeights))
	for code, v := range s.EffectiveWeights {
		weights[string(code)] = v
	}
	coverage := make(map[string]bool, len(s.FeatureCoverageFlag))
	for code, v := range s.FeatureCoverageFlag {
		coverage[string(code)] = v
	}
	return map[string]any{
		"asset":                string(s.Asset),
		"value_date":           s.ValueDate.Format("2006-01-02"),
		"regime_indicator":     s.RegimeIndicator,
		"regime_indicator_raw": s.RegimeIndicatorRaw,
		"risk_on_probability":  s.RiskOnProbability,
		"risk_off_probability": s.RiskOffProbability,
		"transition_risk":      s.TransitionRisk,
		"domain_contributions": contributions,
		"domain_display_names": displayNames,
		"top_drivers":          drivers,
		"effective_weights":    weights,
		"feature_coverage":     coverage,
		"interaction_flags":    s.InteractionFlags,
		"model_version":        s.ModelVersion,
		"config_version":       s.ConfigVersion,
		"code_sha":             s.CodeSHA,
	}
}

func domainScoreRowToJSON(r domain.DomainScore) map[string]any {
	return map[string]any{
		"asset":          string(r.Asset),
		"domain":         string(r.Domain),
		"domain_display": domain.DisplayName(string(r.Domain)),
		"value_date":     r.ValueDate.Format("2006-01-02"),
		"score":          r.Score,
		"components":     r.Components,
		"data_quality":   r.DataQuality,
		"model_version":  r.ModelVersion,
	}
}
