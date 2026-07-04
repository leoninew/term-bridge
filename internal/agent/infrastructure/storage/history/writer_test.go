package history

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriterKeepsSmallOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.log")
	writer, err := NewWriter(path, Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 128})
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}
	if _, err := writer.Write([]byte("one\ntwo\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	flush(t, writer)
	data := readFile(t, path)
	if data != "one\ntwo\n" {
		t.Fatalf("history = %q", data)
	}
}

func TestWriterTrimsByLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.log")
	writer, err := NewWriter(path, Config{MaxLines: 2, MaxBytes: 1024, MaxLineBytes: 128})
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}
	if _, err := writer.Write([]byte("one\ntwo\nthree\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	flush(t, writer)
	data := readFile(t, path)
	if data != "two\nthree\n" {
		t.Fatalf("history = %q", data)
	}
	if !writer.Truncated() {
		t.Fatal("Truncated() = false, want true")
	}
}

func TestWriterTrimsByBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.log")
	writer, err := NewWriter(path, Config{MaxLines: 10, MaxBytes: 6, MaxLineBytes: 128})
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}
	if _, err := writer.Write([]byte("aaaa\nbbbb\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	flush(t, writer)
	data := readFile(t, path)
	if data != "bbbb\n" {
		t.Fatalf("history = %q", data)
	}
}

func TestWriterLimitsLongLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.log")
	writer, err := NewWriter(path, Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 4})
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}
	if _, err := writer.Write([]byte("abcdef\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	flush(t, writer)
	data := readFile(t, path)
	if data != "abcd" {
		t.Fatalf("history = %q", data)
	}
}

func TestWriterPreservesPartialLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.log")
	writer, err := NewWriter(path, Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 128})
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}
	if _, err := writer.Write([]byte("partial")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	data := readFile(t, path)
	if !strings.Contains(data, "partial") {
		t.Fatalf("history = %q", data)
	}
}

func TestWriterBatchesSmallWritesUntilFlush(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.log")
	writer, err := NewWriter(path, Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 128})
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}
	if _, err := writer.Write([]byte("one\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		t.Fatalf("history flushed before threshold: %q", data)
	}
	flush(t, writer)
	if data := readFile(t, path); data != "one\n" {
		t.Fatalf("history = %q", data)
	}
}

func TestWriterFlushesAfterPendingBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.log")
	writer, err := NewWriter(path, Config{MaxLines: 10000, MaxBytes: 1024 * 1024, MaxLineBytes: 1024})
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}
	large := strings.Repeat("x", flushPendingBytes) + "\n"
	if _, err := writer.Write([]byte(large)); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if data := readFile(t, path); len(data) == 0 {
		t.Fatal("history was not flushed after pending byte threshold")
	}
}

func TestWriterFlushesAfterInterval(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.log")
	writer, err := NewWriter(path, Config{MaxLines: 10, MaxBytes: 1024, MaxLineBytes: 128})
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}
	writer.mu.Lock()
	writer.lastFlushed = time.Now().Add(-flushInterval)
	writer.mu.Unlock()
	if _, err := writer.Write([]byte("interval\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if data := readFile(t, path); data != "interval\n" {
		t.Fatalf("history = %q", data)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	return string(data)
}

func flush(t *testing.T, writer *Writer) {
	t.Helper()
	if err := writer.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
}
