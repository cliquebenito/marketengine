package backtest

import "testing"

func TestHysteresisValidate(t *testing.T) {
	ok := DefaultHysteresis()
	if err := ok.Validate(); err != nil {
		t.Fatalf("default config must validate: %v", err)
	}

	cases := []struct {
		name string
		cfg  HysteresisConfig
	}{
		{"transition-inverted", HysteresisConfig{TransitionEnter: 0.4, TransitionExit: 0.5, RiskOnEnter: 0.3, RiskOnExit: 0.15, RiskOffEnter: -0.3, RiskOffExit: -0.15}},
		{"riskon-inverted", HysteresisConfig{TransitionEnter: 0.65, TransitionExit: 0.5, RiskOnEnter: 0.1, RiskOnExit: 0.3, RiskOffEnter: -0.3, RiskOffExit: -0.15}},
		{"riskoff-inverted", HysteresisConfig{TransitionEnter: 0.65, TransitionExit: 0.5, RiskOnEnter: 0.3, RiskOnExit: 0.15, RiskOffEnter: -0.1, RiskOffExit: -0.3}},
	}
	for _, c := range cases {
		if err := c.cfg.Validate(); err == nil {
			t.Errorf("%s: expected Validate() to reject config, got nil", c.name)
		}
	}
}

func TestHysteresisStateMachine(t *testing.T) {

	cfg := HysteresisConfig{
		TransitionEnter: 0.65,
		TransitionExit:  0.50,
		RiskOnEnter:     0.30,
		RiskOnExit:      0.15,
		RiskOffEnter:    -0.30,
		RiskOffExit:     -0.15,
	}

	seq := []struct {
		ri, tr float64
		want   Label
		note   string
	}{
		{0.10, 0.10, LabelTransition, "warm-up: ri below enter, stay neutral"},
		{0.35, 0.10, LabelRiskOn, "cross RiskOnEnter, enter risk_on"},
		{0.20, 0.10, LabelRiskOn, "ri dips below enter but above exit → sticky"},
		{0.40, 0.10, LabelRiskOn, "recover, still risk_on"},
		{0.40, 0.70, LabelTransition, "tr spike overrides risk_on"},
		{0.05, 0.55, LabelTransition, "tr above exit → stay in transition"},
		{-0.35, 0.40, LabelRiskOff, "tr below exit + ri below RiskOffEnter → risk_off"},
		{-0.20, 0.40, LabelRiskOff, "ri above enter but below exit → sticky risk_off"},
		{0.00, 0.40, LabelTransition, "ri above RiskOffExit → leave, land in transition"},
		{0.00, 0.70, LabelTransition, "tr spike → transition"},
		{0.00, 0.55, LabelTransition, "tr still above exit → transition"},
		{0.40, 0.20, LabelRiskOn, "tr below exit + ri above RiskOnEnter → risk_on"},
	}

	prev := LabelTransition
	for i, s := range seq {
		got := ClassifyHysteresis(cfg, prev, s.ri, s.tr)
		if got != s.want {
			t.Errorf("step %d (%s): ri=%.2f tr=%.2f prev=%s → got %s, want %s",
				i, s.note, s.ri, s.tr, prev, got, s.want)
		}
		prev = got
	}
}

func TestHysteresisReducesFlipping(t *testing.T) {

	statelessCfg := ClassifierConfig{
		TransitionRiskHigh: 0.50,
		RiskOnFloor:        0.30,
		RiskOffCeiling:     -0.30,
	}
	hystCfg := HysteresisConfig{
		TransitionEnter: 0.65,
		TransitionExit:  0.50,
		RiskOnEnter:     0.30,
		RiskOnExit:      0.15,
		RiskOffEnter:    -0.30,
		RiskOffExit:     -0.15,
	}

	seq := []struct{ ri, tr float64 }{
		{0.35, 0.10}, {0.28, 0.10}, {0.33, 0.10}, {0.27, 0.10},
		{0.32, 0.10}, {0.25, 0.10}, {0.34, 0.10}, {0.22, 0.10},
		{0.31, 0.10}, {0.26, 0.10}, {0.33, 0.10}, {0.24, 0.10},
	}

	stateless := make([]Label, len(seq))
	for i, s := range seq {
		stateless[i] = Classify(statelessCfg, s.ri, s.tr)
	}

	hysteresis := make([]Label, len(seq))
	prev := LabelTransition
	for i, s := range seq {
		hysteresis[i] = ClassifyHysteresis(hystCfg, prev, s.ri, s.tr)
		prev = hysteresis[i]
	}

	changes := func(ls []Label) int {
		c := 0
		for i := 1; i < len(ls); i++ {
			if ls[i] != ls[i-1] {
				c++
			}
		}
		return c
	}

	statelessChanges := changes(stateless)
	hysteresisChanges := changes(hysteresis)
	if !(hysteresisChanges < statelessChanges) {
		t.Errorf("hysteresis must reduce label changes on this sequence: stateless=%d hysteresis=%d",
			statelessChanges, hysteresisChanges)
	}
}
