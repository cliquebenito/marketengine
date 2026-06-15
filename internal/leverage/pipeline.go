package leverage

import (
	"math"
	"time"

	"marketengine/internal/domain"
	mmath "marketengine/pkg/math"
)

type ScoreInputs struct {
	Asset     domain.Asset
	ValueDate time.Time

	OIPercentile          float64
	OIPercentileAvailable bool

	FundingZ          float64
	FundingZAvailable bool

	BasisZ          float64
	BasisZAvailable bool

	LiqZ          float64
	LiqZAvailable bool

	OIChangeZ          float64
	OIChangeZAvailable bool

	PositionSkewZ          float64
	PositionSkewZAvailable bool

	CrowdDivergenceZ          float64
	CrowdDivergenceZAvailable bool

	TakerAggressionZ          float64
	TakerAggressionZAvailable bool

	FeatureCodesUsed []string
}

func computeScore(in ScoreInputs, cfg Config) domain.DomainScore {
	cfg.Defaults()

	var compSize float64
	if in.OIPercentileAvailable && !math.IsNaN(in.OIPercentile) {

		compSize = -mmath.TanhSquash((in.OIPercentile-0.5)*4, cfg.TanhScale)
	}

	var compFunding float64
	if in.FundingZAvailable && !math.IsNaN(in.FundingZ) {
		compFunding = mmath.TanhSquash(in.FundingZ, cfg.TanhScale)
	}

	var compBasis float64
	if in.BasisZAvailable && !math.IsNaN(in.BasisZ) {
		compBasis = mmath.TanhSquash(in.BasisZ, cfg.TanhScale)
	}

	var compLiq float64
	if in.LiqZAvailable && !math.IsNaN(in.LiqZ) {
		compLiq = -mmath.TanhSquash(in.LiqZ, cfg.TanhScale)
	}

	var compMomentum float64
	if in.OIChangeZAvailable && !math.IsNaN(in.OIChangeZ) {
		compMomentum = -mmath.TanhSquash(in.OIChangeZ, cfg.TanhScale)
	}

	var compCrowd, crowdN float64
	if in.PositionSkewZAvailable && !math.IsNaN(in.PositionSkewZ) {
		compCrowd += -mmath.TanhSquash(in.PositionSkewZ, cfg.TanhScale)
		crowdN++
	}
	if in.CrowdDivergenceZAvailable && !math.IsNaN(in.CrowdDivergenceZ) {
		compCrowd += mmath.TanhSquash(in.CrowdDivergenceZ, cfg.TanhScale)
		crowdN++
	}
	if in.TakerAggressionZAvailable && !math.IsNaN(in.TakerAggressionZ) {
		compCrowd += mmath.TanhSquash(in.TakerAggressionZ, cfg.TanhScale)
		crowdN++
	}
	if crowdN > 0 {
		compCrowd /= crowdN
	}

	score := cfg.WeightSize*compSize +
		cfg.WeightFunding*compFunding +
		cfg.WeightBasis*compBasis +
		cfg.WeightLiq*compLiq +
		cfg.WeightMomentum*compMomentum +
		cfg.WeightCrowdPositioning*compCrowd
	if score > 1 {
		score = 1
	} else if score < -1 {
		score = -1
	}

	components := map[string]float64{
		"component_size":              compSize,
		"component_funding":           compFunding,
		"component_basis":             compBasis,
		"component_liq":               compLiq,
		"component_momentum":          compMomentum,
		"component_crowd_positioning": compCrowd,
	}

	dq := map[string]any{
		"oi_available":               in.OIPercentileAvailable,
		"funding_available":          in.FundingZAvailable,
		"basis_available":            in.BasisZAvailable,
		"liq_available":              in.LiqZAvailable,
		"momentum_available":         in.OIChangeZAvailable,
		"position_skew_available":    in.PositionSkewZAvailable,
		"crowd_divergence_available": in.CrowdDivergenceZAvailable,
		"taker_aggression_available": in.TakerAggressionZAvailable,
		"crowd_positioning_n":        crowdN,
		"partial_coverage": !(in.OIPercentileAvailable && in.FundingZAvailable &&
			in.BasisZAvailable && in.LiqZAvailable && in.OIChangeZAvailable &&
			(in.PositionSkewZAvailable || in.CrowdDivergenceZAvailable || in.TakerAggressionZAvailable)),
	}

	return domain.DomainScore{
		Asset:            in.Asset,
		Domain:           domain.DomainLeverage,
		ValueDate:        in.ValueDate,
		Score:            score,
		Components:       components,
		FeatureCodesUsed: in.FeatureCodesUsed,
		ModelVersion:     cfg.ModelVersion,
		ConfigVersion:    cfg.ConfigVersion,
		CodeSHA:          cfg.CodeSHA,
		SourceRawVersions: map[string]string{
			"exchange_oi":      "exchange_v1",
			"exchange_funding": "exchange_v1",
			"exchange_liqs":    "exchange_v1",
			"deribit_basis":    "deribit_v1",
			"coinglass_basis":  "coinglass_v4",
		},
		DataQuality: dq,
	}
}
