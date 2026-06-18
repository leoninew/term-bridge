package app

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	apperrors "termbridge-go/internal/errors"
	"termbridge-go/internal/logging"
	"termbridge-go/internal/process"
	"termbridge-go/internal/runner"
)

func TestRunCallsRuntimeAndReturnsExitCode(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	cwd := t.TempDir()
	oldRunRuntime := runRuntime
	defer func() { runRuntime = oldRunRuntime }()

	var gotSpec process.ProcessSpec
	runRuntime = func(ctx context.Context, logger *logging.Logger, spec process.ProcessSpec, streams runner.IO) (runner.Result, error) {
		gotSpec = spec
		return runner.Result{ExitCode: 7}, nil
	}

	result, err := Run(context.Background(), Options{Cwd: cwd, Command: []string{"pwsh", "-NoLogo"}, Stdin: bytes.NewReader(nil), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.ExitCode != 7 {
		t.Fatalf("ExitCode = %d, want 7", result.ExitCode)
	}
	if gotSpec.Command != "pwsh" || len(gotSpec.Args) != 1 || gotSpec.Args[0] != "-NoLogo" {
		t.Fatalf("ProcessSpec = %#v", gotSpec)
	}
	if filepath.Clean(gotSpec.Cwd) != filepath.Clean(cwd) {
		t.Fatalf("ProcessSpec.Cwd = %q, want %q", gotSpec.Cwd, cwd)
	}
}

func TestRunReturnsRuntimeErrorFromRunner(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	cwd := t.TempDir()
	oldRunRuntime := runRuntime
	defer func() { runRuntime = oldRunRuntime }()

	runRuntime = func(ctx context.Context, logger *logging.Logger, spec process.ProcessSpec, streams runner.IO) (runner.Result, error) {
		return runner.Result{}, apperrors.Runtime("runner failed", nil)
	}

	result, err := Run(context.Background(), Options{Cwd: cwd, Command: []string{"pwsh"}, Stdin: bytes.NewReader(nil), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	if err == nil {
		t.Fatal("Run() error = nil, want runtime error")
	}
	if apperrors.KindOf(err) != apperrors.KindRuntime {
		t.Fatalf("Run() error kind = %s, want runtime", apperrors.KindOf(err))
	}
	if len(result.Command) != 1 || result.Command[0] != "pwsh" {
		t.Fatalf("Result.Command = %#v", result.Command)
	}
}
