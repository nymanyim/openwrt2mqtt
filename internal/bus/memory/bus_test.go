package memory

import (
	"context"
	"errors"
	"testing"

	"github.com/nymanyim/openwrt2mqtt/internal/event"
)

func TestPublishAndReceive(t *testing.T) {
	queue := New(1)
	message := event.Event{ID: "event-1"}
	if err := queue.Publish(context.Background(), message); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if received := <-queue.Events(); received.ID != message.ID {
		t.Fatalf("received ID = %q", received.ID)
	}
}

func TestPublishStopsWhenContextIsCanceled(t *testing.T) {
	queue := New(1)
	_ = queue.Publish(context.Background(), event.Event{ID: "fills-buffer"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := queue.Publish(ctx, event.Event{ID: "blocked"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Publish() error = %v", err)
	}
}
