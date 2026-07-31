//go:build linux

package neighbor

import (
	"context"
	"testing"
	"time"
)

func TestApplyProbeResultRequestsConfirmationBeforeOfflineDeadline(t *testing.T) {
	state := &deviceState{online: true, verified: true}
	probeStarted := time.Unix(100, 0)
	probeInterval := time.Second
	offlineTimeout := 5 * time.Second

	eventType, confirmOffline := applyProbeResult(state, false, probeStarted, probeStarted.Add(probeTimeout), probeInterval, offlineTimeout)
	if eventType != "" || confirmOffline {
		t.Fatal("first failed probe requested early confirmation")
	}
	wantFailedSince := probeStarted.Add(-probeInterval)
	if !state.failedSince.Equal(wantFailedSince) {
		t.Fatalf("failedSince = %v, want %v", state.failedSince, wantFailedSince)
	}

	deadline := wantFailedSince.Add(offlineTimeout)
	beforeWindow := probeStarted.Add(2*probeInterval + probeTimeout)
	eventType, confirmOffline = applyProbeResult(state, false, beforeWindow.Add(-probeTimeout), beforeWindow, probeInterval, offlineTimeout)
	if eventType != "" || confirmOffline {
		t.Fatal("probe requested confirmation before the confirmation window")
	}

	insideWindow := probeStarted.Add(3*probeInterval + probeTimeout)
	eventType, confirmOffline = applyProbeResult(state, false, insideWindow.Add(-probeTimeout), insideWindow, probeInterval, offlineTimeout)
	if eventType != "" || !confirmOffline {
		t.Fatal("probe did not request confirmation inside the confirmation window")
	}
	confirmationBudget := deadline.Sub(insideWindow)
	if confirmationBudget < confirmationWindow {
		t.Fatalf("confirmation budget = %v, want at least %v", confirmationBudget, confirmationWindow)
	}
}

func TestConfirmationProbeIntervalUsesAvailableWindow(t *testing.T) {
	remaining := 750 * time.Millisecond
	want := 175 * time.Millisecond
	if got := confirmationProbeInterval(remaining); got != want {
		t.Fatalf("confirmation interval = %v, want %v", got, want)
	}
	if lastAttempt := time.Duration(confirmationAttempts-1) * want; lastAttempt+confirmationTimeout > remaining {
		t.Fatalf("confirmation attempts exceed remaining window: last=%v timeout=%v remaining=%v", lastAttempt, confirmationTimeout, remaining)
	}
}

func TestConfirmationProbeIntervalKeepsMinimumSpacing(t *testing.T) {
	if got := confirmationProbeInterval(500 * time.Millisecond); got != confirmationInterval {
		t.Fatalf("confirmation interval = %v, want minimum %v", got, confirmationInterval)
	}
}

func TestWaitUntilHonorsDeadline(t *testing.T) {
	const wait = 20 * time.Millisecond
	started := time.Now()
	if !waitUntil(context.Background(), started.Add(wait)) {
		t.Fatal("waitUntil stopped before the deadline")
	}
	if elapsed := time.Since(started); elapsed < wait {
		t.Fatalf("waitUntil returned after %v, want at least %v", elapsed, wait)
	}
}

func TestWaitUntilStopsWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if waitUntil(ctx, time.Now().Add(time.Second)) {
		t.Fatal("waitUntil reported reaching the deadline after cancellation")
	}
}

func TestApplyProbeResultClearsFailureAfterSuccess(t *testing.T) {
	state := &deviceState{online: true, verified: true}
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
	state := &deviceState{online: false, verified: true, reconnectPending: true}
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
	state := &deviceState{online: false, verified: true, reconnectPending: true}
	checked := time.Unix(100, 0)

	eventType, confirmOffline := applyProbeResult(state, false, checked, checked, time.Second, 5*time.Second)
	if eventType != "" || confirmOffline {
		t.Fatal("failed reconnect probe produced an event")
	}
	if state.online || state.reconnectPending {
		t.Fatalf("unexpected state after failed reconnect: %#v", state)
	}
}

func TestApplyProbeResultEstablishesStartupBaselineWithoutEvent(t *testing.T) {
	state := &deviceState{online: true}
	checked := time.Unix(100, 0)
	eventType, confirmOffline := applyProbeResult(state, true, checked, checked, time.Second, 5*time.Second)
	if eventType != "" || confirmOffline || !state.verified {
		t.Fatalf("unexpected startup baseline result: event=%q confirmation=%v state=%#v", eventType, confirmOffline, state)
	}
}

func TestApplyProbeResultSuppressesStartupFailure(t *testing.T) {
	state := &deviceState{online: true}
	checked := time.Unix(100, 0)
	eventType, confirmOffline := applyProbeResult(state, false, checked, checked, time.Second, 5*time.Second)
	if eventType != "" || confirmOffline || state.verified || !state.failedSince.IsZero() {
		t.Fatalf("unexpected startup failure result: event=%q confirmation=%v state=%#v", eventType, confirmOffline, state)
	}
}
