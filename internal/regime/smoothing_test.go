package regime

import (
	"math"
	"testing"
)

func TestEMASmoothSpan3(t *testing.T) {
	prev := []float64{0.10, 0.20, 0.30, 0.40}
	got := emaSmooth(0.50, prev, 3)
	want := 0.40625
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("emaSmooth span=3: got %.12f, want %.12f (diff %.2e)",
			got, want, math.Abs(got-want))
	}
}

func TestEMASmoothSpan3EmptyPrev(t *testing.T) {
	got := emaSmooth(0.75, nil, 3)
	if got != 0.75 {
		t.Fatalf("empty-prev warm-up: got %v, want 0.75", got)
	}
}

func TestEMASmoothSpanZero(t *testing.T) {
	prev := []float64{0.1, 0.2, 0.3}
	got := emaSmooth(0.9, prev, 0)
	if got != 0.9 {
		t.Fatalf("span=0 must be identity: got %v, want 0.9", got)
	}
}

func TestEMASmoothSpanOne(t *testing.T) {
	prev := []float64{0.1, 0.2, 0.3}
	got := emaSmooth(-0.4, prev, 1)
	if got != -0.4 {
		t.Fatalf("span=1 must be identity: got %v, want -0.4", got)
	}
}

func TestAlphaFromSpan(t *testing.T) {
	cases := []struct {
		span int
		want float64
	}{
		{2, 2.0 / 3.0},
		{3, 0.5},
		{9, 0.2},
		{21, 2.0 / 22.0},
	}
	for _, c := range cases {
		got := alphaFromSpan(c.span)
		if math.Abs(got-c.want) > 1e-12 {
			t.Errorf("alpha(span=%d): got %.12f, want %.12f", c.span, got, c.want)
		}
	}
	if alphaFromSpan(0) != 1.0 || alphaFromSpan(1) != 1.0 {
		t.Errorf("alpha for span<=1 must be 1.0")
	}
}
