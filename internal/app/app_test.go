package app

import (
	"context"
	"testing"

	"github.com/nymanyim/openwrt2mqtt/internal/config"
)

func TestRunStopsWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := New("test").Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestNewRuntimeWithoutDeviceEventHasNoRuntimeComponents(t *testing.T) {
	runtimeApp, err := NewRuntime(context.Background(), "test", config.Runtime{
		DeviceConnectedEnabled: false,
	})
	if err != nil {
		t.Fatalf("NewRuntime() error = %v", err)
	}
	if runtimeApp.collector != nil || runtimeApp.pipeline != nil || runtimeApp.publisher != nil {
		t.Fatal("disabled device event created runtime components")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := runtimeApp.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRunRejectsEmptyVersion(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := New("").Run(ctx); err == nil {
		t.Fatal("Run() expected an error")
	}
}
