package processor

import (
	"context"
	"strings"
	"sync"

	"github.com/nymanyim/openwrt2mqtt/internal/event"
)

// DeviceState suppresses repeated device state events from multiple collectors.
type DeviceState struct {
	mu     sync.Mutex
	states map[string]string
}

func NewDeviceState() *DeviceState {
	return &DeviceState{states: make(map[string]string)}
}

func (p *DeviceState) Process(_ context.Context, message event.Event) (event.Event, bool, error) {
	if message.Type != "device.connected" && message.Type != "device.disconnected" {
		return message, true, nil
	}
	mac, _ := message.Data["mac"].(string)
	mac = strings.ToLower(strings.TrimSpace(mac))
	if mac == "" {
		return message, true, nil
	}
	state := strings.TrimPrefix(message.Type, "device.")
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.states[mac] == state {
		return message, false, nil
	}
	p.states[mac] = state
	return message, true, nil
}
