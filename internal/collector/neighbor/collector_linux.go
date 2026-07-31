//go:build linux

package neighbor

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"syscall"
	"time"

	"github.com/nymanyim/openwrt2mqtt/internal/collector"
)

const (
	netlinkRoute         = 0
	rtmNewNeighbor       = 28
	rtmDelNeighbor       = 29
	rtmGetNeighbor       = 30
	rtmGroupNeighbor     = 4
	nudIncomplete        = 0x01
	nudReachable         = 0x02
	nudStale             = 0x04
	nudDelay             = 0x08
	nudProbe             = 0x10
	nudFailed            = 0x20
	nudNoARP             = 0x40
	nudPermanent         = 0x80
	ndaDestination       = 1
	ndaLinkAddress       = 2
	netlinkBufferSize    = 64 * 1024
	maxConcurrentProbes  = 8
	confirmationAttempts = 3
	confirmationTimeout  = 400 * time.Millisecond
	confirmationInterval = 100 * time.Millisecond
)

type deviceState struct {
	ip               net.IP
	mac              net.HardwareAddr
	data             map[string]any
	online           bool
	verified         bool
	reconnectPending bool
	failedSince      time.Time
}
type Collector struct {
	interfaceName, routerID string
	offlineTimeout          time.Duration
	detectOffline           bool
	probeInterval           time.Duration
}

func NewCollector(interfaceName, routerID string, offlineTimeout time.Duration, detectOffline bool) *Collector {
	return &Collector{interfaceName: interfaceName, routerID: routerID, offlineTimeout: offlineTimeout, detectOffline: detectOffline, probeInterval: time.Second}
}
func (c *Collector) Name() string { return "neighbor" }
func (c *Collector) Start(ctx context.Context, emitter collector.Emitter) error {
	if c.interfaceName == "" {
		return errors.New("neighbor interface must not be empty")
	}
	if c.routerID == "" {
		return errors.New("neighbor router ID must not be empty")
	}
	if emitter == nil {
		return errors.New("neighbor emitter must not be nil")
	}
	device, err := net.InterfaceByName(c.interfaceName)
	if err != nil {
		return fmt.Errorf("find neighbor interface %q: %w", c.interfaceName, err)
	}
	sourceIP, err := interfaceIPv4(device)
	if err != nil {
		return fmt.Errorf("find neighbor interface IPv4: %w", err)
	}
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_RAW, netlinkRoute)
	if err != nil {
		return err
	}
	defer syscall.Close(fd)
	if err = syscall.Bind(fd, &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK, Groups: rtmGroupNeighbor}); err != nil {
		return err
	}
	if err = syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &syscall.Timeval{Usec: 250000}); err != nil {
		return err
	}
	states := make(map[string]*deviceState)
	if err = c.loadSnapshot(fd, device.Index, states); err != nil {
		return err
	}
	buffer := make([]byte, netlinkBufferSize)
	nextProbe := time.Now().Add(c.probeInterval)
	for {
		length, _, readErr := syscall.Recvfrom(fd, buffer, 0)
		if readErr == nil {
			messages, parseErr := syscall.ParseNetlinkMessage(buffer[:length])
			if parseErr == nil {
				for _, message := range messages {
					if err := c.handleMessage(ctx, emitter, device.Index, states, message); err != nil {
						return err
					}
				}
			}
		} else if ctx.Err() != nil {
			return nil
		} else if !errors.Is(readErr, syscall.EINTR) && !errors.Is(readErr, syscall.EAGAIN) && !errors.Is(readErr, syscall.EWOULDBLOCK) {
			return readErr
		}
		if c.detectOffline && !time.Now().Before(nextProbe) {
			if err := c.probeDevices(ctx, emitter, device, sourceIP, states); err != nil {
				return err
			}
			now := time.Now()
			for !nextProbe.After(now) {
				nextProbe = nextProbe.Add(c.probeInterval)
			}
		}
	}
}
func (c *Collector) loadSnapshot(fd, interfaceIndex int, states map[string]*deviceState) error {
	request := make([]byte, syscall.NLMSG_HDRLEN+neighborHeaderSize)
	binary.NativeEndian.PutUint32(request[0:4], uint32(len(request)))
	binary.NativeEndian.PutUint16(request[4:6], rtmGetNeighbor)
	binary.NativeEndian.PutUint16(request[6:8], syscall.NLM_F_REQUEST|syscall.NLM_F_DUMP)
	request[syscall.NLMSG_HDRLEN] = syscall.AF_INET
	if err := syscall.Sendto(fd, request, 0, &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK}); err != nil {
		return err
	}
	buffer := make([]byte, netlinkBufferSize)
	for {
		n, _, err := syscall.Recvfrom(fd, buffer, 0)
		if err != nil {
			return err
		}
		messages, err := syscall.ParseNetlinkMessage(buffer[:n])
		if err != nil {
			continue
		}
		done := false
		for _, message := range messages {
			if message.Header.Type == syscall.NLMSG_DONE {
				done = true
				continue
			}
			observed := parseNeighbor(interfaceIndex, message)
			if observed != nil && observed.active {
				states[observed.mac.String()] = newDeviceState(c.interfaceName, observed, false)
			}
		}
		if done {
			return nil
		}
	}
}
func (c *Collector) handleMessage(ctx context.Context, emitter collector.Emitter, interfaceIndex int, states map[string]*deviceState, message syscall.NetlinkMessage) error {
	observed := parseNeighbor(interfaceIndex, message)
	if observed == nil || !observed.active {
		return nil
	}
	key := observed.mac.String()
	current := states[key]
	now := time.Now()
	if current == nil {
		current = newDeviceState(c.interfaceName, observed, true)
		states[key] = current
		return c.emit(ctx, emitter, current, "device.connected", now)
	}
	current.ip = observed.ip
	current.mac = observed.mac
	current.data = neighborData(c.interfaceName, observed.ip, observed.mac)
	if !current.online {
		current.reconnectPending = true
		return nil
	}
	current.failedSince = time.Time{}
	return nil
}
func (c *Collector) probeDevices(ctx context.Context, emitter collector.Emitter, device *net.Interface, sourceIP net.IP, states map[string]*deviceState) error {
	type probeResult struct {
		state   *deviceState
		online  bool
		started time.Time
		checked time.Time
	}

	results := make(chan probeResult, len(states))
	semaphore := make(chan struct{}, maxConcurrentProbes)
	pending := 0
	for _, state := range states {
		if !state.online && !state.reconnectPending {
			continue
		}
		pending++
		go func(state *deviceState) {
			semaphore <- struct{}{}
			started := time.Now()
			online := probeARP(device, sourceIP, state.ip, state.mac, 250*time.Millisecond)
			checked := time.Now()
			<-semaphore
			results <- probeResult{state: state, online: online, started: started, checked: checked}
		}(state)
	}

	for range pending {
		result := <-results
		eventType, confirmOffline := applyProbeResult(result.state, result.online, result.started, result.checked, c.probeInterval, c.offlineTimeout)
		if confirmOffline {
			if confirmARP(device, sourceIP, result.state.ip, result.state.mac) {
				result.state.failedSince = time.Time{}
				continue
			}
			result.state.online = false
			result.state.failedSince = time.Time{}
			eventType = "device.disconnected"
		}
		if eventType == "" {
			continue
		}
		if err := c.emit(ctx, emitter, result.state, eventType, time.Now()); err != nil {
			return err
		}
	}
	return nil
}

func applyProbeResult(state *deviceState, online bool, started, checked time.Time, probeInterval, offlineTimeout time.Duration) (string, bool) {
	if !state.verified {
		state.failedSince = time.Time{}
		if online {
			state.verified = true
		}
		return "", false
	}
	if online {
		state.failedSince = time.Time{}
		if !state.online && state.reconnectPending {
			state.online = true
			state.reconnectPending = false
			return "device.connected", false
		}
		return "", false
	}
	if !state.online {
		state.reconnectPending = false
		return "", false
	}
	if state.failedSince.IsZero() {
		state.failedSince = started.Add(-probeInterval)
	}
	return "", !checked.Before(state.failedSince.Add(offlineTimeout))
}

func confirmARP(device *net.Interface, sourceIP, targetIP net.IP, targetMAC net.HardwareAddr) bool {
	for attempt := 0; attempt < confirmationAttempts; attempt++ {
		if probeARP(device, sourceIP, targetIP, targetMAC, confirmationTimeout) {
			return true
		}
		if attempt+1 < confirmationAttempts {
			time.Sleep(confirmationInterval)
		}
	}
	return false
}

func (c *Collector) emit(ctx context.Context, emitter collector.Emitter, state *deviceState, eventType string, now time.Time) error {
	if err := emitter.Emit(ctx, newEvent(c.routerID, c.interfaceName, eventType, state.data, now)); err != nil {
		return fmt.Errorf("emit neighbor event: %w", err)
	}
	return nil
}
func newDeviceState(interfaceName string, observed *neighborObservation, verified bool) *deviceState {
	return &deviceState{ip: observed.ip, mac: observed.mac, data: neighborData(interfaceName, observed.ip, observed.mac), online: true, verified: verified}
}
func nativeUint16(value []byte) uint16 { return binary.NativeEndian.Uint16(value) }
