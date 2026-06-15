package math

import (
	"math"
	"testing"
)

func TestMean(t *testing.T) {
	if got := Mean([]float64{1, 2, 3}); got != 2 {
		t.Fatalf("Mean: want 2 got %v", got)
	}
	if !math.IsNaN(Mean(nil)) {
		t.Fatalf("Mean(nil): want NaN")
	}
}

func TestStdDev(t *testing.T) {

	got := StdDev([]float64{1, 2, 3, 4, 5})
	want := math.Sqrt(2.5)
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("StdDev: want %v got %v", want, got)
	}
	if !math.IsNaN(StdDev([]float64{1})) {
		t.Fatalf("StdDev of single point: want NaN")
	}
}

func TestZScore(t *testing.T) {
	xs := []float64{1, 2, 3, 4, 5}

	got := ZScore(5, xs)
	want := 2.0 / math.Sqrt(2.5)
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("ZScore: want %v got %v", want, got)
	}

	if !math.IsNaN(ZScore(1, []float64{1, 1, 1})) {
		t.Fatalf("ZScore constant: want NaN")
	}
}

func TestTanhSquash(t *testing.T) {
	if math.Abs(TanhSquash(0, 2)-0) > 1e-9 {
		t.Fatalf("TanhSquash(0): want 0")
	}

	if math.Abs(TanhSquash(2, 2)-math.Tanh(1)) > 1e-9 {
		t.Fatalf("TanhSquash(2,2): want tanh(1)")
	}
	if !math.IsNaN(TanhSquash(math.NaN(), 2)) {
		t.Fatalf("TanhSquash(NaN): want NaN")
	}
}
