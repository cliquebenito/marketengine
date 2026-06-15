package backtest

const HarnessVersion = "harness_v1.0.0"

type Config struct {
	SLAOffsetMinutes int

	HarnessVersion string

	Classifier ClassifierConfig

	CodeSHA string
}

func (c *Config) Defaults() {
	if c.SLAOffsetMinutes == 0 {
		c.SLAOffsetMinutes = 60
	}
	if c.HarnessVersion == "" {
		c.HarnessVersion = HarnessVersion
	}
	if c.Classifier == (ClassifierConfig{}) {
		c.Classifier = DefaultClassifier()
	}
}

func DefaultConfig() Config {
	c := Config{}
	c.Defaults()
	return c
}
