package config

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"

	apperrors "termbridge-go/internal/infrastructure/errors"
)

func TestLoadDefaults(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)

	cfg, err := Load(Options{Cwd: cwd, Command: []string{"pwsh"}})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.LogLevel != "info" {
		t.Fatalf("LogLevel = %q, want info", cfg.LogLevel)
	}
	if cfg.LogFormat != "text" {
		t.Fatalf("LogFormat = %q, want text", cfg.LogFormat)
	}
	wantLogDir := filepath.Join(cwd, "logs")
	if filepath.Clean(cfg.LogDir) != filepath.Clean(wantLogDir) {
		t.Fatalf("LogDir = %q, want %q", cfg.LogDir, wantLogDir)
	}
	if cfg.LogRequestBodyLimit != 4096 {
		t.Fatalf("LogRequestBodyLimit = %d, want 4096", cfg.LogRequestBodyLimit)
	}
	if cfg.LogResponseBodyLimit != 4096 {
		t.Fatalf("LogResponseBodyLimit = %d, want 4096", cfg.LogResponseBodyLimit)
	}
	wantStateDir := filepath.Join(cwd, ".termbridge")
	if filepath.Clean(cfg.Runtime.StateDir) != filepath.Clean(wantStateDir) {
		t.Fatalf("StateDir = %q, want %q", cfg.Runtime.StateDir, wantStateDir)
	}
	if cfg.History.MaxLines != 10000 {
		t.Fatalf("History.MaxLines = %d", cfg.History.MaxLines)
	}
	if cfg.History.MaxBytes != 5242880 {
		t.Fatalf("History.MaxBytes = %d", cfg.History.MaxBytes)
	}
	if cfg.History.MaxLineBytes != 65536 {
		t.Fatalf("History.MaxLineBytes = %d", cfg.History.MaxLineBytes)
	}
	if cfg.ConfigFile != "" {
		t.Fatalf("ConfigFile = %q, want empty", cfg.ConfigFile)
	}
	if cfg.Web.Error.Debug {
		t.Fatal("Web.Error.Debug = true, want false")
	}
	if cfg.Serve.Host != "127.0.0.1" || cfg.Serve.Port != 9010 || cfg.Serve.Open || cfg.Serve.Dev {
		t.Fatalf("Serve = %#v", cfg.Serve)
	}
	if cfg.Agent.ServerUrl != "http://127.0.0.1:9010" || cfg.Agent.DeviceId != "" || cfg.Agent.DeviceName != "" {
		t.Fatalf("Agent = %#v", cfg.Agent)
	}
	if !reflect.DeepEqual(cfg.Command, []string{"pwsh"}) {
		t.Fatalf("Command = %#v", cfg.Command)
	}
}

func TestLoadRejectsInvalidLogLevel(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeConfig(t, cwd, "log:\n  level: trace\n")

	_, err := Load(Options{Cwd: cwd})
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
	if !apperrors.IsConfig(err) {
		t.Fatalf("Load() error = %T, want config error", err)
	}
}

func TestLoadRejectsInvalidHistoryLimit(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeConfig(t, cwd, "history:\n  max_lines: 0\n")

	_, err := Load(Options{Cwd: cwd})
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
	if !apperrors.IsConfig(err) {
		t.Fatalf("Load() error = %T, want config error", err)
	}
}

func TestLoadRejectsInvalidServePort(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeConfig(t, cwd, "serve:\n  port: 70000\n")

	_, err := Load(Options{Cwd: cwd})
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
	if !apperrors.IsConfig(err) {
		t.Fatalf("Load() error = %T, want config error", err)
	}
}

func TestLoadRejectsEmptyAgentServerUrl(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeConfig(t, cwd, "agent:\n  server_url: \"\"\n")

	_, err := Load(Options{Cwd: cwd})
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
	if !apperrors.IsConfig(err) {
		t.Fatalf("Load() error = %T, want config error", err)
	}
}

func TestLoadRejectsInvalidLogBodyLimit(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{name: "request", content: "log:\n  request_body_limit: 0\n"},
		{name: "response", content: "log:\n  response_body_limit: -1\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			isolateHome(t)
			cwd := t.TempDir()
			writeConfig(t, cwd, tc.content)

			_, err := Load(Options{Cwd: cwd})
			if err == nil {
				t.Fatal("Load() error = nil, want error")
			}
			if !apperrors.IsConfig(err) {
				t.Fatalf("Load() error = %T, want config error", err)
			}
		})
	}
}

func TestLoadRejectsUnknownConfigKey(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeConfig(t, cwd, "log:\n  level: info\nunknown: true\n")

	_, err := Load(Options{Cwd: cwd})
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
	if !apperrors.IsConfig(err) {
		t.Fatalf("Load() error = %T, want config error", err)
	}
}

func TestLoadReadsLocalConfigFile(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	configPath := filepath.Join(cwd, FileName)
	logDir := filepath.Join(cwd, "configured-logs")
	stateDir := filepath.Join(cwd, "configured-state")
	content := "log:\n  level: debug\n  format: json\n  dir: " + filepath.ToSlash(logDir) + "\n  request_body_limit: 128\n  response_body_limit: 256\nhistory:\n  max_lines: 42\n  max_bytes: 2048\n  max_line_bytes: 128\nruntime:\n  state_dir: " + filepath.ToSlash(stateDir) + "\nserve:\n  host: 0.0.0.0\n  port: 9090\n  open: true\n  dev: true\nagent:\n  server_url: http://127.0.0.1:9090\n"
	writeConfig(t, cwd, content)

	cfg, err := Load(Options{Cwd: cwd})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ConfigFile != configPath {
		t.Fatalf("ConfigFile = %q, want %q", cfg.ConfigFile, configPath)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("LogLevel = %q", cfg.LogLevel)
	}
	if cfg.LogFormat != "json" {
		t.Fatalf("LogFormat = %q", cfg.LogFormat)
	}
	if cfg.LogRequestBodyLimit != 128 {
		t.Fatalf("LogRequestBodyLimit = %d, want 128", cfg.LogRequestBodyLimit)
	}
	if cfg.LogResponseBodyLimit != 256 {
		t.Fatalf("LogResponseBodyLimit = %d, want 256", cfg.LogResponseBodyLimit)
	}
	if filepath.Clean(cfg.LogDir) != filepath.Clean(logDir) {
		t.Fatalf("LogDir = %q, want %q", cfg.LogDir, logDir)
	}
	if filepath.Clean(cfg.Runtime.StateDir) != filepath.Clean(stateDir) {
		t.Fatalf("StateDir = %q, want %q", cfg.Runtime.StateDir, stateDir)
	}
	if cfg.History.MaxLines != 42 || cfg.History.MaxBytes != 2048 || cfg.History.MaxLineBytes != 128 {
		t.Fatalf("History = %#v", cfg.History)
	}
	if cfg.Serve.Host != "0.0.0.0" || cfg.Serve.Port != 9090 || !cfg.Serve.Open || !cfg.Serve.Dev {
		t.Fatalf("Serve = %#v", cfg.Serve)
	}
	if cfg.Agent.ServerUrl != "http://127.0.0.1:9090" || cfg.Agent.DeviceId != "" || cfg.Agent.DeviceName != "" {
		t.Fatalf("Agent = %#v", cfg.Agent)
	}
}

func TestLoadReadsWebErrorDebugConfig(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeConfig(t, cwd, "web:\n  error:\n    debug: true\n")

	cfg, err := Load(Options{Cwd: cwd})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Web.Error.Debug {
		t.Fatal("Web.Error.Debug = false, want true")
	}
}

func TestLoadPrefersLocalConfigOverHome(t *testing.T) {
	home := isolateHome(t)
	if err := os.WriteFile(filepath.Join(home, FileName), []byte("log:\n  level: error\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(home) error = %v", err)
	}

	cwd := t.TempDir()
	localConfig := filepath.Join(cwd, FileName)
	writeConfig(t, cwd, "log:\n  level: debug\n")

	cfg, err := Load(Options{Cwd: cwd})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ConfigFile != localConfig {
		t.Fatalf("ConfigFile = %q, want %q", cfg.ConfigFile, localConfig)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("LogLevel = %q, want debug", cfg.LogLevel)
	}
}

func TestLoadUsesPackagedDefaultConfigAndUserOverride(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeConfig(t, cwd, "history:\n  max_lines: 42\n")

	cfg, err := Load(Options{Cwd: cwd})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.History.MaxLines != 42 {
		t.Fatalf("History.MaxLines = %d, want user override", cfg.History.MaxLines)
	}
	if cfg.History.MaxBytes != 5242880 || cfg.History.MaxLineBytes != 65536 {
		t.Fatalf("History defaults were not merged: %#v", cfg.History)
	}
	if filepath.Clean(cfg.Runtime.StateDir) != filepath.Join(cwd, ".termbridge") {
		t.Fatalf("StateDir = %q", cfg.Runtime.StateDir)
	}
}

func TestDefaultDeviceNameUsesHostnameOnly(t *testing.T) {
	hostname, err := os.Hostname()
	want := "termbridge-device"
	if err == nil {
		if sanitized := sanitizeName(hostname); sanitized != "" {
			want = sanitized
		}
	}
	if got := defaultDeviceName(); got != want {
		t.Fatalf("defaultDeviceName() = %q, want %q", got, want)
	}
}

func TestEnsureLocalIdentityWritesCredentialsToDeviceJSON(t *testing.T) {
	isolateHome(t)
	t.Setenv("USERNAME", "tester")
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)

	cfg, err := Load(Options{Cwd: cwd})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	updated, bootstrap, err := EnsureLocalIdentity(cfg)
	if err != nil {
		t.Fatalf("EnsureLocalIdentity() error = %v", err)
	}
	if !bootstrap.Generated {
		t.Fatal("BootstrapResult.Generated = false, want true")
	}
	if updated.Auth.Username != "tester" || updated.Auth.Password == "" {
		t.Fatalf("Auth = %#v", updated.Auth)
	}
	if updated.Agent.DeviceId == "" || updated.Agent.DeviceName != defaultDeviceName() {
		t.Fatalf("Agent = %#v", updated.Agent)
	}

	identityPath := filepath.Join(cwd, ".termbridge", LocalIdentityFileName)
	if filepath.Clean(bootstrap.ConfigFile) != filepath.Clean(identityPath) {
		t.Fatalf("BootstrapResult.ConfigFile = %q, want %q", bootstrap.ConfigFile, identityPath)
	}
	data, err := os.ReadFile(identityPath)
	if err != nil {
		t.Fatalf("ReadFile(identity) error = %v", err)
	}
	content := string(data)
	for _, want := range []string{"\"auth\":", "\"username\": \"tester\"", "\"password\":", "\"agent\":", "\"device_id\":", "\"device_name\": " + strconv.Quote(defaultDeviceName())} {
		if !strings.Contains(content, want) {
			t.Fatalf("identity json missing %q: %s", want, content)
		}
	}
	if strings.Index(content, "\"auth\"") > strings.Index(content, "\"agent\"") {
		t.Fatalf("auth appears after agent: %s", content)
	}
	if strings.Index(content, "\"username\"") > strings.Index(content, "\"password\"") {
		t.Fatalf("username appears after password: %s", content)
	}
	if exists, err := fileExists(filepath.Join(cwd, FileName)); err != nil || exists {
		t.Fatalf("local config exists = %v, err = %v; generated agent identity must stay out of .termbridge.yaml", exists, err)
	}

	reloaded, secondBootstrap, err := EnsureLocalIdentity(updated)
	if err != nil {
		t.Fatalf("second EnsureLocalIdentity() error = %v", err)
	}
	if secondBootstrap.Generated {
		t.Fatal("second BootstrapResult.Generated = true, want false")
	}
	if reloaded.Auth.Username != updated.Auth.Username || reloaded.Auth.Password != updated.Auth.Password {
		t.Fatalf("reloaded Auth = %#v, want %#v", reloaded.Auth, updated.Auth)
	}
}

func TestEnsureLocalIdentityPreservesDeviceJSONAgentFields(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeConfig(t, cwd, "agent:\n  server_url: http://127.0.0.1:9010\n")
	stateDir := filepath.Join(cwd, ".termbridge")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(state dir) error = %v", err)
	}
	identityPath := filepath.Join(stateDir, LocalIdentityFileName)
	before := "{\n  \"auth\": {\"username\": \"admin\"},\n  \"agent\": {\"device_id\": \"dev-1\", \"device_name\": \"local\"}\n}\n"
	if err := os.WriteFile(identityPath, []byte(before), 0o600); err != nil {
		t.Fatalf("WriteFile(identity) error = %v", err)
	}

	cfg, err := Load(Options{Cwd: cwd})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	updated, bootstrap, err := EnsureLocalIdentity(cfg)
	if err != nil {
		t.Fatalf("EnsureLocalIdentity() error = %v", err)
	}
	if !bootstrap.Generated {
		t.Fatal("BootstrapResult.Generated = false, want true")
	}
	if updated.Auth.Username != "admin" || updated.Auth.Password == "" {
		t.Fatalf("Auth = %#v", updated.Auth)
	}
	if updated.Agent.DeviceId != "dev-1" || updated.Agent.DeviceName != "local" {
		t.Fatalf("Agent = %#v", updated.Agent)
	}

	data, err := os.ReadFile(identityPath)
	if err != nil {
		t.Fatalf("ReadFile(identity) error = %v", err)
	}
	content := string(data)
	for _, want := range []string{"\"auth\":", "\"username\": \"admin\"", "\"password\":", "\"agent\":", "\"device_id\": \"dev-1\"", "\"device_name\": \"local\""} {
		if !strings.Contains(content, want) {
			t.Fatalf("identity json missing %q: %s", want, content)
		}
	}
	if strings.Index(content, "\"auth\"") > strings.Index(content, "\"agent\"") {
		t.Fatalf("auth appears after agent: %s", content)
	}
	if strings.Index(content, "\"username\"") > strings.Index(content, "\"password\"") {
		t.Fatalf("username appears after password: %s", content)
	}
}

func TestDefaultConfigFileExists(t *testing.T) {
	if _, err := os.ReadFile(repoDefaultConfigPath(t)); err != nil {
		t.Fatalf("ReadFile(default config) error = %v", err)
	}
}

func TestLoadIgnoresHomeConfigWhenLocalMissing(t *testing.T) {
	home := isolateHome(t)
	if err := os.WriteFile(filepath.Join(home, FileName), []byte("log:\n  level: warn\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	cfg, err := Load(Options{Cwd: cwd})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ConfigFile != "" {
		t.Fatalf("ConfigFile = %q, want empty", cfg.ConfigFile)
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("LogLevel = %q, want info", cfg.LogLevel)
	}
}

func isolateHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}

func writeConfig(t *testing.T, dir string, content string) {
	t.Helper()
	writeDefaultConfig(t, dir)
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(content), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
}

func writeDefaultConfig(t *testing.T, dir string) {
	t.Helper()
	content, err := os.ReadFile(repoDefaultConfigPath(t))
	if err != nil {
		t.Fatalf("ReadFile(default config) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, DefaultFileName), content, 0o644); err != nil {
		t.Fatalf("WriteFile(default config) error = %v", err)
	}
}

func repoDefaultConfigPath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", DefaultFileName))
}
