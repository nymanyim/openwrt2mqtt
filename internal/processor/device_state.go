package processor

import (
	"context"
	"github.com/nymanyim/openwrt2mqtt/internal/event"
	"strings"
	"sync"
)

type DeviceState struct {
	mu                  sync.Mutex
	states              map[string]string
	connectedEnabled    bool
	disconnectedEnabled bool
}

func NewDeviceState(enabled ...bool) *DeviceState {
	connected, disconnected := true, true
	if len(enabled) > 0 {
		connected = enabled[0]
	}
	if len(enabled) > 1 {
		disconnected = enabled[1]
	}
	return &DeviceState{states: make(map[string]string), connectedEnabled: connected, disconnectedEnabled: disconnected}
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
	if p.states[mac] == state {
		p.mu.Unlock()
		return message, false, nil
	}
	p.states[mac] = state
	p.mu.Unlock()
	if state == "connected" {
		return message, p.connectedEnabled, nil
	}
	return message, p.disconnectedEnabled, nil
}
