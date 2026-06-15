package pgoutbox

import (
	"context"

	"github.com/jackc/pgx/v5"

	"marketengine/internal/domain"
	"marketengine/internal/outbox"
	"marketengine/internal/storage"
)

type Publisher struct{ pool *storage.Pool }

func New(pool *storage.Pool) *Publisher { return &Publisher{pool: pool} }

func (p *Publisher) Publish(ctx context.Context, ev domain.Event) error {
	return p.pool.InTx(ctx, func(tx pgx.Tx) error {
		return outbox.Enqueue(ctx, tx, outbox.Event{
			Topic:       ev.Topic,
			AggregateID: ev.AggregateID,
			Payload:     ev.Payload,
		})
	})
}
