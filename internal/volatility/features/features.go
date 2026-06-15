package features

import (
	"math"

	mmath "marketengine/pkg/math"
)

const (
	DVOLDailyName        = "dvol_daily"
	RealizedVol30dName   = "realized_vol_30d"
	IVRVSpreadDailyName  = "iv_rv_spread_daily"
	DVOLOfDVOL30dName    = "dvol_of_dvol_30d"
	IVTermSlopeDailyName = "iv_term_slope_daily"
	IVSkewDailyName      = "iv_skew_daily"

	IVRVSpreadZScore90dName  = "iv_rv_spread_zscore_90d"
	IVSkewProxyZScore90dName = "iv_skew_proxy_zscore_90d"
	DVOLOfDVOLZScore180dName = "dvol_of_dvol_zscore_180d"
	IVTermSlopeZScore90dName = "iv_term_slope_zscore_90d"
	IVSkewRealZScore90dName  = "iv_skew_real_zscore_90d"

	CGOptionsAggOIDailyName       = "cg_options_agg_oi_daily"
	CGOptionsOIVelocity30dName    = "cg_options_oi_velocity_30d"
	CGOptionsOIVelocityZScoreName = "cg_options_oi_velocity_zscore_90d"
	CGPutCallRatioDailyName       = "cg_put_call_ratio_daily"
	CGPutCallRatioZScoreName      = "cg_put_call_ratio_zscore_90d"
	CGMaxPainDistancePctName      = "cg_max_pain_distance_pct_daily"
	CGMaxPainDistanceZScoreName   = "cg_max_pain_distance_pct_zscore_90d"

	GEXNetDealerDailyName     = "gex_net_dealer_daily"
	GEXNetDealerZScore90dName = "gex_net_dealer_zscore_90d"
)

const (
	RealizedVolWindowDays = 30
	RealizedVolMinReturns = 10
	RealizedVolMinCloses  = 20

	VovWindowDays = 30
	VovMinObs     = 20

	ZScore90dWindowDays = 90
	ZScore90dMinObs     = 30

	ZScore180dWindowDays = 180
	ZScore180dMinObs     = 60

	CGOptionsVelocityWindowDays = 30
	CGOptionsZScoreWindowDays   = 90
	CGOptionsZScoreMinObs       = 30
)

func CGOptionsOIVelocity30d(now, past float64) (float64, bool) {
	if past == 0 || math.IsNaN(now) || math.IsNaN(past) || math.IsInf(now, 0) || math.IsInf(past, 0) {
		return 0, false
	}
	return (now - past) / 30.0, true
}

func CGMaxPainDistancePct(spot, maxPain float64) (float64, bool) {
	if spot <= 0 || math.IsNaN(spot) || math.IsNaN(maxPain) {
		return 0, false
	}
	return math.Abs(spot-maxPain) / spot, true
}

func AssetToBinanceSymbol(asset string) string {
	switch asset {
	case "BTC":
		return "BTCUSDT"
	case "ETH":
		return "ETHUSDT"
	}
	return ""
}

func Diff(now, past float64) (value float64, ok bool) {
	if math.IsNaN(now) || math.IsNaN(past) || math.IsInf(now, 0) || math.IsInf(past, 0) {
		return 0, false
	}
	return now - past, true
}

func StdDev(xs []float64, minObs int) (value float64, ok bool) {
	if len(xs) < minObs {
		return 0, false
	}
	sd := mmath.StdDev(xs)
	if math.IsNaN(sd) || math.IsInf(sd, 0) {
		return 0, false
	}
	return sd, true
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
