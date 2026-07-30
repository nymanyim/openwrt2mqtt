package collector

import (
	"context"
	"fmt"
)

// Multi runs collectors concurrently and stops all of them when one exits.
type Multi struct {
	collectors []Collector
}

func NewMulti(collectors ...Collector) *Multi {
	return &Multi{collectors: collectors}
}

func (m *Multi) Name() string {
	return "multi"
}

func (m *Multi) Start(ctx context.Context, emitter Emitter) error {
	active := make([]Collector, 0, len(m.collectors))
	for _, source := range m.collectors {
		if source != nil {
			active = append(active, source)
		}
	}
	if len(active) == 0 {
		<-ctx.Done()
		return nil
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	errors := make(chan error, len(active))
	for _, source := range active {
		source := source
		go func() {
			err := source.Start(runCtx, emitter)
			if err != nil {
				err = fmt.Errorf("collector %q: %w", source.Name(), err)
			}
			errors <- err
		}()
	}

	firstError := <-errors
	cancel()
	for range len(active) - 1 {
		err := <-errors
		if firstError == nil && err != nil {
			firstError = err
		}
	}
	return firstError
}
