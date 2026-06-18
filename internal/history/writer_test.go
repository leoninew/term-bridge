package history

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	return string(data)
}
