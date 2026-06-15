package capitalflows

import (
	"math"
	"time"

	"marketengine/internal/domain"
	mmath "marketengine/pkg/math"
)

var ETFLaunchDate = time.Date(2024, 1, 11, 0, 0, 0, 0, time.UTC)

type ScoreInputs struct {
	Asset     domain.Asset
	ValueDate time.Time

	ZETF          float64
	ZETFAvailable bool

	ZLTH          float64
	ZLTHAvailable bool

	ZMiner          float64
	ZMinerAvailable bool

	ZStablecoinVelocity          float64
	ZStablecoinVelocityAvailable bool

	ZExchangeBalance          float64
	ZExchangeBalanceAvailable bool

	ZBitfinexMargin          float64
	ZBitfinexMarginAvailable bool

	ZStablecoinDryPowder7d           float64
	ZStablecoinDryPowder7dAvailable  bool
	ZStablecoinDryPowder30d          float64
	ZStablecoinDryPowder30dAvailable bool

	ZETFAUMVelocity          float64
	ZETFAUMVelocityAvailable bool

	ZETFConcentrationHHI          float64
	ZETFConcentrationHHIAvailable bool

	ZOptionsDealerSkew          float64
	ZOptionsDealerSkewAvailable bool

	FeatureCodesUsed []string
}

func computeScore(in ScoreInputs, cfg Config) domain.DomainScore {
	cfg.Defaults()

	postETF := !in.ValueDate.Before(ETFLaunchDate)

	var componentETF float64
	if in.ZETFAvailable && !math.IsNaN(in.ZETF) {
		componentETF = mmath.TanhSquash(in.ZETF, cfg.TanhScale)
	}

	var componentLTH float64
	if in.ZLTHAvailable && !math.IsNaN(in.ZLTH) {
		componentLTH = mmath.TanhSquash(in.ZLTH, cfg.TanhScale)
	}

	var componentMiner float64
	if in.ZMinerAvailable && !math.IsNaN(in.ZMiner) {
		componentMiner = mmath.TanhSquash(in.ZMiner, cfg.TanhScale)
	}

	var componentLiquidity, liquidityN float64
	if in.ZStablecoinVelocityAvailable && !math.IsNaN(in.ZStablecoinVelocity) {
		componentLiquidity += mmath.TanhSquash(in.ZStablecoinVelocity, cfg.TanhScale)
		liquidityN++
	}
	if in.ZExchangeBalanceAvailable && !math.IsNaN(in.ZExchangeBalance) {
		componentLiquidity += -mmath.TanhSquash(in.ZExchangeBalance, cfg.TanhScale)
		liquidityN++
	}
	if in.ZBitfinexMarginAvailable && !math.IsNaN(in.ZBitfinexMargin) {
		componentLiquidity += mmath.TanhSquash(in.ZBitfinexMargin, cfg.TanhScale)
		liquidityN++
	}

	if in.ZStablecoinDryPowder7dAvailable && !math.IsNaN(in.ZStablecoinDryPowder7d) {
		componentLiquidity += mmath.TanhSquash(in.ZStablecoinDryPowder7d, cfg.TanhScale)
		liquidityN++
	}
	if in.ZStablecoinDryPowder30dAvailable && !math.IsNaN(in.ZStablecoinDryPowder30d) {
		componentLiquidity += mmath.TanhSquash(in.ZStablecoinDryPowder30d, cfg.TanhScale)
		liquidityN++
	}

	if in.ZETFAUMVelocityAvailable && !math.IsNaN(in.ZETFAUMVelocity) {
		componentLiquidity += mmath.TanhSquash(in.ZETFAUMVelocity, cfg.TanhScale)
		liquidityN++
	}
	if in.ZETFConcentrationHHIAvailable && !math.IsNaN(in.ZETFConcentrationHHI) {
		componentLiquidity += -mmath.TanhSquash(in.ZETFConcentrationHHI, cfg.TanhScale)
		liquidityN++
	}
	if in.ZOptionsDealerSkewAvailable && !math.IsNaN(in.ZOptionsDealerSkew) {
		componentLiquidity += mmath.TanhSquash(in.ZOptionsDealerSkew, cfg.TanhScale)
		liquidityN++
	}
	if liquidityN > 0 {
		componentLiquidity /= liquidityN
	}

	var score float64
	if postETF {
		score = cfg.WeightETF*componentETF +
			cfg.WeightLTH*componentLTH +
			cfg.WeightMiner*componentMiner +
			cfg.WeightLiquidityFlow*componentLiquidity
	} else {

		score = cfg.WeightLTHPre*componentLTH +
			cfg.WeightMinerPre*componentMiner
	}

	if score > 1 {
		score = 1
	} else if score < -1 {
		score = -1
	}

	components := map[string]float64{
		"component_etf":            componentETF,
		"component_lth":            componentLTH,
		"component_miner":          componentMiner,
		"component_liquidity_flow": componentLiquidity,
	}

	var partialCoverage bool
	if postETF {
		partialCoverage = !in.ZETFAvailable || !in.ZLTHAvailable || !in.ZMinerAvailable
	} else {
		partialCoverage = !in.ZLTHAvailable || !in.ZMinerAvailable
	}

	dq := map[string]any{
		"post_etf":                            postETF,
		"etf_available":                       in.ZETFAvailable,
		"lth_available":                       in.ZLTHAvailable,
		"miner_available":                     in.ZMinerAvailable,
		"stablecoin_velocity_available":       in.ZStablecoinVelocityAvailable,
		"exchange_balance_available":          in.ZExchangeBalanceAvailable,
		"bitfinex_margin_available":           in.ZBitfinexMarginAvailable,
		"stablecoin_dry_powder_7d_available":  in.ZStablecoinDryPowder7dAvailable,
		"stablecoin_dry_powder_30d_available": in.ZStablecoinDryPowder30dAvailable,
		"etf_aum_velocity_available":          in.ZETFAUMVelocityAvailable,
		"etf_concentration_hhi_available":     in.ZETFConcentrationHHIAvailable,
		"options_dealer_skew_available":       in.ZOptionsDealerSkewAvailable,
		"liquidity_flow_n":                    liquidityN,
		"partial_coverage":                    partialCoverage,

		"display_name": "Holder Conviction",
		"rename_note":  "domain_code=CAPITAL_FLOWS retained for DB schema stability; semantics since engine_v1.1.0 = Holder Conviction (ETF flows + LTH supply changes)",
	}

	return domain.DomainScore{
		Asset:            in.Asset,
		Domain:           domain.DomainCapitalFlows,
		ValueDate:        in.ValueDate,
		Score:            score,
		Components:       components,
		FeatureCodesUsed: in.FeatureCodesUsed,
		ModelVersion:     cfg.ModelVersion,
		ConfigVersion:    cfg.ConfigVersion,
		CodeSHA:          cfg.CodeSHA,
		SourceRawVersions: map[string]string{
			"coinglass_etf_flows": "coinglass_v4",
			"lth_supply":          "coinglass_v4",
		},
		DataQuality: dq,
	}
}
