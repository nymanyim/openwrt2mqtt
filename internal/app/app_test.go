package app

import (
	"context"
	"testing"
)

func TestRunStopsWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := New("test").Run(ctx); err != nil {
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
