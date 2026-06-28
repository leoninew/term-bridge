package config

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
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
	if cfg.LogHTTP.RequestBodyLimit != 4096 {
		t.Fatalf("LogHTTP.RequestBodyLimit = %d, want 4096", cfg.LogHTTP.RequestBodyLimit)
	}
	if cfg.LogHTTP.ResponseBodyLimit != 4096 {
		t.Fatalf("LogHTTP.ResponseBodyLimit = %d, want 4096", cfg.LogHTTP.ResponseBodyLimit)
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
	if !reflect.DeepEqual(cfg.Gate.Browser.AllowedOrigins, []string{"http://127.0.0.1:9031", "http://localhost:9031"}) {
		t.Fatalf("Gate.Browser.AllowedOrigins = %#v", cfg.Gate.Browser.AllowedOrigins)
	}
	if cfg.Gate.API.ExposeErrors {
		t.Fatal("Gate.API.ExposeErrors = true, want false")
	}
	if cfg.Agent.ListenUrl != "http://127.0.0.1:9030" {
		t.Fatalf("Agent.ListenUrl = %q, want default listen URL", cfg.Agent.ListenUrl)
	}
	if cfg.Agent.ConnectUrl != "http://127.0.0.1:9030" {
		t.Fatalf("Agent.ConnectUrl = %q, want listen URL", cfg.Agent.ConnectUrl)
	}
	if cfg.Cloud.CallbackBaseUrl != "http://127.0.0.1:9030" {
		t.Fatalf("Cloud.CallbackBaseUrl = %q, want local callback base URL", cfg.Cloud.CallbackBaseUrl)
	}
	if cfg.Agent.DeviceId != "" || cfg.Agent.DeviceName != "" {
		t.Fatalf("Agent = %#v, want empty identity", cfg.Agent)
	}
	if cfg.Auth.Username != DefaultAuthUsername || cfg.Auth.Password != DefaultAuthPassword {
		t.Fatalf("Auth = %#v, want default PoC auth", cfg.Auth)
	}
	if cfg.Web.StaticDir != "" {
		t.Fatalf("Web.StaticDir = %q, want empty", cfg.Web.StaticDir)
	}
	if cfg.Agent.PublicUrl != "http://localhost:9031" {
		t.Fatalf("Agent.PublicUrl = %q, want local frontend URL", cfg.Agent.PublicUrl)
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

func TestLoadRejectsInvalidAgentListenURL(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeConfig(t, cwd, "agent:\n  listen_url: not-a-url\n")

	_, err := Load(Options{Cwd: cwd})
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
	if !apperrors.IsConfig(err) {
		t.Fatalf("Load() error = %T, want config error", err)
	}
}

func TestLoadRejectsInvalidAgentPublicURL(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeConfig(t, cwd, "agent:\n  public_url: ftp://example.com\n")

	_, err := Load(Options{Cwd: cwd})
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
	if !apperrors.IsConfig(err) {
		t.Fatalf("Load() error = %T, want config error", err)
	}
}

func TestLoadRejectsInvalidAgentConnectURL(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeConfig(t, cwd, "agent:\n  connect_url: ftp://example.com\n")

	_, err := Load(Options{Cwd: cwd})
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
	if !apperrors.IsConfig(err) {
		t.Fatalf("Load() error = %T, want config error", err)
	}
}

func TestLoadNormalizesZeroOrNegativeLogBodyLimit(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeConfig(t, cwd, "log:\n  http:\n    request_body_limit: 0\n    response_body_limit: -1\n")

	cfg, err := Load(Options{Cwd: cwd})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.LogHTTP.RequestBodyLimit != 0 {
		t.Fatalf("LogHTTP.RequestBodyLimit = %d, want 0", cfg.LogHTTP.RequestBodyLimit)
	}
	if cfg.LogHTTP.ResponseBodyLimit != 0 {
		t.Fatalf("LogHTTP.ResponseBodyLimit = %d, want 0", cfg.LogHTTP.ResponseBodyLimit)
	}
}

func TestLoadReadsLocalConfigFile(t *testing.T) {
	isolateHome(t)
	configureRemoteAuth(t)
	cwd := t.TempDir()
	configPath := filepath.Join(cwd, FileName)
	logDir := filepath.Join(cwd, "configured-logs")
	stateDir := filepath.Join(cwd, "configured-state")
	content := "log:\n  level: debug\n  format: json\n  dir: " + filepath.ToSlash(logDir) + "\n  http:\n    request_body_limit: 128\n    response_body_limit: 256\nhistory:\n  max_lines: 42\n  max_bytes: 2048\n  max_line_bytes: 128\nruntime:\n  state_dir: " + filepath.ToSlash(stateDir) + "\nweb:\n  static_dir: web/dist\ngate:\n  browser:\n    allowed_origins:\n      - \" http://127.0.0.1:9031 \"\n      - http://127.0.0.1:9031\n  api:\n    expose_errors: true\nagent:\n  listen_url: http://0.0.0.0:9090\n  public_url: https://configured.example.com/app/\n  connect_url: https://gate.example.com:9443/\n  device_id: dev-1\n  device_name: local-mac\n"
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
	if cfg.LogHTTP.RequestBodyLimit != 128 {
		t.Fatalf("LogHTTP.RequestBodyLimit = %d, want 128", cfg.LogHTTP.RequestBodyLimit)
	}
	if cfg.LogHTTP.ResponseBodyLimit != 256 {
		t.Fatalf("LogHTTP.ResponseBodyLimit = %d, want 256", cfg.LogHTTP.ResponseBodyLimit)
	}
	if filepath.Clean(cfg.LogDir) != filepath.Clean(logDir) {
		t.Fatalf("LogDir = %q, want %q", cfg.LogDir, logDir)
	}
	if filepath.Clean(cfg.Runtime.StateDir) != filepath.Clean(stateDir) {
		t.Fatalf("StateDir = %q, want %q", cfg.Runtime.StateDir, stateDir)
	}
	if filepath.Clean(cfg.Web.StaticDir) != filepath.Join(cwd, "web", "dist") {
		t.Fatalf("Web.StaticDir = %q", cfg.Web.StaticDir)
	}
	if cfg.Agent.PublicUrl != "https://configured.example.com/app" {
		t.Fatalf("Agent.PublicUrl = %q", cfg.Agent.PublicUrl)
	}
	if cfg.History.MaxLines != 42 || cfg.History.MaxBytes != 2048 || cfg.History.MaxLineBytes != 128 {
		t.Fatalf("History = %#v", cfg.History)
	}
	if cfg.Agent.ListenUrl != "http://0.0.0.0:9090" {
		t.Fatalf("Agent.ListenUrl = %q, want http://0.0.0.0:9090", cfg.Agent.ListenUrl)
	}
	if !reflect.DeepEqual(cfg.Gate.Browser.AllowedOrigins, []string{"http://127.0.0.1:9031", "http://127.0.0.1:9031"}) {
		t.Fatalf("Gate.Browser.AllowedOrigins = %#v", cfg.Gate.Browser.AllowedOrigins)
	}
	if !cfg.Gate.API.ExposeErrors {
		t.Fatal("Gate.API.ExposeErrors = false, want true")
	}
	if cfg.Agent.ConnectUrl != "https://gate.example.com:9443" {
		t.Fatalf("Agent.ConnectUrl = %q", cfg.Agent.ConnectUrl)
	}
	if cfg.Agent.DeviceId != "dev-1" || cfg.Agent.DeviceName != "local-mac" {
		t.Fatalf("Agent = %#v", cfg.Agent)
	}
}

func TestLoadDotEnvOverridesLocalConfigFile(t *testing.T) {
	isolateHome(t)
	configureRemoteAuth(t)
	cwd := t.TempDir()
	logDir := filepath.Join(cwd, "dotenv-logs")
	stateDir := filepath.Join(cwd, "dotenv-state")
	writeConfig(t, cwd, "log:\n  level: info\n  format: text\n  dir: local-logs\n  http:\n    request_body_limit: 128\n    response_body_limit: 256\nhistory:\n  max_lines: 10\n  max_bytes: 1024\n  max_line_bytes: 64\nruntime:\n  state_dir: local-state\ngate:\n  browser:\n    allowed_origins:\n      - http://127.0.0.1:9031\n  api:\n    expose_errors: false\nagent:\n  listen_url: http://127.0.0.1:9090\n  connect_url: http://127.0.0.1:9090\n  device_id: local-device\n  device_name: local-name\n")
	writeDotEnv(t, cwd, "TERMBRIDGE_LOG__LEVEL=debug\nTERMBRIDGE_LOG__FORMAT=json\nTERMBRIDGE_LOG__DIR="+filepath.ToSlash(logDir)+"\nTERMBRIDGE_LOG__HTTP__REQUEST_BODY_LIMIT=512\nTERMBRIDGE_LOG__HTTP__RESPONSE_BODY_LIMIT=1024\nTERMBRIDGE_HISTORY__MAX_LINES=20\nTERMBRIDGE_HISTORY__MAX_BYTES=4096\nTERMBRIDGE_HISTORY__MAX_LINE_BYTES=256\nTERMBRIDGE_RUNTIME__STATE_DIR="+filepath.ToSlash(stateDir)+"\nTERMBRIDGE_WEB__STATIC_DIR=/opt/termbridge/web/dist\nTERMBRIDGE_AGENT__PUBLIC_URL=https://dotenv.example.com\nTERMBRIDGE_AGENT__LISTEN_URL=http://127.0.0.1:9091\nTERMBRIDGE_GATE__BROWSER__ALLOWED_ORIGINS=http://127.0.0.1:9031, http://127.0.0.1:9031,,\nTERMBRIDGE_GATE__API__EXPOSE_ERRORS=true\nTERMBRIDGE_AGENT__CONNECT_URL=https://gate.example.com\nTERMBRIDGE_AGENT__DEVICE_ID=dotenv-device\nTERMBRIDGE_AGENT__DEVICE_NAME=dotenv-name\n")

	cfg, err := Load(Options{Cwd: cwd})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.LogLevel != "debug" || cfg.LogFormat != "json" {
		t.Fatalf("log config = %s/%s", cfg.LogLevel, cfg.LogFormat)
	}
	if filepath.Clean(cfg.LogDir) != filepath.Clean(logDir) {
		t.Fatalf("LogDir = %q, want %q", cfg.LogDir, logDir)
	}
	if cfg.LogHTTP.RequestBodyLimit != 512 || cfg.LogHTTP.ResponseBodyLimit != 1024 {
		t.Fatalf("LogHTTP = %#v", cfg.LogHTTP)
	}
	if cfg.History.MaxLines != 20 || cfg.History.MaxBytes != 4096 || cfg.History.MaxLineBytes != 256 {
		t.Fatalf("History = %#v", cfg.History)
	}
	if filepath.Clean(cfg.Runtime.StateDir) != filepath.Clean(stateDir) {
		t.Fatalf("StateDir = %q, want %q", cfg.Runtime.StateDir, stateDir)
	}
	if cfg.Web.StaticDir != filepath.Clean("/opt/termbridge/web/dist") {
		t.Fatalf("Web.StaticDir = %q", cfg.Web.StaticDir)
	}
	if cfg.Agent.PublicUrl != "https://dotenv.example.com" {
		t.Fatalf("Agent.PublicUrl = %q", cfg.Agent.PublicUrl)
	}
	if cfg.Agent.ListenUrl != "http://127.0.0.1:9091" {
		t.Fatalf("Agent.ListenUrl = %q, want http://127.0.0.1:9091", cfg.Agent.ListenUrl)
	}
	if !reflect.DeepEqual(cfg.Gate.Browser.AllowedOrigins, []string{"http://127.0.0.1:9031", "http://127.0.0.1:9031"}) {
		t.Fatalf("Gate.Browser.AllowedOrigins = %#v", cfg.Gate.Browser.AllowedOrigins)
	}
	if !cfg.Gate.API.ExposeErrors {
		t.Fatal("Gate.API.ExposeErrors = false, want true")
	}
	if cfg.Agent.ConnectUrl != "https://gate.example.com" || cfg.Agent.DeviceId != "dotenv-device" || cfg.Agent.DeviceName != "dotenv-name" {
		t.Fatalf("Agent = %#v", cfg.Agent)
	}
}

func TestLoadOSEnvOverridesDotEnv(t *testing.T) {
	isolateHome(t)
	configureRemoteAuth(t)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	writeDotEnv(t, cwd, "TERMBRIDGE_AGENT__LISTEN_URL=http://127.0.0.1:8080\nTERMBRIDGE_AGENT__CONNECT_URL=http://127.0.0.1:8080\nTERMBRIDGE_GATE__API__EXPOSE_ERRORS=false\n")
	t.Setenv("TERMBRIDGE_AGENT__LISTEN_URL", "http://127.0.0.1:9092")
	t.Setenv("TERMBRIDGE_AGENT__CONNECT_URL", "https://gate.example.com")
	t.Setenv("TERMBRIDGE_GATE__API__EXPOSE_ERRORS", "true")

	cfg, err := Load(Options{Cwd: cwd})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Agent.ListenUrl != "http://127.0.0.1:9092" {
		t.Fatalf("Agent.ListenUrl = %q, want http://127.0.0.1:9092", cfg.Agent.ListenUrl)
	}
	if cfg.Agent.ConnectUrl != "https://gate.example.com" {
		t.Fatalf("Agent.ConnectUrl = %q", cfg.Agent.ConnectUrl)
	}
	if !cfg.Gate.API.ExposeErrors {
		t.Fatal("Gate.API.ExposeErrors = false, want true")
	}
}

func TestLoadIgnoresMissingDotEnv(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)

	cfg, err := Load(Options{Cwd: cwd})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Agent.ListenUrl != "http://127.0.0.1:9030" {
		t.Fatalf("Agent.ListenUrl = %q, want http://127.0.0.1:9030", cfg.Agent.ListenUrl)
	}
}

func TestLoadRejectsInvalidDotEnv(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	writeDotEnv(t, cwd, "TERMBRIDGE_LOG__LEVEL='unterminated")

	_, err := Load(Options{Cwd: cwd})
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
	if !apperrors.IsConfig(err) {
		t.Fatalf("Load() error = %T, want config error", err)
	}
	if !strings.Contains(err.Error(), "read env file") {
		t.Fatalf("Load() error = %v, want env file context", err)
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

func TestEnsureLocalIdentityWritesAgentIdentityToConfigAndUsesDefaultPoCAuth(t *testing.T) {
	isolateHome(t)
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
	if bootstrap.Generated {
		t.Fatal("BootstrapResult.Generated = true, want false")
	}
	if updated.Auth.Username != DefaultAuthUsername || updated.Auth.Password != DefaultAuthPassword {
		t.Fatalf("Auth = %#v, want default PoC auth", updated.Auth)
	}
	if updated.Agent.DeviceId == "" || updated.Agent.DeviceName != defaultDeviceName() {
		t.Fatalf("Agent = %#v", updated.Agent)
	}
	if updated.Agent.ListenUrl != "http://127.0.0.1:9030" || updated.Agent.ConnectUrl != "http://127.0.0.1:9030" {
		t.Fatalf("Agent listen/connect = %q/%q, want default listen URL", updated.Agent.ListenUrl, updated.Agent.ConnectUrl)
	}
	configPath := filepath.Join(cwd, FileName)
	configData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile(config) error = %v", err)
	}
	configContent := string(configData)
	for _, want := range []string{"agent:", "device_id:", "device_name:"} {
		if !strings.Contains(configContent, want) {
			t.Fatalf("local config missing %q: %s", want, configContent)
		}
	}
	if strings.Contains(configContent, "auth:") {
		t.Fatalf("local config unexpectedly contains auth block: %s", configContent)
	}

	reloaded, secondBootstrap, err := EnsureLocalIdentity(updated)
	if err != nil {
		t.Fatalf("second EnsureLocalIdentity() error = %v", err)
	}
	if secondBootstrap.Generated {
		t.Fatal("second BootstrapResult.Generated = true, want false")
	}
	if reloaded.Auth != updated.Auth {
		t.Fatalf("reloaded Auth = %#v, want %#v", reloaded.Auth, updated.Auth)
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
	clearTermBridgeEnv(t)
	t.Setenv("TERMBRIDGE_JWT__SECRET_KEY", "test-secret-key-for-tests")
	return home
}

func clearTermBridgeEnv(t *testing.T) {
	t.Helper()
	for _, key := range configKeys() {
		name := envNameForKey(key)
		previous, existed := os.LookupEnv(name)
		if err := os.Unsetenv(name); err != nil {
			t.Fatalf("Unsetenv(%s) error = %v", name, err)
		}
		t.Cleanup(func() {
			if existed {
				_ = os.Setenv(name, previous)
				return
			}
			_ = os.Unsetenv(name)
		})
	}
}

func configureRemoteAuth(t *testing.T) {
	t.Helper()
	t.Setenv("TERMBRIDGE_AUTH__GOOGLE__CLIENT_ID", "google-client-id")
	t.Setenv("TERMBRIDGE_AUTH__GOOGLE__CLIENT_SECRET", "google-client-secret")
	t.Setenv("TERMBRIDGE_AUTH__GOOGLE__REDIRECT_URL", "https://gate.example.com/oauth/callback")
	t.Setenv("TERMBRIDGE_RESEND__API_KEY", "resend-key")
	t.Setenv("TERMBRIDGE_RESEND__FROM_EMAIL", "noreply@example.com")
}

func writeConfig(t *testing.T, dir string, content string) {
	t.Helper()
	writeDefaultConfig(t, dir)
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(content), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
}

func writeDotEnv(t *testing.T, dir string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, EnvFileName), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(.env) error = %v", err)
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
