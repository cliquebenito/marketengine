package math

import (
	"math"
)

func BSGamma(spot, strike, ivPct float64, daysToExpiry int, rPct float64) float64 {
	if daysToExpiry <= 0 || spot <= 0 || strike <= 0 || ivPct <= 0 {
		return 0
	}
	if math.IsNaN(spot) || math.IsNaN(strike) || math.IsNaN(ivPct) {
		return 0
	}
	sigma := ivPct / 100.0
	r := rPct / 100.0
	t := float64(daysToExpiry) / 365.0
	sqrtT := math.Sqrt(t)
	d1 := (math.Log(spot/strike) + (r+sigma*sigma/2)*t) / (sigma * sqrtT)
	if math.IsNaN(d1) || math.IsInf(d1, 0) {
		return 0
	}

	pdf := math.Exp(-d1*d1/2) / math.Sqrt(2*math.Pi)
	g := pdf / (spot * sigma * sqrtT)
	if math.IsNaN(g) || math.IsInf(g, 0) {
		return 0
	}
	return g
}
