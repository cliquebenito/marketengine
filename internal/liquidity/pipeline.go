package liquidity

import (
	"math"
	"time"

	"marketengine/internal/domain"
	mmath "marketengine/pkg/math"
)

type ScoreInputs struct {
	Asset     domain.Asset
	ValueDate time.Time

	ZStable          float64
	ZStableAvailable bool
	SSRPercentile    float64
	SSRAvailable     bool

	ZNetflow          float64
	ZNetflowAvailable bool

	ZTVL          float64
	ZTVLAvailable bool

	FeatureCodesUsed []string
}

func computeScore(in ScoreInputs, cfg Config) domain.DomainScore {
	cfg.Defaults()

	var componentStablecoinCapital float64
	if in.ZStableAvailable && !math.IsNaN(in.ZStable) {
		stableSignal := mmath.TanhSquash(in.ZStable, cfg.TanhScale)
		if in.SSRAvailable {
			ssrSignal := -mmath.TanhSquash((in.SSRPercentile-0.5)*4, cfg.TanhScale)
			componentStablecoinCapital = (stableSignal + ssrSignal) / 2.0
		} else {
			componentStablecoinCapital = stableSignal
		}
	}

	var componentExchangeNetflow float64
	if in.ZNetflowAvailable && !math.IsNaN(in.ZNetflow) {
		componentExchangeNetflow = -mmath.TanhSquash(in.ZNetflow, cfg.TanhScale)
	}

	var componentDefiCapital float64
	if in.ZTVLAvailable && !math.IsNaN(in.ZTVL) {
		componentDefiCapital = mmath.TanhSquash(in.ZTVL, cfg.TanhScale)
	}

	score := cfg.WeightStablecoinCapital*componentStablecoinCapital +
		cfg.WeightExchangeNetflow*componentExchangeNetflow +
		cfg.WeightDefiCapital*componentDefiCapital
	if score > 1 {
		score = 1
	} else if score < -1 {
		score = -1
	}

	components := map[string]float64{
		"component_stablecoin_capital": componentStablecoinCapital,
		"component_exchange_netflow":   componentExchangeNetflow,
		"component_defi_capital":       componentDefiCapital,
	}

	dq := map[string]any{
		"partial_coverage":            !in.ZStableAvailable || !in.ZNetflowAvailable || !in.ZTVLAvailable,
		"stablecoin_zscore_available": in.ZStableAvailable,
		"ssr_percentile_available":    in.SSRAvailable,
		"netflow_zscore_available":    in.ZNetflowAvailable,
		"tvl_zscore_available":        in.ZTVLAvailable,
	}

	return domain.DomainScore{
		Asset:            in.Asset,
		Domain:           domain.DomainLiquidity,
		ValueDate:        in.ValueDate,
		Score:            score,
		Components:       components,
		FeatureCodesUsed: in.FeatureCodesUsed,
		ModelVersion:     cfg.ModelVersion,
		ConfigVersion:    cfg.ConfigVersion,
		CodeSHA:          cfg.CodeSHA,
		SourceRawVersions: map[string]string{
			"defillama_stablecoin_supply":  "defillama_v1",
			"coinmetrics_exchange_netflow": "coinmetrics_v1",
			"defillama_tvl":                "defillama_v1",
			"coingecko_market_cap":         "coingecko_v1",
		},
		DataQuality: dq,
	}
}
