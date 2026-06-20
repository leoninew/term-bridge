package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	apperrors "termbridge-go/internal/errors"
)

func TestParseExecCommandWithDefaultCwd(t *testing.T) {
	cfg, err := Parse([]string{"exec", "--", "claude", "--dangerously-skip-permissions"}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	want := []string{"claude", "--dangerously-skip-permissions"}
	if cfg.Kind != CommandExec {
		t.Fatalf("Kind = %q", cfg.Kind)
	}
	if !reflect.DeepEqual(cfg.Exec.Command, want) {
		t.Fatalf("Command = %#v, want %#v", cfg.Exec.Command, want)
	}
}

func TestParseExecCommandWithExplicitCwd(t *testing.T) {
	cfg, err := Parse([]string{"--cwd", `D:\project`, "exec", "--", "pwsh"}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if cfg.Cwd != `D:\project` {
		t.Fatalf("Cwd = %q", cfg.Cwd)
	}
	if !reflect.DeepEqual(cfg.Exec.Command, []string{"pwsh"}) {
		t.Fatalf("Command = %#v", cfg.Exec.Command)
	}
}

func TestParseWorkspaceAndSessionCommands(t *testing.T) {
	workspace, err := Parse([]string{"workspace"}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Parse(workspace) error = %v", err)
	}
	if workspace.Kind != CommandWorkspace {
		t.Fatalf("workspace.Kind = %q", workspace.Kind)
	}
	session, err := Parse([]string{"session"}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Parse(session) error = %v", err)
	}
	if session.Kind != CommandSession {
		t.Fatalf("session.Kind = %q", session.Kind)
	}
}

func TestParseWebCommand(t *testing.T) {
	cfg, err := Parse([]string{"--cwd", `D:\project`, "web", "--host", "127.0.0.1", "--port", "8080", "--open", "--dev"}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Parse(web) error = %v", err)
	}
	if cfg.Kind != CommandWeb {
		t.Fatalf("Kind = %q, want web", cfg.Kind)
	}
	if cfg.Cwd != `D:\project` {
		t.Fatalf("Cwd = %q", cfg.Cwd)
	}
	if cfg.Web.Host != "127.0.0.1" || cfg.Web.Port != 8080 || !cfg.Web.Open || !cfg.Web.Dev {
		t.Fatalf("Web = %#v", cfg.Web)
	}
}

func TestParseWebHelp(t *testing.T) {
	cfg, err := Parse([]string{"web", "--help"}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Parse(web --help) error = %v", err)
	}
	if !cfg.Web.ShowHelp {
		t.Fatal("Web.ShowHelp = false, want true")
	}
}

func TestParseWebRejectsPositionalArgs(t *testing.T) {
	_, err := Parse([]string{"web", "pwsh"}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Parse() error = nil, want error")
	}
	if !apperrors.IsUsage(err) {
		t.Fatalf("Parse() error type = %T, want usage error", err)
	}
}

func TestParseRejectsLogFlags(t *testing.T) {
	_, err := Parse([]string{"--log-level", "debug", "exec", "--", "pwsh"}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Parse() error = nil, want error")
	}
	if !apperrors.IsUsage(err) {
		t.Fatalf("Parse() error type = %T, want usage error", err)
	}
}

func TestParseRequiresSubcommand(t *testing.T) {
	_, err := Parse([]string{"claude"}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Parse() error = nil, want error")
	}
	if !apperrors.IsUsage(err) {
		t.Fatalf("Parse() error type = %T, want usage error", err)
	}
}

func TestParseRejectsOldGrammar(t *testing.T) {
	_, err := Parse([]string{"--", "claude"}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Parse() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "old command form") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseExecRequiresSeparator(t *testing.T) {
	_, err := Parse([]string{"exec", "claude"}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Parse() error = nil, want error")
	}
	if !apperrors.IsUsage(err) {
		t.Fatalf("Parse() error type = %T, want usage error", err)
	}
}

func TestParseExecRequiresCommandAfterSeparator(t *testing.T) {
	_, err := Parse([]string{"exec", "--"}, &bytes.Buffer{})
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

func TestParseExecHelp(t *testing.T) {
	cfg, err := Parse([]string{"exec", "--help"}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if !cfg.Exec.ShowHelp {
		t.Fatal("Exec.ShowHelp = false, want true")
	}
}

func TestRunHelpWritesStdoutOnly(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--help"}, bytes.NewReader(nil), &stdout, &stderr)

	if code != apperrors.ExitSuccess {
		t.Fatalf("Run() code = %d, want %d", code, apperrors.ExitSuccess)
	}
	usage := stdout.String()
	for _, want := range []string{
		"termbridge [options] <command>",
		"exec",
		"workspace",
		"session",
		"web",
		"termbridge exec -- claude",
		"termbridge --cwd D:\\project exec -- codex",
		"termbridge web --dev",
	} {
		if !strings.Contains(usage, want) {
			t.Fatalf("stdout missing %q: %s", want, usage)
		}
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

func TestRunMissingCommandReturnsUsageExit(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"claude"}, bytes.NewReader(nil), &stdout, &stderr)

	if code != apperrors.ExitUsage {
		t.Fatalf("Run() code = %d, want %d", code, apperrors.ExitUsage)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("stderr missing usage error: %s", stderr.String())
	}
}

func TestRunInvalidCwdReturnsConfigExit(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--cwd", filepath.Join(t.TempDir(), "missing"), "exec", "--", "pwsh"}, bytes.NewReader(nil), &stdout, &stderr)

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

func TestRunCommandCreatesLogStateAndReturnsCommandExitCode(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	isolateHome(t)

	code := Run(append([]string{"--cwd", cwd, "exec", "--"}, exitCommand(7)...), bytes.NewReader(nil), &stdout, &stderr)

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
	matches, err := filepath.Glob(filepath.Join(cwd, ".termbridge", "*", "*", "exit.json"))
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("exit.json matches = %#v", matches)
	}
}

func exitCommand(code int) []string {
	if runtime.GOOS == "windows" {
		return []string{"cmd.exe", "/C", "exit", "/b", fmt.Sprint(code)}
	}
	return []string{"sh", "-c", fmt.Sprintf("exit %d", code)}
}

func writeDefaultConfig(t *testing.T, dir string) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "..", ".termbridge.default.yaml"))
	if err != nil {
		t.Fatalf("ReadFile(default config) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".termbridge.default.yaml"), content, 0o644); err != nil {
		t.Fatalf("WriteFile(default config) error = %v", err)
	}
}

func isolateHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}
