package bus

import (
	"context"

	"github.com/nymanyim/openwrt2mqtt/internal/event"
)

// Bus decouples event producers from consumers.
type Bus interface {
	Publish(context.Context, event.Event) error
}
