package quota

import "testing"

func TestAttachCounterAcquireRelease(t *testing.T) {
	c := NewAttachCounter()
	if err := c.TryAcquire("u1", 2); err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	if err := c.TryAcquire("u1", 2); err != nil {
		t.Fatalf("second acquire: %v", err)
	}
	if err := c.TryAcquire("u1", 2); err == nil {
		t.Fatal("third acquire should exceed")
	}
	if c.Usage("u1") != 2 {
		t.Fatalf("usage=%d want 2", c.Usage("u1"))
	}
	c.Release("u1")
	if err := c.TryAcquire("u1", 2); err != nil {
		t.Fatalf("re-acquire after release: %v", err)
	}
	if err := c.TryAcquire("u2", 1); err != nil {
		t.Fatalf("other subject: %v", err)
	}
}

func TestAttachCounterZeroLimit(t *testing.T) {
	c := NewAttachCounter()
	if err := c.TryAcquire("u1", 0); err == nil {
		t.Fatal("expected exceed for zero limit")
	}
}
