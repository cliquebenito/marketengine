package volatility

import (
	"math"
	"time"

	"marketengine/internal/domain"
	mmath "marketengine/pkg/math"
)

type ScoreInputs struct {
	Asset     domain.Asset
	ValueDate time.Time

	ZSpread          float64
	ZSpreadAvailable bool

	ZSkewReal           float64
	ZSkewRealAvailable  bool
	ZSkewProxy          float64
	ZSkewProxyAvailable bool

	ZVoV          float64
	ZVoVAvailable bool

	ZTermSlope          float64
	ZTermSlopeAvailable bool

	ZCGOptionsOIVelocity          float64
	ZCGOptionsOIVelocityAvailable bool

	ZCGPutCallRatio          float64
	ZCGPutCallRatioAvailable bool

	ZCGMaxPainDistance          float64
	ZCGMaxPainDistanceAvailable bool

	ZGEXNetDealer          float64
	ZGEXNetDealerAvailable bool

	FeatureCodesUsed []string
}

func computeScore(in ScoreInputs, cfg Config) domain.DomainScore {
	cfg.Defaults()

	var componentFear float64
	if in.ZSpreadAvailable && !math.IsNaN(in.ZSpread) {
		componentFear = -mmath.TanhSquash(in.ZSpread, cfg.TanhScale)
	}

	var componentTail float64
	switch {
	case in.ZSkewRealAvailable && !math.IsNaN(in.ZSkewReal):
		componentTail = -mmath.TanhSquash(in.ZSkewReal, cfg.TanhScale)
	case in.ZSkewProxyAvailable && !math.IsNaN(in.ZSkewProxy):
		componentTail = -mmath.TanhSquash(in.ZSkewProxy, cfg.TanhScale)
	}

	var componentVoV float64
	if in.ZVoVAvailable && !math.IsNaN(in.ZVoV) {
		componentVoV = -mmath.TanhSquash(in.ZVoV, cfg.TanhScale)
	}

	var componentTermStructure float64
	if in.ZTermSlopeAvailable && !math.IsNaN(in.ZTermSlope) {
		componentTermStructure = -mmath.TanhSquash(in.ZTermSlope, cfg.TanhScale)
	}

	use4 := in.ZTermSlopeAvailable && !math.IsNaN(in.ZTermSlope)

	var componentOptionsCross, optionsCrossN float64
	if in.ZCGOptionsOIVelocityAvailable && !math.IsNaN(in.ZCGOptionsOIVelocity) {
		componentOptionsCross += -mmath.TanhSquash(in.ZCGOptionsOIVelocity, cfg.TanhScale)
		optionsCrossN++
	}
	if in.ZCGPutCallRatioAvailable && !math.IsNaN(in.ZCGPutCallRatio) {
		componentOptionsCross += -mmath.TanhSquash(in.ZCGPutCallRatio, cfg.TanhScale)
		optionsCrossN++
	}
	if in.ZCGMaxPainDistanceAvailable && !math.IsNaN(in.ZCGMaxPainDistance) {
		componentOptionsCross += -mmath.TanhSquash(in.ZCGMaxPainDistance, cfg.TanhScale)
		optionsCrossN++
	}

	if in.ZGEXNetDealerAvailable && !math.IsNaN(in.ZGEXNetDealer) {
		componentOptionsCross += -mmath.TanhSquash(in.ZGEXNetDealer, cfg.TanhScale)
		optionsCrossN++
	}
	if optionsCrossN > 0 {
		componentOptionsCross /= optionsCrossN
	}
	useCross := optionsCrossN > 0

	var score float64
	if use4 {
		score = cfg.WeightFearPremium4*componentFear +
			cfg.WeightTermStructure4*componentTermStructure +
			cfg.WeightTailRisk4*componentTail +
			cfg.WeightVoV4*componentVoV
	} else {
		score = cfg.WeightFearPremium*componentFear +
			cfg.WeightTailRisk*componentTail +
			cfg.WeightVoV*componentVoV
	}

	if useCross && cfg.WeightOptionsCrossSource > 0 {
		w := cfg.WeightOptionsCrossSource
		score = (1.0-w)*score + w*componentOptionsCross
	}
	if score > 1 {
		score = 1
	} else if score < -1 {
		score = -1
	}

	components := map[string]float64{
		"component_fear_premium":         componentFear,
		"component_term_structure":       componentTermStructure,
		"component_tail_risk":            componentTail,
		"component_vov":                  componentVoV,
		"component_options_cross_source": componentOptionsCross,
	}

	dq := map[string]any{
		"spread_available":                 in.ZSpreadAvailable,
		"skew_real_available":              in.ZSkewRealAvailable,
		"skew_proxy_available":             in.ZSkewProxyAvailable,
		"vov_available":                    in.ZVoVAvailable,
		"term_structure_available":         in.ZTermSlopeAvailable,
		"cg_options_oi_velocity_available": in.ZCGOptionsOIVelocityAvailable,
		"cg_put_call_ratio_available":      in.ZCGPutCallRatioAvailable,
		"cg_max_pain_distance_available":   in.ZCGMaxPainDistanceAvailable,
		"gex_net_dealer_available":         in.ZGEXNetDealerAvailable,
		"options_cross_source_n":           optionsCrossN,
		"using_4_component":                use4,
		"using_options_cross_source":       useCross,
		"partial_coverage": !(in.ZSpreadAvailable &&
			in.ZVoVAvailable &&
			(in.ZSkewRealAvailable || in.ZSkewProxyAvailable)),
	}

	return domain.DomainScore{
		Asset:            in.Asset,
		Domain:           domain.DomainVolatility,
		ValueDate:        in.ValueDate,
		Score:            score,
		Components:       components,
		FeatureCodesUsed: in.FeatureCodesUsed,
		ModelVersion:     cfg.ModelVersion,
		ConfigVersion:    cfg.ConfigVersion,
		CodeSHA:          cfg.CodeSHA,
		SourceRawVersions: map[string]string{
			"deribit_dvol":    "deribit_dvol_v1",
			"deribit_options": "deribit_options_v1",
			"binance_klines":  "binance_spot_v1",
		},
		DataQuality: dq,
	}
}
