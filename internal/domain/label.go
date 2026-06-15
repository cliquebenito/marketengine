package domain

type Label string

const (
	LabelRiskOn     Label = "risk_on"
	LabelRiskOff    Label = "risk_off"
	LabelTransition Label = "transition"
)

func AllLabels() []Label { return []Label{LabelRiskOn, LabelRiskOff, LabelTransition} }
