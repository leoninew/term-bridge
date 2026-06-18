package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"

	apperrors "termbridge-go/internal/errors"
)

type Config struct {
	Level  string
	Format string
	Dir    string
	Output io.Writer
}

type Logger struct {
	*Slog
	closer io.Closer
}

type Slog = slog.Logger

func New(config Config) (*Logger, error) {
	path := filepath.Join(config.Dir, "termbridge.log")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, apperrors.Config("open log file", err)
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
