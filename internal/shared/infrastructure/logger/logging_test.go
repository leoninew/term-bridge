package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewCreatesDatedLogFile(t *testing.T) {
	dir := t.TempDir()
	day := time.Date(2026, 7, 17, 10, 0, 0, 0, time.Local)
	logger, err := New(Config{
		Level:  "debug",
		Format: "json",
		Dir:    dir,
		Now:    func() time.Time { return day },
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	logger.Info("test message")
	if err := logger.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	path := filepath.Join(dir, FileName(day))
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("os.Stat(%q) error = %v", path, err)
	}
	if info.Size() == 0 {
		t.Fatal("log file is empty")
	}
}

func TestDailyWriterRotatesAcrossDays(t *testing.T) {
	dir := t.TempDir()
	day := time.Date(2026, 7, 17, 23, 59, 0, 0, time.Local)
	current := day
	now := func() time.Time { return current }

	logger, err := New(Config{Level: "info", Format: "text", Dir: dir, Now: now})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	logger.Info("day one")
	current = day.Add(2 * time.Hour)
	logger.Info("day two")
	if err := logger.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	first := filepath.Join(dir, FileName(day))
	second := filepath.Join(dir, FileName(current))
	firstBody, err := os.ReadFile(first)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", first, err)
	}
	secondBody, err := os.ReadFile(second)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", second, err)
	}
	if !strings.Contains(string(firstBody), "day one") {
		t.Fatalf("first day log missing entry: %s", firstBody)
	}
	if !strings.Contains(string(secondBody), "day two") {
		t.Fatalf("second day log missing entry: %s", secondBody)
	}
	if strings.Contains(string(firstBody), "day two") {
		t.Fatal("first day log unexpectedly contains second day entry")
	}
}

func TestLoggerDoesNotReopenFileAfterClose(t *testing.T) {
	dir := t.TempDir()
	day := time.Date(2026, 7, 17, 10, 0, 0, 0, time.Local)
	logger, err := New(Config{Level: "info", Format: "text", Dir: dir, Now: func() time.Time { return day }})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	logger.Info("before close")
	if err := logger.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	logger.Info("after close")

	body, err := os.ReadFile(filepath.Join(dir, FileName(day)))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(body), "after close") {
		t.Fatalf("closed logger reopened the file: %s", body)
	}
}
