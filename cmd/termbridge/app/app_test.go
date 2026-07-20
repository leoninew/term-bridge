package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gitee.com/leoninew/TermBridge-go/internal/agent/application/task/runner"
	agent "gitee.com/leoninew/TermBridge-go/internal/agent/application/user"
	"gitee.com/leoninew/TermBridge-go/internal/agent/model/task/process"
	httpserver "gitee.com/leoninew/TermBridge-go/internal/shared/api/server"
	apperrors "gitee.com/leoninew/TermBridge-go/internal/shared/common/errors"
	"gitee.com/leoninew/TermBridge-go/internal/shared/infrastructure/config"
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
	runRuntime = func(ctx context.Context, logger *slog.Logger, spec process.ProcessSpec, streams runner.IO, hooks runner.Hooks) (runner.Result, error) {
		gotSpec = spec
		gotTerminalOutput = streams.TerminalOutput
		hooks.OnStarted(process.Record{SchemaVersion: 1, Pid: 123, Executable: "pwsh", CommandLine: "pwsh -NoLogo", Cwd: spec.Cwd, StartedAt: time.Now().UTC()})
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
	if gotSpec.CommandText != "pwsh -NoLogo" {
		t.Fatalf("ProcessSpec.CommandText = %q, want %q", gotSpec.CommandText, "pwsh -NoLogo")
	}
	if filepath.Clean(gotSpec.Cwd) != filepath.Clean(cwd) {
		t.Fatalf("ProcessSpec.Cwd = %q, want %q", gotSpec.Cwd, cwd)
	}
	if gotTerminalOutput != stdout {
		t.Fatalf("TerminalOutput = %#v, want original stdout", gotTerminalOutput)
	}
	if got := globOne(t, filepath.Join(cwd, "data", "history", "*.log")); got == "" {
		t.Fatal("history.log was not created")
	}
	assertRuntimeDBCounts(t, cwd, 1, 1, 1)
	assertSessionExitCode(t, cwd, 7)
}

func TestRunReturnsRuntimeErrorFromRunnerAndMarksFailed(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	oldRunRuntime := runRuntime
	defer func() { runRuntime = oldRunRuntime }()

	runRuntime = func(ctx context.Context, logger *slog.Logger, spec process.ProcessSpec, streams runner.IO, hooks runner.Hooks) (runner.Result, error) {
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
	stateFile := globOne(t, filepath.Join(cwd, "data", "workspaces", "*", "sessions", "*", "state.json"))
	if stateFile != "" {
		t.Fatalf("state.json was created: %s", stateFile)
	}
	assertRuntimeDBCounts(t, cwd, 1, 1, 1)
	assertSessionState(t, cwd, "failed")
}

func TestRunMigrateRunsAgentDatabaseMigrations(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)

	result, err := Run(context.Background(), Options{Cwd: cwd, Command: Command{Kind: CommandMigrate, MigrateRole: "agent"}, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	if err != nil {
		t.Fatalf("Run(migrate agent) error = %v", err)
	}
	if filepath.Clean(result.Cwd) != filepath.Clean(cwd) {
		t.Fatalf("Result.Cwd = %q, want %q", result.Cwd, cwd)
	}
	db := openRuntimeDB(t, cwd)
	defer func() { _ = db.Close() }()
	assertTableCount(t, db, "workspaces", 0)
	assertTableCount(t, db, "sessions", 0)
	assertTableCount(t, db, "session_runs", 0)
	if got := globOne(t, filepath.Join(cwd, "data", "private_key.pem")); got != "" {
		t.Fatalf("migrate created device private key: %s", got)
	}
}

func TestPortableAgentProfileLoadsWithoutCloudJWT(t *testing.T) {
	restoreTermBridgeEnvironment(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	if err := os.Mkdir(filepath.Join(cwd, "web"), 0o755); err != nil {
		t.Fatalf("Mkdir(web) error = %v", err)
	}
	profileContent := readRepoFile(t, "scripts", "package", ".env.preflite")
	restorePackageProfileEnvironment(t, profileContent)
	if err := os.WriteFile(filepath.Join(cwd, ".env.preflite"), []byte(profileContent), 0o644); err != nil {
		t.Fatalf("WriteFile(profile) error = %v", err)
	}
	t.Setenv("TERMBRIDGE_ENV", "preflite")
	t.Setenv("TERMBRIDGE_CLOUD__JWT__SECRET_KEY", "")

	cfg, err := config.Load(config.Options{Cwd: cwd, ValidationScope: config.ValidationScopeAgent})
	if err != nil {
		t.Fatalf("Load(agent profile) error = %v", err)
	}
	if cfg.Cloud.Jwt.SecretKey != "" {
		t.Fatalf("agent profile JWT secret = %q", cfg.Cloud.Jwt.SecretKey)
	}
}

func TestRunAgentStartsBackendAndConnectorFromConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	configContent := "local:\n  expose_errors: true\n  listen_url: http://127.0.0.1:9090\n  public_url: http://localhost:9444/dev/\ncloud:\n  public_url: http://termbridge.lvh.me\n"
	t.Setenv("TERMBRIDGE_ENV", "develop")
	writeEnvConfig(t, cwd, "develop", configContent)
	oldRunBackendServer := runBackendServer
	defer func() {
		runBackendServer = oldRunBackendServer
	}()
	var gotServer *httpserver.Server
	runBackendServer = func(ctx context.Context, server *httpserver.Server, onListening func(httpserver.Info)) error {
		gotServer = server
		onListening(httpserver.Info{Url: "http://127.0.0.1:9090"})
		<-ctx.Done()
		return ctx.Err()
	}
	stdout := &bytes.Buffer{}
	serveCtx, cancelServe := context.WithCancel(context.Background())
	defer cancelServe()
	resultCh := make(chan struct {
		result Result
		err    error
	}, 1)
	go func() {
		result, err := Run(serveCtx, Options{Cwd: cwd, Command: Command{Kind: CommandAgent}, Stdout: stdout, Stderr: &bytes.Buffer{}})
		resultCh <- struct {
			result Result
			err    error
		}{result: result, err: err}
	}()
	waitFor(t, time.Second, func() bool {
		return gotServer != nil
	})
	cancelServe()
	outcome := <-resultCh
	result, err := outcome.result, outcome.err
	if err != nil {
		t.Fatalf("Run(agent) error = %v", err)
	}
	if filepath.Clean(result.Cwd) != filepath.Clean(cwd) {
		t.Fatalf("Result.Cwd = %q, want %q", result.Cwd, cwd)
	}
	if gotServer == nil {
		t.Fatal("runBackendServer was not called")
	}
	healthResponse := httptest.NewRecorder()
	gotServer.ServeHTTP(healthResponse, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if healthResponse.Code != http.StatusOK || !strings.Contains(healthResponse.Body.String(), `"status":"ok"`) {
		t.Fatalf("health response = %d %s", healthResponse.Code, healthResponse.Body.String())
	}
	out := stdout.String()
	if strings.Contains(out, "setup?token=") {
		t.Fatalf("stdout exposes local setup URL: %s", out)
	}
	if strings.Contains(out, "admin/admin") {
		t.Fatalf("stdout exposes temporary auth: %s", out)
	}
}

func TestRunAgentStartsCloudConnectorAfterDeviceReport(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	var reportedDevice struct {
		Id        string `json:"id"`
		Name      string `json:"name"`
		PublicKey string `json:"public_key"`
	}
	cloudPublic := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/devices/current" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer cloud-token" {
			http.NotFound(w, r)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&reportedDevice); err != nil {
			t.Fatalf("decode device report: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"accepted":true}`))
	}))
	defer cloudPublic.Close()
	configContent := "local:\n  expose_errors: true\n  listen_url: http://127.0.0.1:9090\n  public_url: http://localhost:9444/dev/\ncloud:\n  public_url: " + cloudPublic.URL + "\n  api_base_url: " + cloudPublic.URL + "/api\n"
	t.Setenv("TERMBRIDGE_ENV", "develop")
	writeEnvConfig(t, cwd, "develop", configContent)
	oldRunBackendServer := runBackendServer
	oldRunAgentClient := runAgentClient
	defer func() {
		runBackendServer = oldRunBackendServer
		runAgentClient = oldRunAgentClient
	}()
	var gotServer *httpserver.Server
	runBackendServer = func(ctx context.Context, server *httpserver.Server, onListening func(httpserver.Info)) error {
		gotServer = server
		onListening(httpserver.Info{Url: "http://127.0.0.1:9090"})
		<-ctx.Done()
		return ctx.Err()
	}
	var clientMu sync.Mutex
	gotClients := []*agent.Client{}
	runAgentClient = func(ctx context.Context, client *agent.Client) error {
		clientMu.Lock()
		gotClients = append(gotClients, client)
		clientMu.Unlock()
		<-ctx.Done()
		return ctx.Err()
	}
	stdout := &bytes.Buffer{}
	serveCtx, cancelServe := context.WithCancel(context.Background())
	defer cancelServe()
	resultCh := make(chan error, 1)
	go func() {
		_, err := Run(serveCtx, Options{Cwd: cwd, Command: Command{Kind: CommandAgent}, Stdout: stdout, Stderr: &bytes.Buffer{}})
		resultCh <- err
	}()
	waitFor(t, time.Second, func() bool {
		return gotServer != nil
	})

	connectRequest := httptest.NewRequest(http.MethodPost, "/api/cloud/connect", nil)
	connectRequest.Header.Set("Authorization", "Bearer cloud-token")
	connectResponse := httptest.NewRecorder()
	gotServer.ServeHTTP(connectResponse, connectRequest)
	if connectResponse.Code != http.StatusOK {
		t.Fatalf("cloud connect status = %d; body=%s", connectResponse.Code, connectResponse.Body.String())
	}
	if reportedDevice.Id == "" || reportedDevice.Name == "" || reportedDevice.PublicKey == "" {
		t.Fatalf("cloud device report = %#v", reportedDevice)
	}
	waitFor(t, time.Second, func() bool {
		clientMu.Lock()
		defer clientMu.Unlock()
		return len(gotClients) == 1
	})
	clientMu.Lock()
	clientSnapshot := append([]*agent.Client(nil), gotClients...)
	clientMu.Unlock()
	gotTargets := map[string]bool{}
	for _, client := range clientSnapshot {
		gotTargets[client.Config().ConnectUrl] = true
	}
	if len(gotTargets) != 1 || !gotTargets[cloudPublic.URL+"/api"] {
		t.Fatalf("agent connector targets after device report = %#v", gotTargets)
	}
	out := stdout.String()
	if !strings.Contains(out, "TermBridge agent connector targeting "+cloudPublic.URL+"/api") {
		t.Fatalf("stdout missing cloud connector target after device report: %s", out)
	}
	cancelServe()
	if err := <-resultCh; err != nil {
		t.Fatalf("Run(agent) error = %v", err)
	}
}

func TestRunExecUsesConfiguredStateDirAndHistoryLimits(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	configuredStateDir := filepath.Join(cwd, "runtime-state")
	configContent := "terminal:\n  history:\n    max_lines: 1\n    max_bytes: 8\n    max_line_bytes: 4\nruntime:\n  state_dir: " + filepath.ToSlash(configuredStateDir) + "\n"
	t.Setenv("TERMBRIDGE_ENV", "develop")
	writeEnvConfig(t, cwd, "develop", configContent)
	oldRunRuntime := runRuntime
	defer func() { runRuntime = oldRunRuntime }()
	runRuntime = func(ctx context.Context, logger *slog.Logger, spec process.ProcessSpec, streams runner.IO, hooks runner.Hooks) (runner.Result, error) {
		hooks.OnStarted(process.Record{SchemaVersion: 1, Pid: 123, Executable: "pwsh", CommandLine: "pwsh", Cwd: spec.Cwd, StartedAt: time.Now().UTC()})
		_, _ = streams.Stdout.Write([]byte("abcdef\nsecond\n"))
		return runner.Result{ExitCode: 0, Exit: process.ExitResult{Code: 0}, Process: process.Record{StartedAt: time.Now().UTC()}}, nil
	}

	_, err := Run(context.Background(), Options{Cwd: cwd, Command: Command{Kind: CommandExec, Exec: ExecCommand{Command: []string{"pwsh"}}}, Stdin: bytes.NewReader(nil), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	defaultState := globOne(t, filepath.Join(cwd, "data", "workspaces", "*", "sessions", "*", "session.json"))
	if defaultState != "" {
		t.Fatalf("default data state was created despite configured state_dir: %s", defaultState)
	}
	historyPath := globOne(t, filepath.Join(configuredStateDir, "history", "*.log"))
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
	runRuntime = func(ctx context.Context, logger *slog.Logger, spec process.ProcessSpec, streams runner.IO, hooks runner.Hooks) (runner.Result, error) {
		hooks.OnStarted(process.Record{SchemaVersion: 1, Pid: 123, Executable: "pwsh", CommandLine: "pwsh", Cwd: spec.Cwd, StartedAt: time.Now().UTC()})
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
	if !strings.Contains(workspaceOut.String(), "WORKSPACE Id") || !strings.Contains(workspaceOut.String(), filepath.Base(cwd)) {
		t.Fatalf("workspace output = %s", workspaceOut.String())
	}
	var sessionOut bytes.Buffer
	if _, err := Run(context.Background(), Options{Cwd: cwd, Command: Command{Kind: CommandSession}, Stdout: &sessionOut}); err != nil {
		t.Fatalf("Run(session) error = %v", err)
	}
	sessionOutput := sessionOut.String()
	for _, want := range []string{"SESSION Id", "COMMAND", "CWD", "stopped", "pwsh", cwd} {
		if !strings.Contains(sessionOutput, want) {
			t.Fatalf("session output missing %q: %s", want, sessionOutput)
		}
	}
}

func TestDeploymentBuildsAreConfigurationNeutral(t *testing.T) {
	developmentEnv := readRepoFile(t, "web", ".env.development")
	if !strings.Contains(developmentEnv, "TERMBRIDGE_LOCAL__MODE=hybrid") {
		t.Fatalf("development frontend entry must use hybrid mode")
	}

	for _, path := range []string{filepath.Join("..", "..", "..", "web", ".env.local"), filepath.Join("..", "..", "..", "web", ".env.cloud"), filepath.Join("..", "..", "..", "Dockerfile.preflite")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("obsolete deployment configuration %s must be absent, err = %v", path, err)
		}
	}

	packageJSON := readRepoFile(t, "web", "package.json")
	assertContains(t, packageJSON, "\"build\": \"vue-tsc --noEmit && vite build\"", "build script must use the neutral bundle build")
	assertNotContains(t, packageJSON, "\"build:local\"", "local build alias must be absent")
	assertNotContains(t, packageJSON, "\"build:cloud\"", "cloud build alias must be absent")
	for _, dockerfile := range []string{"Dockerfile", "Dockerfile.cn"} {
		content := readRepoFile(t, dockerfile)
		assertContains(t, content, "RUN yarn build", dockerfile+" must use the neutral bundle build")
		assertNotContains(t, content, "RUN yarn build:", dockerfile+" must not use a product-line build alias")
		assertNotContains(t, content, "TERMBRIDGE_", dockerfile+" must not bake runtime configuration")
	}
}

func TestPortablePackageShipsPrefliteRuntimeProfile(t *testing.T) {
	taskfile := readRepoFile(t, "Taskfile.yml")
	envExample := readRepoFile(t, ".env.example")
	config := readRepoFile(t, "configs", "config.yaml")
	startCmd := readRepoFile(t, "scripts", "package", "termbridge.cmd")
	startSh := readRepoFile(t, "scripts", "package", "termbridge.sh")
	prefliteEnv := readRepoFile(t, "scripts", "package", ".env.preflite")

	assertContains(t, taskfile, "sh ./scripts/build-version.sh", "Taskfile must derive build version through the shared Git helper")
	assertContains(t, taskfile, "git rev-parse --short=12 HEAD", "Taskfile must derive the short build commit from Git HEAD")
	assertNotContains(t, taskfile, "TERMBRIDGE_BUILD_COMMIT", "Taskfile must not allow the build commit to diverge from Git HEAD")
	assertContains(t, taskfile, "date -u +%Y%m%d-%H%M%S", "Taskfile must derive the UTC build time")
	assertContains(t, envExample, "TERMBRIDGE_BUILD_VERSION=v1.2.3", ".env.example must document the build version override")
	assertContains(t, envExample, "-dirty", ".env.example must document dirty build versions")
	assertNotContains(t, envExample, "TERMBRIDGE_BUILD_COMMIT=", ".env.example must not imply that the build commit is configurable")
	assertContains(t, config, "TERMBRIDGE_BUILD_VERSION 是 Taskfile/CI 编译元数据", "runtime config must document the build metadata boundary")
	assertNotContains(t, config, "build_version:", "runtime config must not advertise build version as a YAML key")
	assertContains(t, taskfile, "cd web && yarn build", "portable package must use the neutral bundle build")
	assertNotContains(t, taskfile, "yarn build:", "portable package must not use a product-line build alias")
	for _, packageRoot := range []string{"{{.WINDOWS_ROOT}}", "{{.LINUX_ROOT}}", "{{.MACOS_ROOT}}"} {
		assertContains(t, taskfile, "cp scripts/package/.env.preflite "+packageRoot+"/", "portable package must include the preflite runtime profile")
	}

	for scriptName, script := range map[string]string{"termbridge.cmd": startCmd, "termbridge.sh": startSh} {
		assertContains(t, script, "TERMBRIDGE_ENV", scriptName+" must select the application environment")
		assertContains(t, script, "preflite", scriptName+" must default to the preflite environment")
		assertContains(t, script, "TERMBRIDGE_LOCAL__PUBLIC_URL", scriptName+" must open the configured local public URL")
		assertNotContains(t, script, ".env.preflite", scriptName+" must leave profile loading to the application")
	}

	for _, want := range []string{
		"TERMBRIDGE_LOCAL__STATIC_DIR=web",
		"TERMBRIDGE_RUNTIME__STATE_DIR=~/.termbridge",
		"TERMBRIDGE_LOCAL__DATABASE__SQLITE__PATH=~/.termbridge/agent.db",
		"TERMBRIDGE_LOG__DIR=~/.termbridge/logs",
		"TERMBRIDGE_CLOUD__PUBLIC_URL=https://termbridge.preflite.cn",
		"TERMBRIDGE_CLOUD__API_BASE_URL=https://termbridge.preflite.cn/api",
	} {
		assertContains(t, prefliteEnv, want, "preflite runtime profile must carry persistent local runtime configuration")
	}
}

func assertContains(t *testing.T, content string, want string, message string) {
	t.Helper()
	if !strings.Contains(content, want) {
		t.Fatalf("%s: missing %q", message, want)
	}
}

func assertNotContains(t *testing.T, content string, unwanted string, message string) {
	t.Helper()
	if strings.Contains(content, unwanted) {
		t.Fatalf("%s: unexpected %q", message, unwanted)
	}
}

func readRepoFile(t *testing.T, path ...string) string {
	t.Helper()
	parts := append([]string{"..", "..", ".."}, path...)
	content, err := os.ReadFile(filepath.Join(parts...))
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", filepath.Join(path...), err)
	}
	return string(content)
}

func restorePackageProfileEnvironment(t *testing.T, profile string) {
	t.Helper()
	for _, line := range strings.Split(profile, "\n") {
		key, _, ok := strings.Cut(line, "=")
		if !ok || strings.HasPrefix(key, "#") || !strings.HasPrefix(key, "TERMBRIDGE_") {
			continue
		}
		previous, existed := os.LookupEnv(key)
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("Unsetenv(%s) error = %v", key, err)
		}
		t.Cleanup(func() {
			if existed {
				_ = os.Setenv(key, previous)
				return
			}
			_ = os.Unsetenv(key)
		})
	}
}

func restoreTermBridgeEnvironment(t *testing.T) {
	t.Helper()
	for _, key := range []string{"TERMBRIDGE_ENV", "TERMBRIDGE_CLOUD__JWT__SECRET_KEY"} {
		previous, existed := os.LookupEnv(key)
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("Unsetenv(%s) error = %v", key, err)
		}
		t.Cleanup(func() {
			if existed {
				_ = os.Setenv(key, previous)
				return
			}
			_ = os.Unsetenv(key)
		})
	}
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
	t.Setenv("TERMBRIDGE_CLOUD__JWT__SECRET_KEY", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
}

func writeEnvConfig(t *testing.T, dir string, environment string, content string) {
	t.Helper()
	configDir := filepath.Join(dir, "configs")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(configs) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config."+environment+".yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(env config) error = %v", err)
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

func waitFor(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !condition() {
		t.Fatalf("condition was not met within %s", timeout)
	}
}

func assertRuntimeDBCounts(t *testing.T, cwd string, wantWorkspaces int, wantSessions int, wantRuns int) {
	t.Helper()
	db := openRuntimeDB(t, cwd)
	defer func() { _ = db.Close() }()
	assertTableCount(t, db, "workspaces", wantWorkspaces)
	assertTableCount(t, db, "sessions", wantSessions)
	assertTableCount(t, db, "session_runs", wantRuns)
}

func assertSessionExitCode(t *testing.T, cwd string, want int) {
	t.Helper()
	db := openRuntimeDB(t, cwd)
	defer func() { _ = db.Close() }()
	var payload string
	if err := db.QueryRow(`SELECT exit_json FROM session_runs ORDER BY updated_at DESC LIMIT 1`).Scan(&payload); err != nil {
		t.Fatalf("query exit_json error = %v", err)
	}
	if !strings.Contains(payload, `"exit_code":`+fmt.Sprintf("%d", want)) && !strings.Contains(payload, `"exit_code": `+fmt.Sprintf("%d", want)) {
		t.Fatalf("exit_json = %s, want exit code %d", payload, want)
	}
}

func assertSessionState(t *testing.T, cwd string, want string) {
	t.Helper()
	db := openRuntimeDB(t, cwd)
	defer func() { _ = db.Close() }()
	var got string
	if err := db.QueryRow(`SELECT current_state FROM sessions ORDER BY updated_at DESC LIMIT 1`).Scan(&got); err != nil {
		t.Fatalf("query current_state error = %v", err)
	}
	if got != want {
		t.Fatalf("current_state = %q, want %q", got, want)
	}
}

func assertTableCount(t *testing.T, db *sql.DB, table string, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&got); err != nil {
		t.Fatalf("query %s count error = %v", table, err)
	}
	if got != want {
		t.Fatalf("%s count = %d, want %d", table, got, want)
	}
}

func openRuntimeDB(t *testing.T, cwd string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(cwd, "data", "agent.db"))
	if err != nil {
		t.Fatalf("Open runtime sqlite error = %v", err)
	}
	return db
}
