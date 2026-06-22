package app

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"termbridge-go/internal/application/agent"
	"termbridge-go/internal/application/runner"
	"termbridge-go/internal/domain/process"
	apperrors "termbridge-go/internal/infrastructure/errors"
	"termbridge-go/internal/infrastructure/logging"
	httpserver "termbridge-go/internal/transport/http/server"
)

func TestRunExecCallsRuntimePersistsSessionAndReturnsExitCode(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	oldRunRuntime := runRuntime
	defer func() { runRuntime = oldRunRuntime }()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	var gotSpec process.ProcessSpec
	var gotTerminalOutput any
	runRuntime = func(ctx context.Context, logger *logging.Logger, spec process.ProcessSpec, streams runner.IO, hooks runner.Hooks) (runner.Result, error) {
		gotSpec = spec
		gotTerminalOutput = streams.TerminalOutput
		hooks.OnStarted(process.Record{SchemaVersion: 1, PID: 123, Executable: "pwsh", CommandLine: "pwsh -NoLogo", Cwd: spec.Cwd, StartedAt: time.Now().UTC()})
		_, _ = streams.Stdout.Write([]byte("TERM_BRIDGE_HISTORY_TEST\n"))
		return runner.Result{ExitCode: 7, Exit: process.ExitResult{Code: 7}, Process: process.Record{StartedAt: time.Now().UTC()}}, nil
	}

	result, err := Run(context.Background(), Options{Cwd: cwd, Command: Command{Kind: CommandExec, Exec: ExecCommand{Command: []string{"pwsh", "-NoLogo"}}}, Stdin: bytes.NewReader(nil), Stdout: stdout, Stderr: stderr})
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
	if gotTerminalOutput != stdout {
		t.Fatalf("TerminalOutput = %#v, want original stdout", gotTerminalOutput)
	}
	if got := globOne(t, filepath.Join(cwd, ".termbridge", "*", "*", "history.log")); got == "" {
		t.Fatal("history.log was not created")
	}
	if got := globOne(t, filepath.Join(cwd, ".termbridge", "*", "*", "exit.json")); got == "" {
		t.Fatal("exit.json was not created")
	}
}

func TestRunReturnsRuntimeErrorFromRunnerAndMarksFailed(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	oldRunRuntime := runRuntime
	defer func() { runRuntime = oldRunRuntime }()

	runRuntime = func(ctx context.Context, logger *logging.Logger, spec process.ProcessSpec, streams runner.IO, hooks runner.Hooks) (runner.Result, error) {
		return runner.Result{}, apperrors.Runtime("runner failed", nil)
	}

	result, err := Run(context.Background(), Options{Cwd: cwd, Command: Command{Kind: CommandExec, Exec: ExecCommand{Command: []string{"pwsh"}}}, Stdin: bytes.NewReader(nil), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	if err == nil {
		t.Fatal("Run() error = nil, want runtime error")
	}
	if apperrors.KindOf(err) != apperrors.KindRuntime {
		t.Fatalf("Run() error kind = %s, want runtime", apperrors.KindOf(err))
	}
	if len(result.Command) != 1 || result.Command[0] != "pwsh" {
		t.Fatalf("Result.Command = %#v", result.Command)
	}
	stateFile := globOne(t, filepath.Join(cwd, ".termbridge", "*", "*", "state.json"))
	if stateFile == "" {
		t.Fatal("state.json was not created")
	}
}

func TestRunServeStartsUnifiedBackendAndAgentFromConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	configContent := "serve:\n  host: 127.0.0.1\n  port: 9090\n  dev: true\nagent:\n  server_url: http://127.0.0.1:9090\n  device_name: local-mac\n"
	if err := os.WriteFile(filepath.Join(cwd, ".termbridge.yaml"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}
	oldRunBackendServer := runBackendServer
	oldRunAgentClient := runAgentClient
	defer func() {
		runBackendServer = oldRunBackendServer
		runAgentClient = oldRunAgentClient
	}()
	var gotServer *httpserver.Server
	var gotClient *agent.Client
	runBackendServer = func(ctx context.Context, server *httpserver.Server, onListening func(httpserver.Info)) error {
		gotServer = server
		onListening(httpserver.Info{URL: "http://127.0.0.1:9090"})
		<-ctx.Done()
		return ctx.Err()
	}
	runAgentClient = func(ctx context.Context, client *agent.Client) error {
		gotClient = client
		return context.Canceled
	}
	stdout := &bytes.Buffer{}
	result, err := Run(context.Background(), Options{Cwd: cwd, Command: Command{Kind: CommandServe}, Stdout: stdout, Stderr: &bytes.Buffer{}})
	if err != nil {
		t.Fatalf("Run(serve) error = %v", err)
	}
	if filepath.Clean(result.Cwd) != filepath.Clean(cwd) {
		t.Fatalf("Result.Cwd = %q, want %q", result.Cwd, cwd)
	}
	if gotServer == nil {
		t.Fatal("runBackendServer was not called")
	}
	localResponse := httptest.NewRecorder()
	gotServer.ServeHTTP(localResponse, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if localResponse.Code != http.StatusOK || !strings.Contains(localResponse.Body.String(), `"status":"ok"`) {
		t.Fatalf("local API response = %d %s", localResponse.Code, localResponse.Body.String())
	}
	gatewayResponse := httptest.NewRecorder()
	gotServer.ServeHTTP(gatewayResponse, httptest.NewRequest(http.MethodGet, "/api/gateway/health", nil))
	if gatewayResponse.Code != http.StatusOK || !strings.Contains(gatewayResponse.Body.String(), `"status":"ok"`) {
		t.Fatalf("gateway API response = %d %s", gatewayResponse.Code, gatewayResponse.Body.String())
	}
	if gotClient == nil {
		t.Fatal("runAgentClient was not called")
	}
	gotConfig := gotClient.Config()
	if gotConfig.ServerURL != "http://127.0.0.1:9090" || gotConfig.DeviceName != "local-mac" {
		t.Fatalf("agent config = %#v", gotConfig)
	}
	out := stdout.String()
	if !strings.Contains(out, "http://127.0.0.1:9090") || !strings.Contains(out, "TermBridge agent connector targeting http://127.0.0.1:9090") {
		t.Fatalf("stdout = %s", out)
	}
	if strings.Contains(out, "admin/admin") {
		t.Fatalf("stdout exposes temporary auth: %s", out)
	}
}

func TestRunExecUsesConfiguredStateDirAndHistoryLimits(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	configuredStateDir := filepath.Join(cwd, "runtime-state")
	configContent := "history:\n  max_lines: 1\n  max_bytes: 8\n  max_line_bytes: 4\nruntime:\n  state_dir: " + filepath.ToSlash(configuredStateDir) + "\n"
	if err := os.WriteFile(filepath.Join(cwd, ".termbridge.yaml"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}
	oldRunRuntime := runRuntime
	defer func() { runRuntime = oldRunRuntime }()
	runRuntime = func(ctx context.Context, logger *logging.Logger, spec process.ProcessSpec, streams runner.IO, hooks runner.Hooks) (runner.Result, error) {
		hooks.OnStarted(process.Record{SchemaVersion: 1, PID: 123, Executable: "pwsh", CommandLine: "pwsh", Cwd: spec.Cwd, StartedAt: time.Now().UTC()})
		_, _ = streams.Stdout.Write([]byte("abcdef\nsecond\n"))
		return runner.Result{ExitCode: 0, Exit: process.ExitResult{Code: 0}, Process: process.Record{StartedAt: time.Now().UTC()}}, nil
	}

	_, err := Run(context.Background(), Options{Cwd: cwd, Command: Command{Kind: CommandExec, Exec: ExecCommand{Command: []string{"pwsh"}}}, Stdin: bytes.NewReader(nil), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	defaultState := globOne(t, filepath.Join(cwd, ".termbridge", "*", "*", "session.json"))
	if defaultState != "" {
		t.Fatalf("default .termbridge state was created despite configured state_dir: %s", defaultState)
	}
	historyPath := globOne(t, filepath.Join(configuredStateDir, "*", "*", "history.log"))
	if historyPath == "" {
		t.Fatal("configured state dir history.log was not created")
	}
	data, err := os.ReadFile(historyPath)
	if err != nil {
		t.Fatalf("ReadFile(history) error = %v", err)
	}
	if string(data) != "seco" {
		t.Fatalf("history = %q, want configured one-line four-byte truncation", data)
	}
}

func TestRunWorkspaceAndSessionList(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	oldRunRuntime := runRuntime
	defer func() { runRuntime = oldRunRuntime }()
	runRuntime = func(ctx context.Context, logger *logging.Logger, spec process.ProcessSpec, streams runner.IO, hooks runner.Hooks) (runner.Result, error) {
		hooks.OnStarted(process.Record{SchemaVersion: 1, PID: 123, Executable: "pwsh", CommandLine: "pwsh", Cwd: spec.Cwd, StartedAt: time.Now().UTC()})
		return runner.Result{ExitCode: 0, Exit: process.ExitResult{Code: 0}, Process: process.Record{StartedAt: time.Now().UTC()}}, nil
	}
	_, err := Run(context.Background(), Options{Cwd: cwd, Command: Command{Kind: CommandExec, Exec: ExecCommand{Command: []string{"pwsh"}}}, Stdin: bytes.NewReader(nil), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	if err != nil {
		t.Fatalf("Run(exec) error = %v", err)
	}
	var workspaceOut bytes.Buffer
	if _, err := Run(context.Background(), Options{Cwd: cwd, Command: Command{Kind: CommandWorkspace}, Stdout: &workspaceOut}); err != nil {
		t.Fatalf("Run(workspace) error = %v", err)
	}
	if !strings.Contains(workspaceOut.String(), "WORKSPACE ID") || !strings.Contains(workspaceOut.String(), filepath.Base(cwd)) {
		t.Fatalf("workspace output = %s", workspaceOut.String())
	}
	var sessionOut bytes.Buffer
	if _, err := Run(context.Background(), Options{Cwd: cwd, Command: Command{Kind: CommandSession}, Stdout: &sessionOut}); err != nil {
		t.Fatalf("Run(session) error = %v", err)
	}
	sessionOutput := sessionOut.String()
	for _, want := range []string{"SESSION ID", "COMMAND", "CWD", "LOG", "stopped", "pwsh", cwd, "termbridge.log"} {
		if !strings.Contains(sessionOutput, want) {
			t.Fatalf("session output missing %q: %s", want, sessionOutput)
		}
	}
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

func globOne(t *testing.T, pattern string) string {
	t.Helper()
	matches, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("Glob(%q) error = %v", pattern, err)
	}
	if len(matches) == 0 {
		return ""
	}
	return matches[0]
}
