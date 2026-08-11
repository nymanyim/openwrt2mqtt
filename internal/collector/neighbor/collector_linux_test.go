//go:build linux

package neighbor

import (
	"net"
	"testing"
	"time"
)

func testTracker(timeout time.Duration) (*presenceTracker, *deviceState) {
	mac, _ := net.ParseMAC("02:00:00:00:00:01")
	state := &deviceState{ip: net.IPv4(192, 0, 2, 10), mac: mac, online: true, verified: true, lastSeen: time.Unix(100, 0)}
	return newPresenceTracker("br-lan", timeout, map[string]*deviceState{mac.String(): state}), state
}

func probeFor(state *deviceState, online bool, started, checked time.Time) probeResult {
	return probeResult{probeRequest: probeRequest{key: state.mac.String(), ip: append(net.IP(nil), state.ip...), mac: append(net.HardwareAddr(nil), state.mac...), generation: state.generation, probeID: state.probeID}, online: online, started: started, checked: checked}
}

func TestOfflineTimeoutStartsAtFirstActualFailure(t *testing.T) {
	tracker, state := testTracker(3 * time.Second)
	started := time.Unix(101, 0)
	if eventType, _ := tracker.applyProbe(probeFor(state, false, started, started.Add(250*time.Millisecond))); eventType != "" {
		t.Fatalf("first failure emitted %q", eventType)
	}
	if !state.failedSince.Equal(started) {
		t.Fatalf("failedSince = %v, want %v", state.failedSince, started)
	}
	if eventType, _ := tracker.applyProbe(probeFor(state, false, started.Add(2*time.Second), started.Add(2250*time.Millisecond))); eventType != "" {
		t.Fatalf("failure before timeout emitted %q", eventType)
	}
}

func TestContinuousFailureEmitsOneDisconnect(t *testing.T) {
	tracker, state := testTracker(3 * time.Second)
	started := time.Unix(101, 0)
	tracker.applyProbe(probeFor(state, false, started, started.Add(probeWindow)))
	eventType, _ := tracker.applyProbe(probeFor(state, false, started.Add(3*time.Second), started.Add(3*time.Second+probeWindow)))
	if eventType != "device.disconnected" {
		t.Fatalf("event = %q", eventType)
	}
	if eventType, _ = tracker.applyProbe(probeFor(state, false, started.Add(4*time.Second), started.Add(4*time.Second+probeWindow))); eventType != "" {
		t.Fatalf("repeated failure emitted %q", eventType)
	}
}

func TestThreeSecondTimeoutHasBoundedDetectionLatency(t *testing.T) {
	const timeout = 3 * time.Second
	if maximum := time.Second + timeout + probeWindow; maximum >= 5*time.Second {
		t.Fatalf("maximum detection latency = %v", maximum)
	}
	if typical := timeout + probeWindow; typical >= 4*time.Second {
		t.Fatalf("typical detection latency = %v", typical)
	}
}

func TestTrafficInvalidatesPendingProbe(t *testing.T) {
	tracker, state := testTracker(3 * time.Second)
	request := probeFor(state, false, time.Unix(101, 0), time.Unix(105, 0))
	if eventType, _ := tracker.observeTraffic(state.mac, time.Unix(102, 0)); eventType != "" {
		t.Fatalf("traffic emitted %q", eventType)
	}
	if eventType, _ := tracker.applyProbe(request); eventType != "" {
		t.Fatalf("stale probe emitted %q", eventType)
	}
	if !state.online || !state.failedSince.IsZero() {
		t.Fatalf("unexpected state: %#v", state)
	}
}

func TestTrafficTimelineSuppressesFalseDisconnectAndReconnect(t *testing.T) {
	tracker, state := testTracker(3 * time.Second)
	started := time.Unix(101, 0)
	tracker.applyProbe(probeFor(state, false, started, started.Add(time.Second)))
	pending := probeFor(state, false, started.Add(3*time.Second), started.Add(4*time.Second))
	if eventType, _ := tracker.observeTraffic(state.mac, started.Add(3500*time.Millisecond)); eventType != "" {
		t.Fatalf("recovery traffic emitted %q", eventType)
	}
	if eventType, _ := tracker.applyProbe(pending); eventType != "" {
		t.Fatalf("stale final probe emitted %q", eventType)
	}
	if !state.online || !state.failedSince.IsZero() {
		t.Fatalf("unexpected final state: %#v", state)
	}
}

func TestNeighborEvidenceInvalidatesPendingProbe(t *testing.T) {
	tracker, state := testTracker(3 * time.Second)
	request := probeFor(state, false, time.Unix(101, 0), time.Unix(105, 0))
	observation := &neighborObservation{ip: state.ip, mac: state.mac, active: true}
	if eventType, _ := tracker.observeNeighbor(observation, time.Unix(102, 0)); eventType != "" {
		t.Fatalf("neighbor evidence emitted %q", eventType)
	}
	if eventType, _ := tracker.applyProbe(request); eventType != "" {
		t.Fatalf("stale probe emitted %q", eventType)
	}
}

func TestProbeSuccessClearsFailure(t *testing.T) {
	tracker, state := testTracker(3 * time.Second)
	started := time.Unix(101, 0)
	tracker.applyProbe(probeFor(state, false, started, started.Add(250*time.Millisecond)))
	if eventType, _ := tracker.applyProbe(probeFor(state, true, started.Add(time.Second), started.Add(1250*time.Millisecond))); eventType != "" {
		t.Fatalf("success emitted %q", eventType)
	}
	if !state.failedSince.IsZero() {
		t.Fatalf("failedSince was not cleared: %v", state.failedSince)
	}
}

func TestStartupBaselineRequiresPositiveEvidence(t *testing.T) {
	mac, _ := net.ParseMAC("02:00:00:00:00:01")
	state := &deviceState{ip: net.IPv4(192, 0, 2, 10), mac: mac, online: true}
	tracker := newPresenceTracker("br-lan", 3*time.Second, map[string]*deviceState{mac.String(): state})
	checked := time.Unix(101, 0)
	if eventType, _ := tracker.applyProbe(probeFor(state, false, checked, checked.Add(time.Second))); eventType != "" || state.verified {
		t.Fatalf("startup failure changed baseline: event=%q state=%#v", eventType, state)
	}
	if eventType, _ := tracker.applyProbe(probeFor(state, true, checked.Add(time.Second), checked.Add(2*time.Second))); eventType != "" || !state.verified {
		t.Fatalf("startup success failed baseline verification: event=%q state=%#v", eventType, state)
	}
}

func TestReconnectRequiresPositiveEvidence(t *testing.T) {
	tracker, state := testTracker(3 * time.Second)
	state.online = false
	state.reconnectPending = true
	checked := time.Unix(101, 0)
	if eventType, _ := tracker.applyProbe(probeFor(state, false, checked, checked.Add(time.Second))); eventType != "" || state.online {
		t.Fatalf("failed reconnect changed state: event=%q state=%#v", eventType, state)
	}
	state.reconnectPending = true
	if eventType, _ := tracker.applyProbe(probeFor(state, true, checked.Add(time.Second), checked.Add(2*time.Second))); eventType != "device.connected" || !state.online {
		t.Fatalf("successful reconnect: event=%q state=%#v", eventType, state)
	}
}

func TestOnlyOneProbePerDevice(t *testing.T) {
	tracker, state := testTracker(3 * time.Second)
	first := tracker.beginProbes()
	if got := len(first); got != 1 {
		t.Fatalf("first probe count = %d", got)
	}
	if got := len(tracker.beginProbes()); got != 0 {
		t.Fatalf("duplicate probe count = %d", got)
	}
	state.probePending = false
	second := tracker.beginProbes()
	if got := len(second); got != 1 {
		t.Fatalf("next probe count = %d", got)
	}
	if first[0].probeID == second[0].probeID {
		t.Fatal("probe ID was reused")
	}
	if eventType, _ := tracker.applyProbe(probeResult{probeRequest: first[0], online: false, started: time.Unix(101, 0), checked: time.Unix(105, 0)}); eventType != "" || !state.probePending {
		t.Fatalf("stale probe changed current probe: event=%q state=%#v", eventType, state)
	}
}

func TestTrafficParserAcceptsPresenceProtocols(t *testing.T) {
	local, _ := net.ParseMAC("02:00:00:00:00:ff")
	source, _ := net.ParseMAC("02:00:00:00:00:01")
	for _, etherType := range []uint16{etherTypeARP, etherTypeIPv4, etherTypeIPv6} {
		frame := make([]byte, ethernetHeaderSize)
		copy(frame[6:12], source)
		frame[12], frame[13] = byte(etherType>>8), byte(etherType)
		if got := sourceMAC(frame, local); got == nil || got.String() != source.String() {
			t.Fatalf("ether type %#x returned %v", etherType, got)
		}
	}
}

func TestTrafficParserRejectsInvalidSources(t *testing.T) {
	local, _ := net.ParseMAC("02:00:00:00:00:ff")
	for _, source := range []string{"00:00:00:00:00:00", "01:00:5e:00:00:16", local.String()} {
		mac, _ := net.ParseMAC(source)
		frame := make([]byte, ethernetHeaderSize)
		copy(frame[6:12], mac)
		frame[12], frame[13] = 0x08, 0x00
		if got := sourceMAC(frame, local); got != nil {
			t.Fatalf("source %s returned %v", source, got)
		}
	}
}
