package pipeline

import (
	"context"
	"sync"
	"testing"

	"github.com/nymanyim/openwrt2mqtt/internal/bus/memory"
	"github.com/nymanyim/openwrt2mqtt/internal/event"
)

type recordingPublisher struct {
	published chan event.Event
}

func (p *recordingPublisher) Publish(_ context.Context, message event.Event) error {
	p.published <- message
	return nil
}
func (p *recordingPublisher) Close() error { return nil }

type dropProcessor struct {
	processed chan struct{}
}

func (p dropProcessor) Process(_ context.Context, _ event.Event) (event.Event, bool, error) {
	close(p.processed)
	return event.Event{}, false, nil
}

func TestRunPublishesQueuedEvent(t *testing.T) {
	queue := memory.New(1)
	output := &recordingPublisher{published: make(chan event.Event, 1)}
	pipe := New(queue, output)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wait sync.WaitGroup
	wait.Add(1)
	go func() {
		defer wait.Done()
		if err := pipe.Run(ctx); err != nil {
			t.Errorf("Run() error = %v", err)
		}
	}()

	if err := pipe.Emit(ctx, event.Event{ID: "event-1"}); err != nil {
		t.Fatalf("Emit() error = %v", err)
	}
	if received := <-output.published; received.ID != "event-1" {
		t.Fatalf("published ID = %q", received.ID)
	}
	cancel()
	wait.Wait()
}

func TestRunDropsRejectedEvent(t *testing.T) {
	queue := memory.New(1)
	output := &recordingPublisher{published: make(chan event.Event, 1)}
	processed := make(chan struct{})
	pipe := New(queue, output, dropProcessor{processed: processed})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = pipe.Run(ctx) }()
	if err := pipe.Emit(ctx, event.Event{ID: "event-1"}); err != nil {
		t.Fatalf("Emit() error = %v", err)
	}
	<-processed
	select {
	case message := <-output.published:
		t.Fatalf("unexpected published event: %#v", message)
	default:
	}
}
