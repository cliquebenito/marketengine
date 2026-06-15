package volatility

type Config struct {
	ModelVersion  string
	ConfigVersion string
	CodeSHA       string

	IntermediateVersion string
	FinalVersion        string

	WeightFearPremium float64
	WeightTailRisk    float64
	WeightVoV         float64

	WeightFearPremium4   float64
	WeightTermStructure4 float64
	WeightTailRisk4      float64
	WeightVoV4           float64

	WeightOptionsCrossSource float64

	TanhScale float64
}

func (c *Config) Defaults() {

	if c.WeightFearPremium == 0 {
		c.WeightFearPremium = 0.40
	}
	if c.WeightTailRisk == 0 {
		c.WeightTailRisk = 0.35
	}
	if c.WeightVoV == 0 {
		c.WeightVoV = 0.25
	}

	if c.WeightFearPremium4 == 0 {
		c.WeightFearPremium4 = 0.30
	}
	if c.WeightTermStructure4 == 0 {
		c.WeightTermStructure4 = 0.30
	}
	if c.WeightTailRisk4 == 0 {
		c.WeightTailRisk4 = 0.25
	}
	if c.WeightVoV4 == 0 {
		c.WeightVoV4 = 0.15
	}
	if c.WeightOptionsCrossSource == 0 {
		c.WeightOptionsCrossSource = 0.20
	}
	if c.TanhScale == 0 {
		c.TanhScale = 2.0
	}
}
