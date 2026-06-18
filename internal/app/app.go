package app

import (
	"context"
	"io"
	"strings"
	"time"

	"termbridge-go/internal/config"
	apperrors "termbridge-go/internal/errors"
	"termbridge-go/internal/logging"
	"termbridge-go/internal/process"
	"termbridge-go/internal/pty/gopty"
	"termbridge-go/internal/runner"
)

type Options struct {
	Cwd     string
	Command []string
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
}

type Result struct {
	Cwd      string
	Command  []string
	ExitCode int
}

var runRuntime = func(ctx context.Context, logger *logging.Logger, spec process.ProcessSpec, streams runner.IO) (runner.Result, error) {
	r := runner.CommandRunner{
		Manager:        gopty.NewManager(),
		Logger:         logger,
		InterruptGrace: 1500 * time.Millisecond,
	}
	return r.Run(ctx, spec, streams)
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

	spec, err := process.NewSpec(cfg.Cwd, cfg.Command, process.DefaultTerminalSize())
	if err != nil {
		return Result{}, apperrors.Runtime("build process spec", err)
	}

	runtimeResult, err := runRuntime(ctx, logger, spec, runner.IO{Stdin: options.Stdin, Stdout: options.Stdout, Stderr: options.Stderr})
	if err != nil {
		return Result{Cwd: cfg.Cwd, Command: append([]string(nil), cfg.Command...)}, err
	}

	return Result{Cwd: cfg.Cwd, Command: append([]string(nil), cfg.Command...), ExitCode: runtimeResult.ExitCode}, nil
}
