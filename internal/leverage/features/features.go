package features

import (
	"math"

	mmath "marketengine/pkg/math"
)

const (
	OIUsdRawName               = "oi_usd_raw"
	OIMcapRatioName            = "oi_mcap_ratio"
	FundingRateDailyName       = "funding_rate_daily"
	BasisFromFundingApproxName = "basis_from_funding_approx"
	Basis3mDailyName           = "basis_3m_daily"
	LiquidationsDailyLogName   = "liquidations_daily_log"
	OIChange30dName            = "oi_change_30d"

	OIMcapPercentile365dName  = "oi_mcap_percentile_365d"
	FundingRateZScore90dName  = "funding_rate_zscore_90d"
	Basis3mZScore90dName      = "basis_3m_zscore_90d"
	LiquidationsStress60dName = "liquidations_stress_60d"
	OIChangeZScore30d180dName = "oi_change_zscore_30d_180d"

	TopTradersPositionSkewZName = "top_traders_position_skew_zscore_90d"
	CrowdVsSmartDivergenceZName = "crowd_vs_smart_divergence_zscore_90d"
	TakerAggressionZName        = "taker_aggression_zscore_90d"

	TopTradersPositionSkewDailyName = "top_traders_position_skew_daily"
	CrowdVsSmartDivergenceDailyName = "crowd_vs_smart_divergence_daily"
	TakerAggressionDailyName        = "taker_aggression_daily"
)

const (
	OIPercentileWindowDays   = 365
	OIPercentileMinObs       = 30
	FundingZScoreWindowDays  = 90
	FundingZScoreMinObs      = 30
	BasisZScoreWindowDays    = 90
	BasisZScoreMinObs        = 30
	LiqStressWindowDays      = 60
	LiqStressMinObs          = 20
	OIChangeZScoreWindowDays = 180
	OIChangeZScoreMinObs     = 60
	OIChangeLagDays          = 30

	CrowdPositioningZScoreWindowDays = 90
	CrowdPositioningZScoreMinObs     = 30
)

type CrowdPositioningInputs struct {
	GlobalAccountRatio float64
	TopAccountRatio    float64
	TopPositionRatio   float64
	TakerBuyUSD        float64
	TakerSellUSD       float64
}

func TopTradersPositionSkewDaily(in CrowdPositioningInputs) (float64, bool) {
	if in.TopPositionRatio <= 0 {
		return 0, false
	}
	return math.Abs(in.TopPositionRatio - 1.0), true
}

func CrowdVsSmartDivergenceDaily(in CrowdPositioningInputs) (float64, bool) {
	if in.TopAccountRatio <= 0 || in.GlobalAccountRatio <= 0 {
		return 0, false
	}
	return math.Log(in.TopAccountRatio / in.GlobalAccountRatio), true
}

func TakerAggressionDaily(in CrowdPositioningInputs) (float64, bool) {
	denom := in.TakerBuyUSD + in.TakerSellUSD
	if denom <= 0 {
		return 0, false
	}
	return (in.TakerBuyUSD - in.TakerSellUSD) / denom, true
}

func PctChange(now, past float64) (value float64, ok bool) {
	if past == 0 || math.IsNaN(now) || math.IsNaN(past) || math.IsInf(now, 0) || math.IsInf(past, 0) {
		return 0, false
	}
	return (now - past) / past, true
}

func Ratio(numer, denom float64) (value float64, ok bool) {
	if denom == 0 || math.IsNaN(numer) || math.IsNaN(denom) || math.IsInf(numer, 0) || math.IsInf(denom, 0) {
		return 0, false
	}
	return numer / denom, true
}

func Log1p(x float64) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return math.NaN()
	}
	return math.Log1p(x)
}

func ZScoreLatest(series []float64, minObs int) (value float64, ok bool) {
	if len(series) < minObs {
		return 0, false
	}
	latest := series[len(series)-1]
	z := mmath.ZScore(latest, series)
	if math.IsNaN(z) || math.IsInf(z, 0) {
		return 0, false
	}
	return z, true
}

func PercentileRank(x float64, xs []float64) (value float64, ok bool) {
	if len(xs) == 0 {
		return 0, false
	}
	below := 0
	for _, v := range xs {
		if v <= x {
			below++
		}
	}
	return float64(below) / float64(len(xs)), true
}

func AnnualizedBasisFromFunding(rate float64) float64 {
	return rate * 3 * 365 * 100
}

func CoinIDForAsset(asset string) string {
	switch asset {
	case "BTC":
		return "bitcoin"
	case "ETH":
		return "ethereum"
	}
	return ""
}

func PerpSymbolForAsset(asset string) string {
	switch asset {
	case "BTC":
		return "BTCUSDT"
	case "ETH":
		return "ETHUSDT"
	}
	return ""
}
