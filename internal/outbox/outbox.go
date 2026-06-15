package outbox

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type Event struct {
	Topic       string
	AggregateID string
	Payload     map[string]any
}

func Enqueue(ctx context.Context, tx pgx.Tx, e Event) error {
	payload, err := json.Marshal(e.Payload)
	if err != nil {
		return fmt.Errorf("marshal outbox payload: %w", err)
	}
	_, err = tx.Exec(ctx, `
INSERT INTO outbox_events (topic, aggregate_id, payload)
VALUES ($1, $2, $3)`, e.Topic, e.AggregateID, payload)
	return err
}
