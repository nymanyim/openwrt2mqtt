//go:build linux

package neighbor

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sort"
	"syscall"
	"time"

	"github.com/nymanyim/openwrt2mqtt/internal/collector"
)

const (
	netlinkRoute        = 0
	rtmNewNeighbor      = 28
	rtmDelNeighbor      = 29
	rtmGetNeighbor      = 30
	rtmGroupNeighbor    = 4
	nudIncomplete       = 0x01
	nudReachable        = 0x02
	nudStale            = 0x04
	nudDelay            = 0x08
	nudProbe            = 0x10
	nudFailed           = 0x20
	nudNoARP            = 0x40
	nudPermanent        = 0x80
	ndaDestination      = 1
	ndaLinkAddress      = 2
	netlinkBufferSize   = 64 * 1024
	maxConcurrentProbes = 8
	probeAttempts       = 5
	probeWindow         = time.Second
	probeInterval       = 150 * time.Millisecond
)

type deviceState struct {
	ip               net.IP
	mac              net.HardwareAddr
	data             map[string]any
	online           bool
	verified         bool
	reconnectPending bool
	lastSeen         time.Time
	offlineDeadline  time.Time
	disconnectedAt   time.Time
	generation       uint64
	probePending     bool
	probeID          uint64
}

type probeRequest struct {
	key        string
	ip         net.IP
	mac        net.HardwareAddr
	generation uint64
	probeID    uint64
	deadline   time.Time
}

type probeResult struct {
	probeRequest
	online  bool
	started time.Time
	checked time.Time
}

type presenceTracker struct {
	interfaceName  string
	offlineTimeout time.Duration
	states         map[string]*deviceState
}

func newPresenceTracker(interfaceName string, offlineTimeout time.Duration, states map[string]*deviceState) *presenceTracker {
	return &presenceTracker{interfaceName: interfaceName, offlineTimeout: offlineTimeout, states: states}
}

func (t *presenceTracker) observeNeighbor(observed *neighborObservation, now time.Time) (string, *deviceState) {
	key := observed.mac.String()
	state := t.states[key]
	if state == nil {
		state = newDeviceState(t.interfaceName, observed, true, now)
		t.states[key] = state
		return "device.connected", state
	}
	state.ip = observed.ip
	state.mac = observed.mac
	state.data = neighborData(t.interfaceName, observed.ip, observed.mac)
	if !observed.confirmed {
		return "", state
	}
	if !state.verified {
		state.verified = true
		t.markSeen(state, now)
		return "", state
	}
	if !state.online {
		state.online = true
		state.reconnectPending = false
		t.markSeen(state, now)
		return "device.connected", state
	}
	t.markSeen(state, now)
	return "", state
}

func (t *presenceTracker) observeTraffic(mac net.HardwareAddr, observedAt time.Time) (string, *deviceState) {
	state := t.states[mac.String()]
	if state == nil {
		return "", nil
	}
	if observedAt.Before(state.lastSeen) || (!state.online && !observedAt.After(state.disconnectedAt)) {
		return "", state
	}
	if !state.verified {
		state.verified = true
		t.markSeen(state, observedAt)
		return "", state
	}
	if !state.online {
		state.online = true
		state.reconnectPending = false
		t.markSeen(state, observedAt)
		return "device.connected", state
	}
	t.markSeen(state, observedAt)
	return "", state
}

func (t *presenceTracker) beginProbes(now time.Time, limit int) []probeRequest {
	if limit <= 0 {
		return nil
	}
	requests := make([]probeRequest, 0, len(t.states))
	for key, state := range t.states {
		if state.probePending || (!state.online && !state.reconnectPending) {
			continue
		}
		deadline := state.lastSeen.Add(t.offlineTimeout)
		if deadline.After(now.Add(probeWindow)) {
			continue
		}
		requests = append(requests, probeRequest{
			key:        key,
			ip:         append(net.IP(nil), state.ip...),
			mac:        append(net.HardwareAddr(nil), state.mac...),
			generation: state.generation,
			deadline:   deadline,
		})
	}
	sort.Slice(requests, func(i, j int) bool { return requests[i].deadline.Before(requests[j].deadline) })
	if len(requests) > limit {
		requests = requests[:limit]
	}
	for index := range requests {
		state := t.states[requests[index].key]
		state.probePending = true
		state.probeID++
		requests[index].probeID = state.probeID
	}
	return requests
}

func (t *presenceTracker) applyProbe(result probeResult) (string, *deviceState) {
	state := t.states[result.key]
	if state == nil {
		return "", nil
	}
	if state.probeID == result.probeID {
		state.probePending = false
	}
	if state.generation != result.generation || state.probeID != result.probeID || !state.ip.Equal(result.ip) || state.mac.String() != result.mac.String() {
		return "", state
	}
	if !state.verified {
		if result.online {
			state.verified = true
			t.markSeen(state, result.checked)
		}
		return "", state
	}
	if result.online {
		if !state.online {
			state.online = true
			state.reconnectPending = false
			t.markSeen(state, result.checked)
			return "device.connected", state
		}
		t.markSeen(state, result.checked)
		return "", state
	}
	if !state.online {
		state.reconnectPending = false
		return "", state
	}
	if state.offlineDeadline.IsZero() {
		state.offlineDeadline = result.deadline
	}
	if result.checked.Before(state.offlineDeadline) {
		return "", state
	}
	state.online = false
	state.reconnectPending = false
	state.offlineDeadline = time.Time{}
	state.disconnectedAt = result.checked
	state.generation++
	return "device.disconnected", state
}

func (t *presenceTracker) markSeen(state *deviceState, now time.Time) {
	if now.After(state.lastSeen) {
		state.lastSeen = now
	}
	state.offlineDeadline = time.Time{}
	state.generation++
	state.probeID++
	state.probePending = false
}

type Collector struct {
	interfaceName, routerID string
	offlineTimeout          time.Duration
	detectOffline           bool
	pollInterval            time.Duration
}

func NewCollector(interfaceName, routerID string, offlineTimeout time.Duration, detectOffline bool) *Collector {
	return &Collector{interfaceName: interfaceName, routerID: routerID, offlineTimeout: offlineTimeout, detectOffline: detectOffline, pollInterval: time.Second}
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
	var sourceIP net.IP
	if c.detectOffline {
		sourceIP, err = interfaceIPv4(device)
		if err != nil {
			return fmt.Errorf("find neighbor interface IPv4: %w", err)
		}
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
	tracker := newPresenceTracker(c.interfaceName, c.offlineTimeout, states)
	traffic := make(chan trafficObservation, 256)
	var trafficErrors <-chan error
	if c.detectOffline {
		errors := make(chan error, 1)
		trafficErrors = errors
		go func() { errors <- observeTraffic(ctx, device, traffic) }()
	}
	probeResults := make(chan probeResult, maxConcurrentProbes*2)
	probeSlots := make(chan struct{}, maxConcurrentProbes)
	activeProbes := 0
	ticker := time.NewTicker(c.pollInterval)
	defer ticker.Stop()
	buffer := make([]byte, netlinkBufferSize)

	emitEvent := func(eventType string, state *deviceState, now time.Time) error {
		if eventType == "" || state == nil {
			return nil
		}
		return c.emit(ctx, emitter, state, eventType, now)
	}
	processTraffic := func(observed trafficObservation) error {
		eventType, state := tracker.observeTraffic(observed.mac, observed.observedAt)
		return emitEvent(eventType, state, time.Now())
	}
	processAsync := func() error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case err := <-trafficErrors:
				if err != nil && ctx.Err() == nil {
					return fmt.Errorf("observe neighbor traffic: %w", err)
				}
				trafficErrors = nil
			case observed := <-traffic:
				if err := processTraffic(observed); err != nil {
					return err
				}
			case result := <-probeResults:
				activeProbes--
				draining := true
				for draining {
					select {
					case observed := <-traffic:
						if err := processTraffic(observed); err != nil {
							return err
						}
					default:
						draining = false
					}
				}
				eventType, state := tracker.applyProbe(result)
				if err := emitEvent(eventType, state, time.Now()); err != nil {
					return err
				}
			case <-ticker.C:
				if c.detectOffline {
					for _, request := range tracker.beginProbes(time.Now(), maxConcurrentProbes-activeProbes) {
						request := request
						activeProbes++
						go runProbe(ctx, device, sourceIP, request, probeSlots, probeResults)
					}
				}
			default:
				return nil
			}
		}
	}

	for {
		if err := processAsync(); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}

		length, _, readErr := syscall.Recvfrom(fd, buffer, 0)
		if readErr == nil {
			messages, parseErr := syscall.ParseNetlinkMessage(buffer[:length])
			if parseErr == nil {
				for _, message := range messages {
					observed := parseNeighbor(device.Index, message)
					if observed == nil || !observed.active {
						continue
					}
					eventType, state := tracker.observeNeighbor(observed, time.Now())
					if eventType != "" {
						if err := c.emit(ctx, emitter, state, eventType, time.Now()); err != nil {
							return err
						}
					}
				}
			}
		} else if ctx.Err() != nil {
			return nil
		} else if !errors.Is(readErr, syscall.EINTR) && !errors.Is(readErr, syscall.EAGAIN) && !errors.Is(readErr, syscall.EWOULDBLOCK) {
			return readErr
		}

		if err := processAsync(); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}
	}
}

func runProbe(ctx context.Context, device *net.Interface, sourceIP net.IP, request probeRequest, slots chan struct{}, results chan<- probeResult) {
	select {
	case slots <- struct{}{}:
	case <-ctx.Done():
		return
	}
	started := time.Now()
	deadline := request.deadline
	if !deadline.After(started) {
		deadline = started.Add(time.Microsecond)
	}
	online := probeARPAttempts(device, sourceIP, request.ip, request.mac, deadline, probeAttempts, probeInterval)
	checked := time.Now()
	<-slots
	select {
	case results <- probeResult{probeRequest: request, online: online, started: started, checked: checked}:
	case <-ctx.Done():
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
				states[observed.mac.String()] = newDeviceState(c.interfaceName, observed, false, time.Now())
			}
		}
		if done {
			return nil
		}
	}
}

func (c *Collector) emit(ctx context.Context, emitter collector.Emitter, state *deviceState, eventType string, now time.Time) error {
	if err := emitter.Emit(ctx, newEvent(c.routerID, c.interfaceName, eventType, state.data, now)); err != nil {
		return fmt.Errorf("emit neighbor event: %w", err)
	}
	return nil
}

func newDeviceState(interfaceName string, observed *neighborObservation, verified bool, now time.Time) *deviceState {
	return &deviceState{
		ip:       observed.ip,
		mac:      observed.mac,
		data:     neighborData(interfaceName, observed.ip, observed.mac),
		online:   true,
		verified: verified,
		lastSeen: now,
	}
}

func nativeUint16(value []byte) uint16 { return binary.NativeEndian.Uint16(value) }
