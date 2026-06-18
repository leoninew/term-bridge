package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	apperrors "termbridge-go/internal/errors"
)

func TestParseCommandWithDefaultCwd(t *testing.T) {
	cfg, err := Parse([]string{"--", "claude", "--dangerously-skip-permissions"}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	want := []string{"claude", "--dangerously-skip-permissions"}
	if !reflect.DeepEqual(cfg.Command, want) {
		t.Fatalf("Command = %#v, want %#v", cfg.Command, want)
	}
}

func TestParseCommandWithExplicitCwd(t *testing.T) {
	cfg, err := Parse([]string{"--cwd", `D:\project`, "--", "pwsh"}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if cfg.Cwd != `D:\project` {
		t.Fatalf("Cwd = %q", cfg.Cwd)
	}
	if !reflect.DeepEqual(cfg.Command, []string{"pwsh"}) {
		t.Fatalf("Command = %#v", cfg.Command)
	}
}

func TestParseRejectsLogFlags(t *testing.T) {
	_, err := Parse([]string{"--log-level", "debug", "--", "pwsh"}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Parse() error = nil, want error")
	}
	if !apperrors.IsUsage(err) {
		t.Fatalf("Parse() error type = %T, want usage error", err)
	}
}

func TestParseRequiresSeparator(t *testing.T) {
	_, err := Parse([]string{"claude"}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Parse() error = nil, want error")
	}
	if !apperrors.IsUsage(err) {
		t.Fatalf("Parse() error type = %T, want usage error", err)
	}
}

func TestParseRequiresCommandAfterSeparator(t *testing.T) {
	_, err := Parse([]string{"--"}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Parse() error = nil, want error")
	}
	if !apperrors.IsUsage(err) {
		t.Fatalf("Parse() error type = %T, want usage error", err)
	}
}

func TestParseVersionWithoutCommand(t *testing.T) {
	cfg, err := Parse([]string{"--version"}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if !cfg.ShowVersion {
		t.Fatal("ShowVersion = false, want true")
	}
}

func TestRunHelpWritesStdoutOnly(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--help"}, bytes.NewReader(nil), &stdout, &stderr)

	if code != apperrors.ExitSuccess {
		t.Fatalf("Run() code = %d, want %d", code, apperrors.ExitSuccess)
	}
	if !strings.Contains(stdout.String(), "termbridge [options] -- <command>") {
		t.Fatalf("stdout missing usage: %s", stdout.String())
	}
	if strings.Contains(stdout.String(), "--log-level") || strings.Contains(stdout.String(), "--config") {
		t.Fatalf("stdout exposes config-only flags: %s", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunVersionWritesStdoutOnly(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--version"}, bytes.NewReader(nil), &stdout, &stderr)

	if code != apperrors.ExitSuccess {
		t.Fatalf("Run() code = %d, want %d", code, apperrors.ExitSuccess)
	}
	if !strings.Contains(stdout.String(), "termbridge") {
		t.Fatalf("stdout missing version: %s", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunMissingSeparatorReturnsUsageExit(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"claude"}, bytes.NewReader(nil), &stdout, &stderr)

	if code != apperrors.ExitUsage {
		t.Fatalf("Run() code = %d, want %d", code, apperrors.ExitUsage)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "missing -- before command") {
		t.Fatalf("stderr missing usage error: %s", stderr.String())
	}
}

func TestRunInvalidCwdReturnsConfigExit(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--cwd", filepath.Join(t.TempDir(), "missing"), "--", "pwsh"}, bytes.NewReader(nil), &stdout, &stderr)

	if code != apperrors.ExitConfig {
		t.Fatalf("Run() code = %d, want %d", code, apperrors.ExitConfig)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "invalid cwd") {
		t.Fatalf("stderr missing config error: %s", stderr.String())
	}
}

func TestRunCommandCreatesLogAndReturnsCommandExitCode(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cwd := t.TempDir()
	isolateHome(t)

	code := Run([]string{"--cwd", cwd, "--", "cmd.exe", "/C", "exit", "/b", "7"}, bytes.NewReader(nil), &stdout, &stderr)

	if code != 7 {
		t.Fatalf("Run() code = %d, want 7; stderr=%s", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "termbridge command runner is not implemented yet") {
		t.Fatalf("stdout contains old placeholder: %s", stdout.String())
	}
	if strings.Contains(stderr.String(), "error:") {
		t.Fatalf("stderr contains TermBridge error for user exit code: %s", stderr.String())
	}
	logFile := filepath.Join(cwd, "logs", "termbridge.log")
	info, err := os.Stat(logFile)
	if err != nil {
		t.Fatalf("os.Stat(%q) error = %v", logFile, err)
	}
	if info.Size() == 0 {
		t.Fatal("log file is empty")
	}
}

func isolateHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}
