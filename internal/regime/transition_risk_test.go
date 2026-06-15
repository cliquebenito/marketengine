package regime

import (
	"math"
	"testing"
)

func TestTransitionBaselineFormula(t *testing.T) {
	apply := func(rocZ, divZ, baseline, rocW, divW, discount float64) float64 {
		raw := rocW*math.Tanh(math.Abs(rocZ)/2) + divW*math.Tanh(math.Abs(divZ)/2)
		var shifted float64
		if baseline > 0 {
			shifted = (raw - baseline) / (1 - baseline)
		} else {
			shifted = raw
		}
		if shifted < 0 {
			shifted = 0
		}
		if shifted > 1 {
			shifted = 1
		}
		return discount * shifted
	}

	cases := []struct {
		name     string
		rocZ     float64
		divZ     float64
		baseline float64
		want     float64
		tol      float64
	}{
		{"zero z → raw=0 → below baseline → 0", 0, 0, 0.30, 0.0, 1e-9},
		{"large |z|=10 → raw→1 → ~1 after rescale", 10, 10, 0.30, 1.0, 1e-3},
		{

			name: "|z|=1 → sub-0.5",
			rocZ: 1, divZ: 1, baseline: 0.30,
			want: (0.4621171572600098 - 0.30) / 0.70,
			tol:  1e-6,
		},
		{

			name: "baseline=0 reproduces v1.1 saturated output",
			rocZ: 2, divZ: 2, baseline: 0,
			want: 0.7*math.Tanh(1) + 0.3*math.Tanh(1),
			tol:  1e-6,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := apply(c.rocZ, c.divZ, c.baseline, 0.7, 0.3, 1.0)
			if math.Abs(got-c.want) > c.tol {
				t.Fatalf("got %.6f want %.6f (tol %g)", got, c.want, c.tol)
			}
		})
	}
}

func TestTransitionBaselineClipsNegative(t *testing.T) {

	raw := 0.7*math.Tanh(0.25) + 0.3*math.Tanh(0.25)
	if raw >= 0.30 {
		t.Fatalf("test precondition violated: raw %.4f ≥ baseline 0.30", raw)
	}
	shifted := (raw - 0.30) / (1 - 0.30)
	if shifted >= 0 {
		t.Fatalf("expected shifted<0, got %.4f", shifted)
	}

}
