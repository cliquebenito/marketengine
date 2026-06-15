package features

import (
	"math"

	mmath "marketengine/pkg/math"
)

const (
	StablecoinSupplyTotalName     = "stablecoin_supply_total"
	StablecoinSupplyMajorName     = "stablecoin_supply_usdt_usdc_dai"
	StablecoinGrowth30dName       = "stablecoin_growth_30d"
	StablecoinGrowthZScore90dName = "stablecoin_growth_zscore_90d"
	SSRName                       = "ssr"
	SSRPercentileRank365dName     = "ssr_percentile_rank_365d"
	ExchangeNetflow7dName         = "exchange_netflow_7d"
	ExchangeNetflowZScore180dName = "exchange_netflow_zscore_180d"
	DefiTVLUsdName                = "defi_tvl_usd"
	DefiTVLGrowth30dName          = "defi_tvl_growth_30d"
	DefiTVLGrowthZScore180dName   = "defi_tvl_growth_zscore_180d"
)

const (
	GrowthWindowDays           = 30
	StablecoinZScoreWindowDays = 90
	StablecoinZScoreMinObs     = 30
	NetflowZScoreWindowDays    = 180
	NetflowZScoreMinObs        = 60
	TVLGrowthWindowDays        = 30
	TVLZScoreWindowDays        = 180
	TVLZScoreMinObs            = 60
	SSRPercentileWindowDays    = 365
	SSRPercentileMinObs        = 30
)

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

func CoinIDForAsset(asset string) string {
	switch asset {
	case "BTC":
		return "bitcoin"
	case "ETH":
		return "ethereum"
	}
	return ""
}
