package leverage

type Config struct {
	ModelVersion  string
	ConfigVersion string
	CodeSHA       string

	IntermediateVersion string
	FinalVersion        string

	WeightSize             float64
	WeightFunding          float64
	WeightBasis            float64
	WeightLiq              float64
	WeightMomentum         float64
	WeightCrowdPositioning float64

	TanhScale float64
}

func (c *Config) Defaults() {

	if c.WeightSize == 0 {
		c.WeightSize = 0.255
	}
	if c.WeightFunding == 0 {
		c.WeightFunding = 0.1275
	}
	if c.WeightBasis == 0 {
		c.WeightBasis = 0.17
	}
	if c.WeightLiq == 0 {
		c.WeightLiq = 0.17
	}
	if c.WeightMomentum == 0 {
		c.WeightMomentum = 0.1275
	}
	if c.WeightCrowdPositioning == 0 {
		c.WeightCrowdPositioning = 0.15
	}
	if c.TanhScale == 0 {
		c.TanhScale = 2.0
	}
}
