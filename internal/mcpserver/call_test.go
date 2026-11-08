package mcpserver

import (
	"context"
	"errors"
	"testing"
)

func TestCallStopsWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	release := make(chan struct{})
	t.Cleanup(func() { close(release) })

	_, err := call(ctx, func() (string, error) {
		<-release
		return "late", nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("call error = %v, want context.Canceled", err)
	}
}
