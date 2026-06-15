package domain

import (
	"fmt"
	"time"
)

type RegimeState struct {
	Asset               Asset
	ValueDate           time.Time
	RegimeIndicator     float64
	RegimeIndicatorRaw  float64
	RiskOnProbability   float64
	RiskOffProbability  float64
	TransitionRisk      float64
	ModelVersion        string
	ConfigVersion       string
	CodeSHA             string
	DomainContributions map[DomainCode]float64
	TopDrivers          []TopDriver
	EffectiveWeights    map[DomainCode]float64
	FeatureCoverageFlag map[DomainCode]bool
	InteractionFlags    []string
}

type TopDriver struct {
	Domain       DomainCode
	Contribution float64
	Share        float64
}

type IndicatorPoint struct {
	ValueDate       time.Time
	RegimeIndicator float64
}

func (s RegimeState) Validate() error {
	if err := s.Asset.Validate(); err != nil {
		return fmt.Errorf("regime asset: %w", err)
	}
	if s.ValueDate.IsZero() {
		return fmt.Errorf("regime value_date is zero")
	}
	if s.ModelVersion == "" {
		return fmt.Errorf("regime missing model_version")
	}
	if s.RegimeIndicator < -1.0001 || s.RegimeIndicator > 1.0001 {
		return fmt.Errorf("regime_indicator out of range: %v", s.RegimeIndicator)
	}
	for _, p := range []float64{s.RiskOnProbability, s.RiskOffProbability, s.TransitionRisk} {
		if p < -0.0001 || p > 1.0001 {
			return fmt.Errorf("probability out of range: %v", p)
		}
	}
	return nil
}
