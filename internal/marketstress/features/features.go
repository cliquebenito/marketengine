package features

import (
	"math"

	mmath "marketengine/pkg/math"
)

const (
	BtcAltCorrelation30dName = "btc_alt_correlation_30d"
	PegDeviationDailyName    = "peg_deviation_daily"
	CoinbasePremiumAbsName   = "coinbase_premium_abs_daily"
	BasisInversionDepthName  = "basis_inversion_depth_daily"

	BtcAltCorrelationZScore180dName   = "btc_alt_correlation_zscore_180d"
	StablecoinPegStressScoreName      = "stablecoin_peg_stress_score"
	CoinbasePremiumAbsZScore90dName   = "coinbase_premium_abs_zscore_90d"
	BasisInversionDepthZScore180dName = "basis_inversion_depth_zscore_180d"

	BookImbalanceDailyName     = "book_imbalance_daily"
	BookImbalanceZScoreName    = "book_imbalance_zscore_90d"
	FuturesSpotRatioDailyName  = "futures_spot_ratio_daily"
	FuturesSpotRatioZScoreName = "futures_spot_ratio_zscore_90d"
)

const (
	CorrelationWindowDays = 30
	CorrelationMinPrices  = 20
	CorrelationMinReturns = 10

	CorrelationZScoreWindowDays     = 180
	CorrelationZScoreMinObs         = 30
	PegZScoreWindowDays             = 180
	PegZScoreMinObs                 = 30
	CoinbasePremiumZScoreWindowDays = 90
	CoinbasePremiumZScoreMinObs     = 30
	BasisInversionZScoreWindowDays  = 180
	BasisInversionZScoreMinObs      = 30

	MicrostructureZScoreWindowDays = 90
	MicrostructureZScoreMinObs     = 30
)

var AltSymbols = []string{"ETHUSDT", "SOLUSDT", "BNBUSDT", "XRPUSDT", "ADAUSDT", "DOGEUSDT"}

func LogReturns(prices []float64) []float64 {
	if len(prices) < 2 {
		return nil
	}
	out := make([]float64, 0, len(prices)-1)
	for i := 1; i < len(prices); i++ {
		if prices[i-1] <= 0 {
			continue
		}
		out = append(out, math.Log(prices[i]/prices[i-1]))
	}
	return out
}

func AvgBtcAltCorrelation(btcReturns []float64, altReturnsList [][]float64) (value float64, ok bool) {
	var sum float64
	count := 0
	for _, alt := range altReturnsList {
		n := len(btcReturns)
		if len(alt) < n {
			n = len(alt)
		}
		if n < CorrelationMinReturns {
			continue
		}
		bRet := btcReturns[len(btcReturns)-n:]
		aRet := alt[len(alt)-n:]
		c := mmath.Correlation(bRet, aRet)
		if math.IsNaN(c) {
			continue
		}
		sum += c
		count++
	}
	if count == 0 {
		return 0, false
	}
	return sum / float64(count), true
}

func PegDeviationScaled(usdt float64, usdtOk bool, usdc float64, usdcOk bool) (value float64, ok bool) {
	if !usdtOk && !usdcOk {
		return 0, false
	}
	var maxDev float64
	if usdtOk {
		if d := math.Abs(usdt - 1.0); d > maxDev {
			maxDev = d
		}
	}
	if usdcOk {
		if d := math.Abs(usdc - 1.0); d > maxDev {
			maxDev = d
		}
	}
	return math.Log1p(maxDev * 10000), true
}

func CoinbasePremiumAbsFromPrices(coinbaseClose, binanceClose float64) (value float64, ok bool) {
	if binanceClose <= 0 {
		return 0, false
	}
	return math.Abs(coinbaseClose-binanceClose) / binanceClose, true
}

func BasisInversionDepth(basis float64) float64 {
	return math.Max(0, -basis)
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
