package domain

import (
	"fmt"
	"time"
)

type DomainCode string

const (
	DomainLiquidity    DomainCode = "LIQUIDITY"
	DomainLeverage     DomainCode = "LEVERAGE"
	DomainMarketStress DomainCode = "MARKET_STRESS"
	DomainCapitalFlows DomainCode = "CAPITAL_FLOWS"
	DomainVolatility   DomainCode = "VOLATILITY_REGIME"
)

func AllDomains() []DomainCode {
	return []DomainCode{
		DomainLiquidity, DomainLeverage, DomainMarketStress,
		DomainCapitalFlows, DomainVolatility,
	}
}

func (d DomainCode) Validate() error {
	for _, x := range AllDomains() {
		if x == d {
			return nil
		}
	}
	return fmt.Errorf("invalid domain %q", d)
}

func (d DomainCode) String() string { return string(d) }

type DomainScore struct {
	Asset             Asset
	Domain            DomainCode
	ValueDate         time.Time
	Score             float64
	Components        map[string]float64
	FeatureCodesUsed  []string
	ModelVersion      string
	ConfigVersion     string
	CodeSHA           string
	SourceRawVersions map[string]string
	DataQuality       map[string]any
}

func (s DomainScore) Validate() error {
	if err := s.Asset.Validate(); err != nil {
		return fmt.Errorf("score asset: %w", err)
	}
	if err := s.Domain.Validate(); err != nil {
		return fmt.Errorf("score domain: %w", err)
	}
	if s.ValueDate.IsZero() {
		return fmt.Errorf("score value_date is zero")
	}
	if s.Score < -1.0001 || s.Score > 1.0001 {
		return fmt.Errorf("score out of range: %v", s.Score)
	}
	if s.ModelVersion == "" {
		return fmt.Errorf("score missing model_version")
	}
	return nil
}
