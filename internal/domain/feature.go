package domain

import (
	"fmt"
	"time"
)

type FeatureKey struct {
	Name    string
	Version string
}

func (k FeatureKey) String() string { return k.Name + ":" + k.Version }

type Feature struct {
	Key               FeatureKey
	Asset             Asset
	ValueDate         time.Time
	Timeframe         string
	Value             float64
	SourceRawVersions map[string]string
	CodeSHA           string
}

func (f Feature) Validate() error {
	if f.Key.Name == "" || f.Key.Version == "" {
		return fmt.Errorf("feature missing key: name=%q version=%q", f.Key.Name, f.Key.Version)
	}
	if err := f.Asset.Validate(); err != nil {
		return fmt.Errorf("feature asset: %w", err)
	}
	if f.ValueDate.IsZero() {
		return fmt.Errorf("feature value_date is zero")
	}
	if f.Timeframe == "" {
		return fmt.Errorf("feature missing timeframe")
	}
	return nil
}
