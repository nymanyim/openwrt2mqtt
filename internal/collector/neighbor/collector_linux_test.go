//go:build linux

package neighbor

import (
	"net"
	"testing"
	"time"
)

func testTracker(timeout time.Duration) (*presenceTracker, *deviceState) {
	mac, _ := net.ParseMAC("02:00:00:00:00:01")
	state := &deviceState{
		ip:       net.IPv4(192, 0, 2, 10),
		mac:      mac,
		online:   true,
		verified: true,
		lastSeen: time.Unix(100, 0),
	}
	return newPresenceTracker("br-lan", timeout, map[string]*deviceState{mac.String(): state}), state
}

func probeResultFor(request probeRequest, online bool, checked time.Time) probeResult {
	return probeResult{
		probeRequest: request,
		online:       online,
		started:      checked.Add(-probeWindow),
		checked:      checked,
	}
}

func requireProbe(t *testing.T, tracker *presenceTracker, now time.Time) probeRequest {
	t.Helper()
	requests := tracker.beginProbes(now, maxConcurrentProbes)
	if len(requests) != 1 {
		t.Fatalf("probe count = %d, want 1", len(requests))
	}
	return requests[0]
}

func enterOfflineCandidate(t *testing.T, tracker *presenceTracker, state *deviceState) time.Time {
	t.Helper()
	candidateDeadline := state.lastSeen.Add(tracker.offlineTimeout - offlineConfirmationWindow)
	request := requireProbe(t, tracker, candidateDeadline.Add(-probeWindow))
	if !request.deadline.Equal(candidateDeadline) {
		t.Fatalf("candidate deadline = %v, want %v", request.deadline, candidateDeadline)
	}
	if eventType, _ := tracker.applyProbe(probeResultFor(request, false, candidateDeadline)); eventType != "" {
		t.Fatalf("candidate failure emitted %q", eventType)
	}
	totalDeadline := state.lastSeen.Add(tracker.offlineTimeout)
	if !state.online || !state.offlineDeadline.Equal(totalDeadline) {
		t.Fatalf("candidate state = %#v, total deadline = %v", state, totalDeadline)
	}
	return totalDeadline
}

func TestConfiguredOfflineTimeIsTotalDuration(t *testing.T) {
	for _, timeout := range []time.Duration{8 * time.Second, 10 * time.Second} {
		t.Run(timeout.String(), func(t *testing.T) {
			tracker, state := testTracker(timeout)
			candidateDeadline := state.lastSeen.Add(timeout - offlineConfirmationWindow)
			if got := tracker.beginProbes(candidateDeadline.Add(-probeWindow-time.Millisecond), maxConcurrentProbes); len(got) != 0 {
				t.Fatalf("early probe count = %d", len(got))
			}
			totalDeadline := enterOfflineCandidate(t, tracker, state)
			if got := totalDeadline.Sub(state.lastSeen); got != timeout {
				t.Fatalf("total offline duration = %v, want %v", got, timeout)
			}
			if got := totalDeadline.Sub(candidateDeadline); got != offlineConfirmationWindow {
				t.Fatalf("confirmation window = %v, want %v", got, offlineConfirmationWindow)
			}
		})
	}
}

func TestEightSecondMinimumUsesThreePlusFivePhases(t *testing.T) {
	tracker, state := testTracker(8 * time.Second)
	candidateDeadline := state.lastSeen.Add(3 * time.Second)
	if got := tracker.beginProbes(state.lastSeen.Add(time.Second-time.Millisecond), maxConcurrentProbes); len(got) != 0 {
		t.Fatalf("early probe count = %d", len(got))
	}
	request := requireProbe(t, tracker, state.lastSeen.Add(time.Second))
	if !request.deadline.Equal(candidateDeadline) {
		t.Fatalf("candidate deadline = %v, want %v", request.deadline, candidateDeadline)
	}
	if eventType, _ := tracker.applyProbe(probeResultFor(request, false, candidateDeadline)); eventType != "" {
		t.Fatalf("candidate failure emitted %q", eventType)
	}
	totalDeadline := state.lastSeen.Add(8 * time.Second)
	if !state.offlineDeadline.Equal(totalDeadline) {
		t.Fatalf("total deadline = %v, want %v", state.offlineDeadline, totalDeadline)
	}
	if got := tracker.beginProbes(totalDeadline.Add(-probeWindow-time.Millisecond), maxConcurrentProbes); len(got) != 0 {
		t.Fatalf("final probe started early: %d", len(got))
	}
	finalRequest := requireProbe(t, tracker, totalDeadline.Add(-probeWindow))
	if !finalRequest.deadline.Equal(totalDeadline) {
		t.Fatalf("final deadline = %v, want %v", finalRequest.deadline, totalDeadline)
	}
}

func TestFailureBeforePhaseDeadlineDoesNotAdvanceState(t *testing.T) {
	tracker, state := testTracker(8 * time.Second)
	candidateDeadline := state.lastSeen.Add(3 * time.Second)
	request := requireProbe(t, tracker, candidateDeadline.Add(-probeWindow))
	checked := candidateDeadline.Add(-time.Millisecond)
	if eventType, _ := tracker.applyProbe(probeResultFor(request, false, checked)); eventType != "" {
		t.Fatalf("early failure emitted %q", eventType)
	}
	if !state.online || !state.offlineDeadline.IsZero() {
		t.Fatalf("early failure changed state: %#v", state)
	}
}

func TestTrafficCancelsOfflineCandidateSilently(t *testing.T) {
	tracker, state := testTracker(8 * time.Second)
	totalDeadline := enterOfflineCandidate(t, tracker, state)
	observedAt := totalDeadline.Add(-3 * time.Second)
	if eventType, _ := tracker.observeTraffic(state.mac, observedAt); eventType != "" {
		t.Fatalf("candidate recovery emitted %q", eventType)
	}
	if !state.online || !state.offlineDeadline.IsZero() || !state.lastSeen.Equal(observedAt) {
		t.Fatalf("candidate recovery state: %#v", state)
	}
}

func TestConfirmedNeighborCancelsOfflineCandidateSilently(t *testing.T) {
	tracker, state := testTracker(8 * time.Second)
	totalDeadline := enterOfflineCandidate(t, tracker, state)
	observed := &neighborObservation{ip: state.ip, mac: state.mac, active: true, confirmed: true}
	observedAt := totalDeadline.Add(-2 * time.Second)
	if eventType, _ := tracker.observeNeighbor(observed, observedAt); eventType != "" {
		t.Fatalf("candidate recovery emitted %q", eventType)
	}
	if !state.online || !state.offlineDeadline.IsZero() || !state.lastSeen.Equal(observedAt) {
		t.Fatalf("candidate recovery state: %#v", state)
	}
}

func TestSuccessfulFinalProbeCancelsOfflineCandidate(t *testing.T) {
	tracker, state := testTracker(8 * time.Second)
	totalDeadline := enterOfflineCandidate(t, tracker, state)
	request := requireProbe(t, tracker, totalDeadline.Add(-probeWindow))
	if eventType, _ := tracker.applyProbe(probeResultFor(request, true, totalDeadline.Add(-probeSettleWindow))); eventType != "" {
		t.Fatalf("successful final probe emitted %q", eventType)
	}
	if !state.online || !state.offlineDeadline.IsZero() {
		t.Fatalf("successful final probe state: %#v", state)
	}
}

func TestTrafficInvalidatesPendingFinalProbe(t *testing.T) {
	tracker, state := testTracker(8 * time.Second)
	totalDeadline := enterOfflineCandidate(t, tracker, state)
	request := requireProbe(t, tracker, totalDeadline.Add(-probeWindow))
	observedAt := totalDeadline.Add(-time.Millisecond)
	if eventType, _ := tracker.observeTraffic(state.mac, observedAt); eventType != "" {
		t.Fatalf("traffic emitted %q", eventType)
	}
	if eventType, _ := tracker.applyProbe(probeResultFor(request, false, totalDeadline)); eventType != "" {
		t.Fatalf("stale final probe emitted %q", eventType)
	}
	if !state.online || !state.offlineDeadline.IsZero() {
		t.Fatalf("stale final probe changed state: %#v", state)
	}
}

func TestFinalFailureDisconnectsAtTotalDeadlineOnce(t *testing.T) {
	tracker, state := testTracker(8 * time.Second)
	totalDeadline := enterOfflineCandidate(t, tracker, state)
	request := requireProbe(t, tracker, totalDeadline.Add(-probeWindow))
	if eventType, _ := tracker.applyProbe(probeResultFor(request, false, totalDeadline.Add(-time.Millisecond))); eventType != "" {
		t.Fatalf("failure before total deadline emitted %q", eventType)
	}
	state.probePending = false
	request = requireProbe(t, tracker, totalDeadline)
	if eventType, _ := tracker.applyProbe(probeResultFor(request, false, totalDeadline)); eventType != "device.disconnected" {
		t.Fatalf("event = %q, want device.disconnected", eventType)
	}
	if state.online || !state.disconnectedAt.Equal(totalDeadline) {
		t.Fatalf("disconnected state: %#v", state)
	}
	if eventType, _ := tracker.applyProbe(probeResultFor(request, false, totalDeadline.Add(time.Second))); eventType != "" {
		t.Fatalf("repeated failure emitted %q", eventType)
	}
}

func TestOnlyEvidenceAfterPublishedDisconnectReconnects(t *testing.T) {
	tracker, state := testTracker(8 * time.Second)
	totalDeadline := enterOfflineCandidate(t, tracker, state)
	request := requireProbe(t, tracker, totalDeadline.Add(-probeWindow))
	if eventType, _ := tracker.applyProbe(probeResultFor(request, false, totalDeadline)); eventType != "device.disconnected" {
		t.Fatalf("disconnect event = %q", eventType)
	}
	if eventType, _ := tracker.observeTraffic(state.mac, totalDeadline.Add(-time.Millisecond)); eventType != "" || state.online {
		t.Fatalf("queued traffic reconnected device: event=%q state=%#v", eventType, state)
	}
	if eventType, _ := tracker.observeTraffic(state.mac, totalDeadline.Add(time.Millisecond)); eventType != "device.connected" || !state.online {
		t.Fatalf("new traffic did not reconnect device: event=%q state=%#v", eventType, state)
	}
}

func TestWeakNeighborDoesNotRefreshConfirmedPresence(t *testing.T) {
	tracker, state := testTracker(8 * time.Second)
	lastSeen := state.lastSeen
	observed := &neighborObservation{ip: state.ip, mac: state.mac, active: true}
	if eventType, _ := tracker.observeNeighbor(observed, lastSeen.Add(time.Second)); eventType != "" {
		t.Fatalf("weak neighbor emitted %q", eventType)
	}
	if !state.lastSeen.Equal(lastSeen) {
		t.Fatalf("weak neighbor refreshed lastSeen: %v", state.lastSeen)
	}
}

func TestUnconfirmedNewNeighborDoesNotEmitConnected(t *testing.T) {
	tracker := newPresenceTracker("br-lan", 8*time.Second, make(map[string]*deviceState))
	mac, _ := net.ParseMAC("02:00:00:00:00:02")
	observed := &neighborObservation{ip: net.IPv4(192, 0, 2, 11), mac: mac, active: true}
	if eventType, state := tracker.observeNeighbor(observed, time.Unix(100, 0)); eventType != "" || state != nil {
		t.Fatalf("unconfirmed neighbor changed presence: event=%q state=%#v", eventType, state)
	}
}

func TestStartupBaselineRequiresPositiveEvidence(t *testing.T) {
	mac, _ := net.ParseMAC("02:00:00:00:00:01")
	state := &deviceState{ip: net.IPv4(192, 0, 2, 10), mac: mac, online: true, lastSeen: time.Unix(100, 0)}
	tracker := newPresenceTracker("br-lan", 8*time.Second, map[string]*deviceState{mac.String(): state})
	candidateDeadline := state.lastSeen.Add(3 * time.Second)
	request := requireProbe(t, tracker, candidateDeadline.Add(-probeWindow))
	if eventType, _ := tracker.applyProbe(probeResultFor(request, false, candidateDeadline)); eventType != "" || state.verified {
		t.Fatalf("startup failure changed baseline: event=%q state=%#v", eventType, state)
	}
	state.probePending = false
	request = requireProbe(t, tracker, candidateDeadline)
	if eventType, _ := tracker.applyProbe(probeResultFor(request, true, candidateDeadline)); eventType != "" || !state.verified {
		t.Fatalf("startup success failed baseline verification: event=%q state=%#v", eventType, state)
	}
}

func TestOnlyOneProbePerDevice(t *testing.T) {
	tracker, state := testTracker(8 * time.Second)
	now := state.lastSeen.Add(time.Second)
	first := tracker.beginProbes(now, maxConcurrentProbes)
	if len(first) != 1 {
		t.Fatalf("first probe count = %d", len(first))
	}
	if got := len(tracker.beginProbes(now, maxConcurrentProbes)); got != 0 {
		t.Fatalf("duplicate probe count = %d", got)
	}
	state.probePending = false
	second := tracker.beginProbes(now, maxConcurrentProbes)
	if len(second) != 1 {
		t.Fatalf("second probe count = %d", len(second))
	}
	if first[0].probeID == second[0].probeID {
		t.Fatal("probe ID was reused")
	}
	if eventType, _ := tracker.applyProbe(probeResultFor(first[0], false, first[0].deadline)); eventType != "" || !state.probePending {
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
