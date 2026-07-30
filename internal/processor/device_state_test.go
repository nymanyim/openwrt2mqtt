package processor

import (
	"context"
	"testing"

	"github.com/nymanyim/openwrt2mqtt/internal/event"
)

func TestDeviceStateSuppressesRepeatedState(t *testing.T) {
	processor := NewDeviceState()
	connected := event.Event{Type: "device.connected", Data: map[string]any{"mac": "AA:BB:CC:DD:EE:FF"}}
	if _, keep, _ := processor.Process(context.Background(), connected); !keep {
		t.Fatal("first connected event was suppressed")
	}
	if _, keep, _ := processor.Process(context.Background(), connected); keep {
		t.Fatal("duplicate connected event was retained")
	}
	disconnected := event.Event{Type: "device.disconnected", Data: connected.Data}
	if _, keep, _ := processor.Process(context.Background(), disconnected); !keep {
		t.Fatal("state transition was suppressed")
	}
}
