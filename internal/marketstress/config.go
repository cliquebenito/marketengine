package marketstress

type Config struct {
	ModelVersion  string
	ConfigVersion string
	CodeSHA       string

	IntermediateVersion string

	FinalVersion string

	LeverageBasisVersion string

	WeightCorrelation    float64
	WeightPeg            float64
	WeightCoinbase       float64
	WeightBasisInversion float64
	WeightMicrostructure float64

	TanhScale float64
}

func (c *Config) Defaults() {

	if c.WeightCorrelation == 0 {
		c.WeightCorrelation = 0.24
	}
	if c.WeightPeg == 0 {
		c.WeightPeg = 0.20
	}
	if c.WeightCoinbase == 0 {
		c.WeightCoinbase = 0.12
	}
	if c.WeightBasisInversion == 0 {
		c.WeightBasisInversion = 0.24
	}
	if c.WeightMicrostructure == 0 {
		c.WeightMicrostructure = 0.20
	}
	if c.TanhScale == 0 {
		c.TanhScale = 2.0
	}
}
