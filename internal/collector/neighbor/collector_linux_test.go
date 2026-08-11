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
	return probeResult{probeRequest: probeRequest{key: state.mac.String(), ip: append(net.IP(nil), state.ip...), mac: append(net.HardwareAddr(nil), state.mac...), generation: state.generation, probeID: state.probeID, deadline: state.lastSeen.Add(5 * time.Second)}, online: online, started: started, checked: checked}
}

func TestOfflineDeadlineUsesLastConfirmedEvidence(t *testing.T) {
	tracker, state := testTracker(5 * time.Second)
	started := state.lastSeen.Add(time.Second)
	if eventType, _ := tracker.applyProbe(probeFor(state, false, started, started.Add(probeWindow))); eventType != "" {
		t.Fatalf("first failure emitted %q", eventType)
	}
	want := state.lastSeen.Add(5 * time.Second)
	if !state.offlineDeadline.Equal(want) {
		t.Fatalf("offlineDeadline = %v, want %v", state.offlineDeadline, want)
	}
}

func TestFailureBeforeDeadlineDoesNotDisconnect(t *testing.T) {
	tracker, state := testTracker(5 * time.Second)
	checked := state.lastSeen.Add(5*time.Second - time.Millisecond)
	if eventType, _ := tracker.applyProbe(probeFor(state, false, checked.Add(-probeWindow), checked)); eventType != "" {
		t.Fatalf("failure before deadline emitted %q", eventType)
	}
	if !state.online {
		t.Fatal("device disconnected before deadline")
	}
}

func TestFailureAtDeadlineDisconnectsOnce(t *testing.T) {
	tracker, state := testTracker(5 * time.Second)
	deadline := state.lastSeen.Add(5 * time.Second)
	eventType, _ := tracker.applyProbe(probeFor(state, false, deadline.Add(-probeWindow), deadline))
	if eventType != "device.disconnected" {
		t.Fatalf("event = %q", eventType)
	}
	if eventType, _ = tracker.applyProbe(probeFor(state, false, deadline, deadline.Add(probeWindow))); eventType != "" {
		t.Fatalf("repeated failure emitted %q", eventType)
	}
}

func TestFiveSecondTimeoutIncludesFinalProbeWindow(t *testing.T) {
	const timeout = 5 * time.Second
	finalProbeStart := timeout - probeWindow
	if finalProbeStart != 4*time.Second {
		t.Fatalf("final probe starts at %v", finalProbeStart)
	}
	if finalProbeStart+probeWindow != timeout {
		t.Fatalf("final probe ends at %v", finalProbeStart+probeWindow)
	}
}

func TestProbeSchedulingWaitsForFinalWindow(t *testing.T) {
	tracker, state := testTracker(5 * time.Second)
	if got := tracker.beginProbes(state.lastSeen.Add(3*time.Second), maxConcurrentProbes); len(got) != 0 {
		t.Fatalf("early probe count = %d", len(got))
	}
	got := tracker.beginProbes(state.lastSeen.Add(4*time.Second), maxConcurrentProbes)
	if len(got) != 1 {
		t.Fatalf("final-window probe count = %d", len(got))
	}
	if want := state.lastSeen.Add(5 * time.Second); !got[0].deadline.Equal(want) {
		t.Fatalf("deadline = %v, want %v", got[0].deadline, want)
	}
}

func TestTrafficInvalidatesPendingProbe(t *testing.T) {
	tracker, state := testTracker(5 * time.Second)
	request := probeFor(state, false, state.lastSeen.Add(4*time.Second), state.lastSeen.Add(5*time.Second))
	if eventType, _ := tracker.observeTraffic(state.mac, state.lastSeen.Add(4500*time.Millisecond)); eventType != "" {
		t.Fatalf("traffic emitted %q", eventType)
	}
	if eventType, _ := tracker.applyProbe(request); eventType != "" {
		t.Fatalf("stale probe emitted %q", eventType)
	}
	if !state.online || !state.offlineDeadline.IsZero() {
		t.Fatalf("unexpected state: %#v", state)
	}
}

func TestWeakNeighborDoesNotRefreshConfirmedPresence(t *testing.T) {
	tracker, state := testTracker(5 * time.Second)
	lastSeen := state.lastSeen
	observed := &neighborObservation{ip: state.ip, mac: state.mac, active: true}
	if eventType, _ := tracker.observeNeighbor(observed, lastSeen.Add(time.Second)); eventType != "" {
		t.Fatalf("weak neighbor emitted %q", eventType)
	}
	if !state.lastSeen.Equal(lastSeen) {
		t.Fatalf("weak neighbor refreshed lastSeen: %v", state.lastSeen)
	}
}

func TestConfirmedNeighborInvalidatesPendingProbe(t *testing.T) {
	tracker, state := testTracker(5 * time.Second)
	request := probeFor(state, false, state.lastSeen.Add(4*time.Second), state.lastSeen.Add(5*time.Second))
	observed := &neighborObservation{ip: state.ip, mac: state.mac, active: true, confirmed: true}
	if eventType, _ := tracker.observeNeighbor(observed, state.lastSeen.Add(4500*time.Millisecond)); eventType != "" {
		t.Fatalf("confirmed neighbor emitted %q", eventType)
	}
	if eventType, _ := tracker.applyProbe(request); eventType != "" {
		t.Fatalf("stale probe emitted %q", eventType)
	}
}

func TestProbeSuccessClearsOfflineDeadline(t *testing.T) {
	tracker, state := testTracker(5 * time.Second)
	failedAt := state.lastSeen.Add(time.Second)
	tracker.applyProbe(probeFor(state, false, failedAt, failedAt.Add(probeWindow)))
	if eventType, _ := tracker.applyProbe(probeFor(state, true, failedAt.Add(time.Second), failedAt.Add(2*time.Second))); eventType != "" {
		t.Fatalf("success emitted %q", eventType)
	}
	if !state.offlineDeadline.IsZero() {
		t.Fatalf("offline deadline was not cleared: %v", state.offlineDeadline)
	}
}

func TestStartupBaselineRequiresPositiveEvidence(t *testing.T) {
	mac, _ := net.ParseMAC("02:00:00:00:00:01")
	state := &deviceState{ip: net.IPv4(192, 0, 2, 10), mac: mac, online: true}
	tracker := newPresenceTracker("br-lan", 5*time.Second, map[string]*deviceState{mac.String(): state})
	checked := time.Unix(101, 0)
	if eventType, _ := tracker.applyProbe(probeFor(state, false, checked.Add(-probeWindow), checked)); eventType != "" || state.verified {
		t.Fatalf("startup failure changed baseline: event=%q state=%#v", eventType, state)
	}
	if eventType, _ := tracker.applyProbe(probeFor(state, true, checked, checked.Add(probeWindow))); eventType != "" || !state.verified {
		t.Fatalf("startup success failed baseline verification: event=%q state=%#v", eventType, state)
	}
}

func TestReconnectRequiresPositiveEvidence(t *testing.T) {
	tracker, state := testTracker(5 * time.Second)
	state.online = false
	state.reconnectPending = true
	checked := time.Unix(101, 0)
	if eventType, _ := tracker.applyProbe(probeFor(state, false, checked.Add(-probeWindow), checked)); eventType != "" || state.online {
		t.Fatalf("failed reconnect changed state: event=%q state=%#v", eventType, state)
	}
	state.reconnectPending = true
	if eventType, _ := tracker.applyProbe(probeFor(state, true, checked, checked.Add(probeWindow))); eventType != "device.connected" || !state.online {
		t.Fatalf("successful reconnect: event=%q state=%#v", eventType, state)
	}
}

func TestOnlyOneProbePerDevice(t *testing.T) {
	tracker, state := testTracker(5 * time.Second)
	now := state.lastSeen.Add(4 * time.Second)
	first := tracker.beginProbes(now, maxConcurrentProbes)
	if got := len(first); got != 1 {
		t.Fatalf("first probe count = %d", got)
	}
	if got := len(tracker.beginProbes(now, maxConcurrentProbes)); got != 0 {
		t.Fatalf("duplicate probe count = %d", got)
	}
	state.probePending = false
	second := tracker.beginProbes(now, maxConcurrentProbes)
	if got := len(second); got != 1 {
		t.Fatalf("next probe count = %d", got)
	}
	if first[0].probeID == second[0].probeID {
		t.Fatal("probe ID was reused")
	}
	if eventType, _ := tracker.applyProbe(probeResult{probeRequest: first[0], checked: state.lastSeen.Add(5 * time.Second)}); eventType != "" || !state.probePending {
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
