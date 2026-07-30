package memory

import (
	"context"

	"github.com/nymanyim/openwrt2mqtt/internal/event"
)

const defaultCapacity = 128

// Bus is a bounded, context-aware in-memory event queue.
type Bus struct {
	events chan event.Event
}

func New(capacity int) *Bus {
	if capacity <= 0 {
		capacity = defaultCapacity
	}
	return &Bus{events: make(chan event.Event, capacity)}
}

func (b *Bus) Publish(ctx context.Context, message event.Event) error {
	select {
	case b.events <- message:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *Bus) Events() <-chan event.Event {
	return b.events
}
