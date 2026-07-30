package collector

import (
	"context"
	"errors"
	"testing"
)

type testCollector struct {
	name    string
	started chan struct{}
	err     error
}

func (c *testCollector) Name() string { return c.name }

func (c *testCollector) Start(ctx context.Context, _ Emitter) error {
	close(c.started)
	if c.err != nil {
		return c.err
	}
	<-ctx.Done()
	return nil
}

func TestMultiStartsAllCollectorsAndPropagatesError(t *testing.T) {
	expected := errors.New("failed")
	first := &testCollector{name: "first", started: make(chan struct{})}
	second := &testCollector{name: "second", started: make(chan struct{}), err: expected}

	err := NewMulti(first, second).Start(context.Background(), nil)
	if !errors.Is(err, expected) {
		t.Fatalf("Start() error = %v", err)
	}
	select {
	case <-first.started:
	default:
		t.Fatal("first collector was not started")
	}
	select {
	case <-second.started:
	default:
		t.Fatal("second collector was not started")
	}
}
