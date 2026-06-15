package math

import "math"

func Mean(xs []float64) float64 {
	if len(xs) == 0 {
		return math.NaN()
	}
	var s float64
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

func StdDev(xs []float64) float64 {
	if len(xs) < 2 {
		return math.NaN()
	}
	m := Mean(xs)
	var s float64
	for _, x := range xs {
		d := x - m
		s += d * d
	}
	return math.Sqrt(s / float64(len(xs)-1))
}

func ZScore(x float64, xs []float64) float64 {
	sd := StdDev(xs)
	if math.IsNaN(sd) || sd == 0 {
		return math.NaN()
	}
	return (x - Mean(xs)) / sd
}

func Correlation(xs, ys []float64) float64 {
	n := len(xs)
	if n < 2 || n != len(ys) {
		return math.NaN()
	}
	mx := Mean(xs)
	my := Mean(ys)
	var sxy, sxx, syy float64
	for i := 0; i < n; i++ {
		dx := xs[i] - mx
		dy := ys[i] - my
		sxy += dx * dy
		sxx += dx * dx
		syy += dy * dy
	}
	if sxx == 0 || syy == 0 {
		return math.NaN()
	}
	return sxy / math.Sqrt(sxx*syy)
}

func TanhSquash(z, scale float64) float64 {
	if math.IsNaN(z) {
		return math.NaN()
	}
	return math.Tanh(z / scale)
}
