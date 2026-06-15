package backtest

import "fmt"

type Label string

const (
	LabelRiskOn     Label = "risk_on"
	LabelRiskOff    Label = "risk_off"
	LabelTransition Label = "transition"
)

type ClassifierConfig struct {
	TransitionRiskHigh float64

	RiskOnFloor float64

	RiskOffCeiling float64
}

func DefaultClassifier() ClassifierConfig {
	return ClassifierConfig{
		TransitionRiskHigh: 0.65,
		RiskOnFloor:        0.30,
		RiskOffCeiling:     -0.30,
	}
}

func Classify(cfg ClassifierConfig, regimeIndicator, transitionRisk float64) Label {
	if transitionRisk >= cfg.TransitionRiskHigh {
		return LabelTransition
	}
	if regimeIndicator > cfg.RiskOnFloor {
		return LabelRiskOn
	}
	if regimeIndicator < cfg.RiskOffCeiling {
		return LabelRiskOff
	}
	return LabelTransition
}

type HysteresisConfig struct {
	TransitionEnter float64
	TransitionExit  float64
	RiskOnEnter     float64
	RiskOnExit      float64
	RiskOffEnter    float64
	RiskOffExit     float64
}

func DefaultHysteresis() HysteresisConfig {
	return HysteresisConfig{
		TransitionEnter: 0.86,
		TransitionExit:  0.80,
		RiskOnEnter:     0.20,
		RiskOnExit:      0.05,
		RiskOffEnter:    -0.20,
		RiskOffExit:     -0.05,
	}
}

func (c HysteresisConfig) Validate() error {
	if !(c.TransitionEnter > c.TransitionExit) {
		return fmt.Errorf("hysteresis: TransitionEnter (%.3f) must exceed TransitionExit (%.3f)", c.TransitionEnter, c.TransitionExit)
	}
	if !(c.RiskOnEnter > c.RiskOnExit) {
		return fmt.Errorf("hysteresis: RiskOnEnter (%.3f) must exceed RiskOnExit (%.3f)", c.RiskOnEnter, c.RiskOnExit)
	}
	if !(c.RiskOffEnter < c.RiskOffExit) {
		return fmt.Errorf("hysteresis: RiskOffEnter (%.3f) must be below RiskOffExit (%.3f)", c.RiskOffEnter, c.RiskOffExit)
	}
	return nil
}

func ClassifyHysteresis(cfg HysteresisConfig, prev Label, regimeIndicator, transitionRisk float64) Label {
	if transitionRisk >= cfg.TransitionEnter {
		return LabelTransition
	}

	switch prev {
	case LabelRiskOn:
		if regimeIndicator <= cfg.RiskOnExit {

			if regimeIndicator < cfg.RiskOffEnter {
				return LabelRiskOff
			}
			return LabelTransition
		}
		return LabelRiskOn
	case LabelRiskOff:
		if regimeIndicator >= cfg.RiskOffExit {
			if regimeIndicator > cfg.RiskOnEnter {
				return LabelRiskOn
			}
			return LabelTransition
		}
		return LabelRiskOff
	default:

		if transitionRisk < cfg.TransitionExit {
			if regimeIndicator > cfg.RiskOnEnter {
				return LabelRiskOn
			}
			if regimeIndicator < cfg.RiskOffEnter {
				return LabelRiskOff
			}
		}
		return LabelTransition
	}
}
