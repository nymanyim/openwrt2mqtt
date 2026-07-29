package publisher

import (
	"context"

	"github.com/nymanyim/openwrt2mqtt/internal/event"
)

// Publisher sends normalized events to an external transport.
type Publisher interface {
	Publish(context.Context, event.Event) error
	Close() error
}
