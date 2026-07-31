//go:build linux

package neighbor

import (
	"testing"
	"time"
)

func TestApplyProbeResultRequestsConfirmationAtConfiguredTimeout(t *testing.T) {
	state := &deviceState{online: true}
	probeStarted := time.Unix(100, 0)
	probeInterval := time.Second
	offlineTimeout := 5 * time.Second

	eventType, confirmOffline := applyProbeResult(state, false, probeStarted, probeStarted.Add(250*time.Millisecond), probeInterval, offlineTimeout)
	if eventType != "" || confirmOffline {
		t.Fatal("first failed probe requested early disconnection")
	}
	wantFailedSince := probeStarted.Add(-probeInterval)
	if !state.failedSince.Equal(wantFailedSince) {
		t.Fatalf("failedSince = %v, want %v", state.failedSince, wantFailedSince)
	}

	eventType, confirmOffline = applyProbeResult(state, false, probeStarted.Add(3749*time.Millisecond), probeStarted.Add(3999*time.Millisecond), probeInterval, offlineTimeout)
	if eventType != "" || confirmOffline {
		t.Fatal("probe requested confirmation before offline timeout")
	}

	eventType, confirmOffline = applyProbeResult(state, false, probeStarted.Add(4*time.Second), probeStarted.Add(4250*time.Millisecond), probeInterval, offlineTimeout)
	if eventType != "" || !confirmOffline {
		t.Fatal("probe did not request confirmation at offline timeout")
	}
}

func TestApplyProbeResultClearsFailureAfterSuccess(t *testing.T) {
	state := &deviceState{online: true}
	started := time.Unix(100, 0)

	applyProbeResult(state, false, started, started.Add(250*time.Millisecond), time.Second, 5*time.Second)
	eventType, confirmOffline := applyProbeResult(state, true, started.Add(time.Second), started.Add(time.Second), time.Second, 5*time.Second)
	if eventType != "" || confirmOffline {
		t.Fatal("successful probe produced an event")
	}
	if !state.failedSince.IsZero() {
		t.Fatalf("failedSince was not cleared: %v", state.failedSince)
	}
}

func TestApplyProbeResultRequiresProbeBeforeReconnect(t *testing.T) {
	state := &deviceState{online: false, reconnectPending: true}
	checked := time.Unix(100, 0)

	eventType, confirmOffline := applyProbeResult(state, true, checked, checked, time.Second, 5*time.Second)
	if eventType != "device.connected" || confirmOffline {
		t.Fatalf("event = %q, confirmation = %v", eventType, confirmOffline)
	}
	if !state.online || state.reconnectPending {
		t.Fatalf("unexpected state after reconnect: %#v", state)
	}
}

func TestApplyProbeResultRejectsUnconfirmedReconnect(t *testing.T) {
	state := &deviceState{online: false, reconnectPending: true}
	checked := time.Unix(100, 0)

	eventType, confirmOffline := applyProbeResult(state, false, checked, checked, time.Second, 5*time.Second)
	if eventType != "" || confirmOffline {
		t.Fatal("failed reconnect probe produced an event")
	}
	if state.online || state.reconnectPending {
		t.Fatalf("unexpected state after failed reconnect: %#v", state)
	}
}
