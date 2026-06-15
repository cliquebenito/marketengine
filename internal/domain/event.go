package domain

type Event struct {
	Topic       string
	AggregateID string
	Payload     map[string]any
}
