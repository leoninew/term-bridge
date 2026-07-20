package application

import (
	"testing"
	"time"

	shared "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/shared/v1"
)

func TestCloseTerminalsUnblocksHandlers(t *testing.T) {
	c := New(Config{})
	ch := make(chan *shared.TunnelFrame)
	c.addTerminal("stream-1", ch)

	done := make(chan struct{})
	go func() {
		_, ok := <-ch
		if ok {
			t.Error("expected closed inbound channel")
		}
		close(done)
	}()

	c.closeTerminals()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for terminal channel close")
	}

	c.mu.Lock()
	remaining := len(c.terms)
	c.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("terms remaining = %d, want 0", remaining)
	}
}
