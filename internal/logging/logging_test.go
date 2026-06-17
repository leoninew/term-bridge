package logging

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewCreatesLogFile(t *testing.T) {
	dir := t.TempDir()
	logger, err := New(Config{Level: "debug", Format: "json", Dir: dir})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	logger.Info("test message")
	if err := logger.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	path := filepath.Join(dir, "termbridge.log")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("os.Stat() error = %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("log file is empty")
	}
}
