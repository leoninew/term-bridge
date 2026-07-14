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

	apperrors "gitee.com/leoninew/TermBridge-go/internal/shared/common/errors"
	"gitee.com/leoninew/TermBridge-go/internal/shared/common/utils/version"
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

func TestParseWorkspaceSessionAndMigrateCommands(t *testing.T) {
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
	for _, role := range []string{"agent", "cloud"} {
		migrate, err := Parse([]string{"migrate", role}, &bytes.Buffer{})
		if err != nil {
			t.Fatalf("Parse(migrate %s) error = %v", role, err)
		}
		if migrate.Kind != CommandMigrate || migrate.Migrate.Role != role {
			t.Fatalf("migrate = %#v, want role %q", migrate, role)
		}
	}
}

func TestParseAgentAndCloudCommands(t *testing.T) {
	agent, err := Parse([]string{"--cwd", `D:\project`, "agent"}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Parse(agent) error = %v", err)
	}
	if agent.Kind != CommandAgent {
		t.Fatalf("Kind = %q, want agent", agent.Kind)
	}
	if agent.Cwd != `D:\project` {
		t.Fatalf("Cwd = %q", agent.Cwd)
	}
	cloud, err := Parse([]string{"cloud"}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Parse(cloud) error = %v", err)
	}
	if cloud.Kind != CommandCloud {
		t.Fatalf("Kind = %q, want cloud", cloud.Kind)
	}
}

func TestParseRoleHelp(t *testing.T) {
	for _, command := range []string{"agent", "cloud"} {
		t.Run(command, func(t *testing.T) {
			cfg, err := Parse([]string{command, "--help"}, &bytes.Buffer{})
			if err != nil {
				t.Fatalf("Parse(%s --help) error = %v", command, err)
			}
			if !cfg.Role.ShowHelp {
				t.Fatal("Role.ShowHelp = false, want true")
			}
		})
	}
}

func TestParseRoleRejectsPositionalArgs(t *testing.T) {
	for _, command := range []string{"agent", "cloud"} {
		t.Run(command, func(t *testing.T) {
			_, err := Parse([]string{command, "extra"}, &bytes.Buffer{})
			if err == nil {
				t.Fatal("Parse() error = nil, want error")
			}
			if !apperrors.IsUsage(err) {
				t.Fatalf("Parse() error type = %T, want usage error", err)
			}
		})
	}
}

func TestParseRejectsRemovedBackendCommands(t *testing.T) {
	for _, command := range []string{"gateway", "serve", "web"} {
		t.Run(command, func(t *testing.T) {
			_, err := Parse([]string{command}, &bytes.Buffer{})
			if err == nil {
				t.Fatal("Parse() error = nil, want error")
			}
			if !apperrors.IsUsage(err) {
				t.Fatalf("Parse() error type = %T, want usage error", err)
			}
		})
	}
}

func TestParseRoleRejectsGatewayAgentRuntimeFlags(t *testing.T) {
	for _, flagName := range []string{"--host", "--port", "--open", "--dev", "--gateway-url", "--device-name", "--seed-session-command"} {
		t.Run(flagName, func(t *testing.T) {
			_, err := Parse([]string{"agent", flagName, "value"}, &bytes.Buffer{})
			if err == nil {
				t.Fatal("Parse() error = nil, want error")
			}
			if !apperrors.IsUsage(err) {
				t.Fatalf("Parse() error type = %T, want usage error", err)
			}
		})
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

func TestParseVersionCommand(t *testing.T) {
	cfg, err := Parse([]string{"version"}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if cfg.Kind != CommandVersion {
		t.Fatalf("Kind = %q, want version", cfg.Kind)
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
		"agent",
		"cloud",
		"exec",
		"workspace",
		"session",
		"migrate",
		"version",
		"termbridge agent",
		"termbridge cloud",
		"termbridge migrate agent",
		"termbridge migrate cloud",
		"termbridge exec -- claude",
		"termbridge --cwd D:\\project exec -- codex",
	} {
		if !strings.Contains(usage, want) {
			t.Fatalf("stdout missing %q: %s", want, usage)
		}
	}
	for _, removed := range []string{"termbridge gateway", "termbridge web", "--gateway-url", "--device-name", "--seed-session-command"} {
		if strings.Contains(stdout.String(), removed) {
			t.Fatalf("stdout exposes removed entry %q: %s", removed, stdout.String())
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
	originalVersion, originalCommit, originalBuildTime := version.Version, version.Commit, version.BuildTime
	version.Version = "v1.2.3"
	version.Commit = "0123456789ab"
	version.BuildTime = "2026-07-14T12:34:56Z"
	t.Cleanup(func() {
		version.Version, version.Commit, version.BuildTime = originalVersion, originalCommit, originalBuildTime
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"version"}, bytes.NewReader(nil), &stdout, &stderr)

	if code != apperrors.ExitSuccess {
		t.Fatalf("Run() code = %d, want %d", code, apperrors.ExitSuccess)
	}
	for _, want := range []string{"termbridge v1.2.3", "commit=0123456789ab", "built=2026-07-14T12:34:56Z"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q: %s", want, stdout.String())
		}
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
	matches, err := filepath.Glob(filepath.Join(cwd, "data", "workspaces", "*", "sessions", "*", "exit.json"))
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("exit.json matches = %#v, want none", matches)
	}
	if _, err := os.Stat(filepath.Join(cwd, "data", "agent.db")); err != nil {
		t.Fatalf("runtime database missing: %v", err)
	}
	historyMatches, err := filepath.Glob(filepath.Join(cwd, "data", "history", "*.log"))
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	if len(historyMatches) != 1 {
		t.Fatalf("history.log matches = %#v", historyMatches)
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
	content, err := os.ReadFile(filepath.Join("..", "..", "..", "configs", "config.yaml"))
	if err != nil {
		t.Fatalf("ReadFile(default config) error = %v", err)
	}
	configDir := filepath.Join(dir, "configs")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(configs) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), content, 0o644); err != nil {
		t.Fatalf("WriteFile(default config) error = %v", err)
	}
	t.Setenv("TERMBRIDGE_JWT__SECRET_KEY", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
}

func isolateHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}
