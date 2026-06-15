package marketstress

import (
	"math"
	"time"

	"marketengine/internal/domain"
	mmath "marketengine/pkg/math"
)

type ScoreInputs struct {
	Asset     domain.Asset
	ValueDate time.Time

	ZCorrelation          float64
	ZCorrelationAvailable bool

	ZPeg          float64
	ZPegAvailable bool

	ZCoinbase          float64
	ZCoinbaseAvailable bool

	ZBasis          float64
	ZBasisAvailable bool

	ZBookImbalance             float64
	ZBookImbalanceAvailable    bool
	ZFuturesSpotRatio          float64
	ZFuturesSpotRatioAvailable bool

	FeatureCodesUsed []string
}

func computeScore(in ScoreInputs, cfg Config) domain.DomainScore {
	cfg.Defaults()

	var compCorr, compPeg, compCoinbase, compBasis float64
	if in.ZCorrelationAvailable && !math.IsNaN(in.ZCorrelation) {
		compCorr = mmath.TanhSquash(in.ZCorrelation, cfg.TanhScale)
	}
	if in.ZPegAvailable && !math.IsNaN(in.ZPeg) {
		compPeg = mmath.TanhSquash(in.ZPeg, cfg.TanhScale)
	}
	if in.ZCoinbaseAvailable && !math.IsNaN(in.ZCoinbase) {
		compCoinbase = mmath.TanhSquash(in.ZCoinbase, cfg.TanhScale)
	}
	if in.ZBasisAvailable && !math.IsNaN(in.ZBasis) {
		compBasis = mmath.TanhSquash(in.ZBasis, cfg.TanhScale)
	}

	var compMicro, microN float64
	if in.ZBookImbalanceAvailable && !math.IsNaN(in.ZBookImbalance) {
		compMicro += -mmath.TanhSquash(in.ZBookImbalance, cfg.TanhScale)
		microN++
	}
	if in.ZFuturesSpotRatioAvailable && !math.IsNaN(in.ZFuturesSpotRatio) {
		compMicro += mmath.TanhSquash(in.ZFuturesSpotRatio, cfg.TanhScale)
		microN++
	}
	if microN > 0 {
		compMicro /= microN
	}

	score := -(cfg.WeightCorrelation*compCorr +
		cfg.WeightPeg*compPeg +
		cfg.WeightCoinbase*compCoinbase +
		cfg.WeightBasisInversion*compBasis +
		cfg.WeightMicrostructure*compMicro)
	if score > 1 {
		score = 1
	} else if score < -1 {
		score = -1
	}

	components := map[string]float64{
		"component_correlation":     compCorr,
		"component_peg":             compPeg,
		"component_coinbase":        compCoinbase,
		"component_basis_inversion": compBasis,
		"component_microstructure":  compMicro,
	}

	dq := map[string]any{
		"partial_coverage":             !in.ZCorrelationAvailable || !in.ZPegAvailable || !in.ZCoinbaseAvailable || !in.ZBasisAvailable,
		"correlation_available":        in.ZCorrelationAvailable,
		"peg_available":                in.ZPegAvailable,
		"coinbase_available":           in.ZCoinbaseAvailable,
		"basis_available":              in.ZBasisAvailable,
		"book_imbalance_available":     in.ZBookImbalanceAvailable,
		"futures_spot_ratio_available": in.ZFuturesSpotRatioAvailable,
		"microstructure_n":             microN,
	}

	return domain.DomainScore{
		Asset:            in.Asset,
		Domain:           domain.DomainMarketStress,
		ValueDate:        in.ValueDate,
		Score:            score,
		Components:       components,
		FeatureCodesUsed: in.FeatureCodesUsed,
		ModelVersion:     cfg.ModelVersion,
		ConfigVersion:    cfg.ConfigVersion,
		CodeSHA:          cfg.CodeSHA,
		SourceRawVersions: map[string]string{
			"binance_klines":   "binance_spot_v1",
			"kraken_ohlc":      "kraken_v1",
			"coinbase_candles": "coinbase_v1",
			"leverage_basis":   cfg.LeverageBasisVersion,
		},
		DataQuality: dq,
	}
}
