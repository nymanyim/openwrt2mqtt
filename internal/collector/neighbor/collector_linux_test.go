//go:build linux

package neighbor

import (
	"testing"
	"time"
)

func TestApplyProbeResultDisconnectsAtTimeout(t *testing.T) {
	state := &deviceState{online: true}
	started := time.Unix(100, 0)

	if applyProbeResult(state, false, started, 5*time.Second) {
		t.Fatal("first failure disconnected device")
	}
	if state.failedSince != started {
		t.Fatalf("failedSince = %v, want %v", state.failedSince, started)
	}
	if applyProbeResult(state, false, started.Add(4999*time.Millisecond), 5*time.Second) {
		t.Fatal("device disconnected before timeout")
	}
	if !applyProbeResult(state, false, started.Add(5*time.Second), 5*time.Second) {
		t.Fatal("device did not disconnect at timeout")
	}
	if state.online {
		t.Fatal("device remained online after timeout")
	}
	if !state.failedSince.IsZero() {
		t.Fatalf("failedSince was not cleared: %v", state.failedSince)
	}
}

func TestApplyProbeResultClearsFailureAfterSuccess(t *testing.T) {
	state := &deviceState{online: true}
	started := time.Unix(100, 0)

	applyProbeResult(state, false, started, 5*time.Second)
	if applyProbeResult(state, true, started.Add(time.Second), 5*time.Second) {
		t.Fatal("successful probe disconnected device")
	}
	if !state.failedSince.IsZero() {
		t.Fatalf("failedSince was not cleared: %v", state.failedSince)
	}
	if !state.online {
		t.Fatal("successful probe marked device offline")
	}
}
