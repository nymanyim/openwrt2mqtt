package pipeline

import (
	"context"
	"errors"
	"fmt"

	"github.com/nymanyim/openwrt2mqtt/internal/bus/memory"
	"github.com/nymanyim/openwrt2mqtt/internal/event"
	"github.com/nymanyim/openwrt2mqtt/internal/processor"
	"github.com/nymanyim/openwrt2mqtt/internal/publisher"
)

// Pipeline consumes queued events, runs processors, and publishes accepted events.
type Pipeline struct {
	bus        *memory.Bus
	processors []processor.Processor
	publisher  publisher.Publisher
}

func New(bus *memory.Bus, publisher publisher.Publisher, processors ...processor.Processor) *Pipeline {
	return &Pipeline{bus: bus, publisher: publisher, processors: processors}
}

// Emit implements collector.Emitter.
func (p *Pipeline) Emit(ctx context.Context, message event.Event) error {
	if p.bus == nil {
		return errors.New("pipeline bus must not be nil")
	}
	return p.bus.Publish(ctx, message)
}

func (p *Pipeline) Run(ctx context.Context) error {
	if p.bus == nil {
		return errors.New("pipeline bus must not be nil")
	}
	if p.publisher == nil {
		return errors.New("pipeline publisher must not be nil")
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case message := <-p.bus.Events():
			accepted, err := p.process(ctx, message)
			if err != nil {
				return err
			}
			if accepted == nil {
				continue
			}
			if err := p.publisher.Publish(ctx, *accepted); err != nil {
				return fmt.Errorf("publish event %q: %w", accepted.ID, err)
			}
		}
	}
}

func (p *Pipeline) process(ctx context.Context, message event.Event) (*event.Event, error) {
	current := message
	for _, eventProcessor := range p.processors {
		if eventProcessor == nil {
			continue
		}
		processed, keep, err := eventProcessor.Process(ctx, current)
		if err != nil {
			return nil, fmt.Errorf("process event %q: %w", current.ID, err)
		}
		if !keep {
			return nil, nil
		}
		current = processed
	}
	return &current, nil
}
