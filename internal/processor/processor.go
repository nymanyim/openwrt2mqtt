package processor

import (
	"context"

	"github.com/nymanyim/openwrt2mqtt/internal/event"
)

// Processor transforms or drops an event before publication.
type Processor interface {
	Process(context.Context, event.Event) (event.Event, bool, error)
}
