package collector

import (
	"context"

	"github.com/nymanyim/openwrt2mqtt/internal/event"
)

// Emitter accepts events without exposing downstream transports.
type Emitter interface {
	Emit(context.Context, event.Event) error
}

// Collector observes one OpenWrt event source.
type Collector interface {
	Name() string
	Start(context.Context, Emitter) error
}
