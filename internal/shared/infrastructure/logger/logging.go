package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	apperrors "gitee.com/leoninew/TermBridge-go/internal/shared/common/errors"
)

const filePrefix = "termbridge"

type Config struct {
	Level  string
	Format string
	Dir    string
	Output io.Writer
	// Now is optional; used by tests to control day boundaries for rotation.
	Now func() time.Time
}

type Logger struct {
	*Slog
	closer io.Closer
}

type Slog = slog.Logger

func New(config Config) (*Logger, error) {
	now := config.Now
	if now == nil {
		now = time.Now
	}

	file, err := newDailyWriter(config.Dir, now)
	if err != nil {
		return nil, err
	}

	output := io.Writer(file)
	if config.Output != nil {
		output = io.MultiWriter(file, config.Output)
	}

	opts := &slog.HandlerOptions{Level: parseLevel(config.Level)}
	var handler slog.Handler
	if config.Format == "json" {
		handler = slog.NewJSONHandler(output, opts)
	} else {
		handler = slog.NewTextHandler(output, opts)
	}

	return &Logger{Slog: slog.New(handler), closer: file}, nil
}

func (l *Logger) Close() error {
	if l == nil || l.closer == nil {
		return nil
	}
	return l.closer.Close()
}

// FileName returns the dated log file name for the given local day.
func FileName(day time.Time) string {
	return fmt.Sprintf("%s.%s.log", filePrefix, day.Format("2006-01-02"))
}

func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

type dailyWriter struct {
	dir    string
	now    func() time.Time
	mu     sync.Mutex
	day    string
	file   *os.File
	closed bool
}

func newDailyWriter(dir string, now func() time.Time) (*dailyWriter, error) {
	w := &dailyWriter{dir: dir, now: now}
	if err := w.rotate(dayKey(now())); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *dailyWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return 0, os.ErrClosed
	}

	day := dayKey(w.now())
	if w.file == nil || w.day != day {
		if err := w.rotate(day); err != nil {
			return 0, err
		}
	}
	return w.file.Write(p)
}

func (w *dailyWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	w.closed = true
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

func (w *dailyWriter) rotate(day string) error {
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
	}

	path := filepath.Join(w.dir, fmt.Sprintf("%s.%s.log", filePrefix, day))
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return apperrors.Config("open log file", err)
	}
	w.file = file
	w.day = day
	return nil
}

func dayKey(t time.Time) string {
	return t.Format("2006-01-02")
}
