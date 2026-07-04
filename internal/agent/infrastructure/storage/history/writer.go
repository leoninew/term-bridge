package history

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	flushPendingBytes = 64 * 1024
	flushInterval     = 200 * time.Millisecond
)

type Writer struct {
	mu          sync.Mutex
	path        string
	config      Config
	lines       [][]byte
	partial     []byte
	truncated   bool
	pending     int
	lastFlushed time.Time
}

func NewWriter(path string, config Config) (*Writer, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return &Writer{path: path, config: config, lastFlushed: time.Now()}, nil
}

func (w *Writer) Write(p []byte) (int, error) {
	written := len(p)
	if len(p) == 0 {
		return 0, nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for len(p) > 0 {
		idx := bytes.IndexByte(p, '\n')
		if idx < 0 {
			w.partial = append(w.partial, p...)
			w.limitPartial()
			break
		}
		line := append(w.partial, p[:idx+1]...)
		w.partial = nil
		w.addLine(line)
		p = p[idx+1:]
	}
	w.pending += written
	return written, w.flushIfNeededLocked()
}

func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.partial) > 0 {
		w.addLine(w.partial)
		w.partial = nil
	}
	return w.flushLocked()
}

func (w *Writer) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.flushLocked()
}

func (w *Writer) Truncated() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.truncated
}

func (w *Writer) addLine(line []byte) {
	if w.config.MaxLineBytes > 0 && len(line) > w.config.MaxLineBytes {
		line = append([]byte(nil), line[:w.config.MaxLineBytes]...)
		w.truncated = true
	} else {
		line = append([]byte(nil), line...)
	}
	w.lines = append(w.lines, line)
	w.trim()
}

func (w *Writer) limitPartial() {
	if w.config.MaxLineBytes > 0 && len(w.partial) > w.config.MaxLineBytes {
		w.partial = append([]byte(nil), w.partial[:w.config.MaxLineBytes]...)
		w.truncated = true
	}
}

func (w *Writer) trim() {
	if w.config.MaxLines > 0 {
		for len(w.lines) > w.config.MaxLines {
			w.lines = w.lines[1:]
			w.truncated = true
		}
	}
	if w.config.MaxBytes > 0 {
		for w.totalBytes() > w.config.MaxBytes && len(w.lines) > 0 {
			w.lines = w.lines[1:]
			w.truncated = true
		}
	}
}

func (w *Writer) totalBytes() int64 {
	var total int64
	for _, line := range w.lines {
		total += int64(len(line))
	}
	total += int64(len(w.partial))
	return total
}

func (w *Writer) flushIfNeededLocked() error {
	if w.pending >= flushPendingBytes || time.Since(w.lastFlushed) >= flushInterval {
		return w.flushLocked()
	}
	return nil
}

func (w *Writer) flushLocked() error {
	var data []byte
	for _, line := range w.lines {
		data = append(data, line...)
	}
	data = append(data, w.partial...)
	if err := os.WriteFile(w.path, data, 0o644); err != nil {
		return err
	}
	w.pending = 0
	w.lastFlushed = time.Now()
	return nil
}
