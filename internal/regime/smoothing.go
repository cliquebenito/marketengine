package regime

func alphaFromSpan(span int) float64 {
	if span <= 1 {
		return 1.0
	}
	return 2.0 / (float64(span) + 1.0)
}

func emaSmooth(raw float64, prev []float64, span int) float64 {
	if span <= 1 {
		return raw
	}
	if len(prev) == 0 {
		return raw
	}
	alpha := alphaFromSpan(span)
	ema := prev[0]
	for i := 1; i < len(prev); i++ {
		ema = alpha*prev[i] + (1-alpha)*ema
	}
	return alpha*raw + (1-alpha)*ema
}
