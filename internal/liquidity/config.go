package liquidity

type Config struct {
	ModelVersion  string
	ConfigVersion string
	CodeSHA       string

	SupplyFeatureVersion        string
	GrowthFeatureVersion        string
	ZScoreFeatureVersion        string
	Netflow7dFeatureVersion     string
	NetflowZScoreFeatureVersion string
	TVLFeatureVersion           string
	TVLGrowthFeatureVersion     string
	TVLZScoreFeatureVersion     string
	SupplyMajorFeatureVersion   string
	SSRFeatureVersion           string
	SSRPercentileFeatureVersion string

	WeightStablecoinCapital float64
	WeightExchangeNetflow   float64
	WeightDefiCapital       float64

	TanhScale float64
}

func (c *Config) Defaults() {
	if c.WeightStablecoinCapital == 0 {
		c.WeightStablecoinCapital = 0.40
	}
	if c.WeightExchangeNetflow == 0 {
		c.WeightExchangeNetflow = 0.35
	}
	if c.WeightDefiCapital == 0 {
		c.WeightDefiCapital = 0.25
	}
	if c.TanhScale == 0 {
		c.TanhScale = 2.0
	}
}
