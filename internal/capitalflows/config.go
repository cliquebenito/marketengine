package capitalflows

type Config struct {
	ModelVersion  string
	ConfigVersion string
	CodeSHA       string

	IntermediateVersion string
	FinalVersion        string

	TanhScale float64

	WeightETF           float64
	WeightLTH           float64
	WeightMiner         float64
	WeightLiquidityFlow float64

	WeightLTHPre   float64
	WeightMinerPre float64
}

func (c *Config) Defaults() {
	if c.TanhScale == 0 {
		c.TanhScale = 2.0
	}

	if c.WeightETF == 0 {
		c.WeightETF = 0.416
	}
	if c.WeightLTH == 0 {
		c.WeightLTH = 0.384
	}
	if c.WeightLiquidityFlow == 0 {
		c.WeightLiquidityFlow = 0.20
	}

	if c.WeightLTHPre == 0 {
		c.WeightLTHPre = 1.00
	}

}
