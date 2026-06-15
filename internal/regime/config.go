package regime

import "marketengine/internal/domain"

type Config struct {
	ModelVersion  string
	ConfigVersion string
	CodeSHA       string

	Weights map[domain.DomainCode]float64

	SigmoidK float64

	RocWindowDays    int
	RocWeight        float64
	DivergenceWeight float64

	TransitionBaseline float64

	MinCoverage float64

	NormLookbackDays int

	NormMinSamples int

	SmoothingSpanDays int
}

func (c *Config) Defaults() {
	if c.Weights == nil {
		c.Weights = map[domain.DomainCode]float64{
			domain.DomainLiquidity:    0.25,
			domain.DomainLeverage:     0.20,
			domain.DomainMarketStress: 0.20,
			domain.DomainCapitalFlows: 0.20,
			domain.DomainVolatility:   0.15,
		}
	}
	if c.SigmoidK == 0 {
		c.SigmoidK = 1.5
	}
	if c.RocWindowDays == 0 {
		c.RocWindowDays = 14
	}
	if c.RocWeight == 0 {
		c.RocWeight = 0.7
	}
	if c.DivergenceWeight == 0 {
		c.DivergenceWeight = 0.3
	}
	if c.TransitionBaseline == 0 {
		c.TransitionBaseline = 0.30
	}
	if c.MinCoverage == 0 {
		c.MinCoverage = 0.6
	}
	if c.NormLookbackDays == 0 {
		c.NormLookbackDays = 365
	}
	if c.NormMinSamples == 0 {
		c.NormMinSamples = 60
	}
	if c.SmoothingSpanDays == 0 {
		c.SmoothingSpanDays = 21
	}
}

func DefaultConfig() Config {
	c := Config{}
	c.Defaults()
	return c
}
