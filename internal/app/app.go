package app

import (
	"context"
	"strings"

	"termbridge-go/internal/config"
	apperrors "termbridge-go/internal/errors"
	"termbridge-go/internal/logging"
)

type Options struct {
	Cwd     string
	Command []string
}

type Result struct {
	Cwd     string
	Command []string
}

func Run(ctx context.Context, options Options) (Result, error) {
	select {
	case <-ctx.Done():
		return Result{}, apperrors.Internal("context cancelled", ctx.Err())
	default:
	}

	cfg, err := config.Load(config.Options{
		Cwd:     options.Cwd,
		Command: options.Command,
	})
	if err != nil {
		return Result{}, err
	}

	logger, err := logging.New(logging.Config{Level: cfg.LogLevel, Format: cfg.LogFormat, Dir: cfg.LogDir})
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = logger.Close() }()

	logger.Info("termbridge command parsed", "cwd", cfg.Cwd, "command", strings.Join(cfg.Command, " "), "config", cfg.ConfigFile)

	result := Result{Cwd: cfg.Cwd, Command: append([]string(nil), cfg.Command...)}
	return result, apperrors.Runtime("termbridge command runner is not implemented yet", nil)
}
