package quota

import "sync"

// AttachCounter tracks concurrent terminal attaches per subject in-process.
type AttachCounter struct {
	mu    sync.Mutex
	usage map[string]int
}

func NewAttachCounter() *AttachCounter {
	return &AttachCounter{usage: map[string]int{}}
}

// TryAcquire increments usage when under limit. limit <= 0 rejects all.
func (c *AttachCounter) TryAcquire(subject string, limit int) error {
	if subject == "" {
		subject = LocalSubject
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if limit <= 0 {
		return ErrExceeded
	}
	if c.usage[subject] >= limit {
		return ErrExceeded
	}
	c.usage[subject]++
	return nil
}

func (c *AttachCounter) Release(subject string) {
	if subject == "" {
		subject = LocalSubject
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	n := c.usage[subject]
	if n <= 1 {
		delete(c.usage, subject)
		return
	}
	c.usage[subject] = n - 1
}

func (c *AttachCounter) Usage(subject string) int {
	if subject == "" {
		subject = LocalSubject
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.usage[subject]
}
