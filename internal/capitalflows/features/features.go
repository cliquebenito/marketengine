package features

import (
	"math"

	mmath "marketengine/pkg/math"
)

const (
	ETFNetflowDailyName    = "etf_netflow_daily"
	LTHSupplyDailyName     = "lth_supply_daily"
	LTHSupplyChange30dName = "lth_supply_change_30d"

	ETFNetflowZScore90dName       = "etf_netflow_zscore_90d"
	LTHSupplyChangeZScore180dName = "lth_supply_change_zscore_180d"
	MinerNetflowZScore180dName    = "miner_netflow_zscore_180d"

	StablecoinMcapVelocity30dName      = "stablecoin_mcap_velocity_30d"
	StablecoinVelocityZScore90dName    = "stablecoin_mcap_velocity_zscore_90d"
	ExchangeBalanceChange7dName        = "exchange_balance_change_7d"
	ExchangeBalanceChangeZScore90dName = "exchange_balance_change_zscore_90d"
	BitfinexMarginSkewDailyName        = "bitfinex_margin_skew_daily"
	BitfinexMarginSkewZScore90dName    = "bitfinex_margin_skew_zscore_90d"

	StablecoinDryPowderChange7dName   = "stablecoin_dry_powder_change_7d"
	StablecoinDryPowderChange7dZName  = "stablecoin_dry_powder_change_7d_zscore_90d"
	StablecoinDryPowderChange30dName  = "stablecoin_dry_powder_change_30d"
	StablecoinDryPowderChange30dZName = "stablecoin_dry_powder_change_30d_zscore_90d"

	ETFAUMTotalDailyName             = "etf_aum_total_daily"
	ETFAUMVelocity30dName            = "etf_aum_velocity_30d"
	ETFAUMVelocityZScore90dName      = "etf_aum_velocity_zscore_90d"
	ETFConcentrationHHIDailyName     = "etf_concentration_hhi_daily"
	ETFConcentrationHHIZScore90dName = "etf_concentration_hhi_zscore_90d"
	OptionsDealerSkewProxyDailyName  = "options_dealer_skew_proxy_daily"
	OptionsDealerSkewProxyZScoreName = "options_dealer_skew_proxy_zscore_90d"
)

const (
	LTHChangeWindowDays = 30
	ETFZScoreWindowDays = 90
	ETFZScoreMinObs     = 30
	LTHZScoreWindowDays = 180
	LTHZScoreMinObs     = 60

	StablecoinVelocityWindowDays = 30
	LiquidityFlowZWindowDays     = 90
	LiquidityFlowZMinObs         = 30
)

func StablecoinMcapVelocity30d(now, past float64) (float64, bool) {
	if past == 0 || math.IsNaN(now) || math.IsNaN(past) || math.IsInf(now, 0) || math.IsInf(past, 0) {
		return 0, false
	}
	return (now - past) / 30.0, true
}

func ETFAUMVelocity30d(now, past float64) (float64, bool) {
	if past <= 0 || math.IsNaN(now) || math.IsNaN(past) || math.IsInf(now, 0) || math.IsInf(past, 0) {
		return 0, false
	}
	return (now - past) / 30.0, true
}

func OptionsDealerSkewProxy(callNotional, putNotional float64) (float64, bool) {
	denom := callNotional + putNotional
	if denom <= 0 || math.IsNaN(callNotional) || math.IsNaN(putNotional) {
		return 0, false
	}
	return (callNotional - putNotional) / denom, true
}

func BitfinexMarginSkew(long, short float64) (float64, bool) {
	const eps = 1.0
	if math.IsNaN(long) || math.IsNaN(short) || long < 0 || short < 0 {
		return 0, false
	}
	return math.Log((long + eps) / (short + eps)), true
}

func PctChange(now, past float64) (value float64, ok bool) {
	if past == 0 || math.IsNaN(now) || math.IsNaN(past) || math.IsInf(now, 0) || math.IsInf(past, 0) {
		return 0, false
	}
	return (now - past) / past, true
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
