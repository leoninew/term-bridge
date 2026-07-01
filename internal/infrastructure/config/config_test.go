package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	apperrors "termbridge-go/internal/infrastructure/errors"
	"termbridge-go/internal/infrastructure/security"
)

func TestLoadDefaults(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)

	cfg, err := Load(Options{Cwd: cwd, Command: []string{"pwsh"}})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.DefaultConfigFile != filepath.Join(cwd, ConfigDirName, DefaultFileName) {
		t.Fatalf("DefaultConfigFile = %q", cfg.DefaultConfigFile)
	}
	if cfg.Environment != "" || cfg.EnvConfigFile != "" {
		t.Fatalf("Environment = %q EnvConfigFile = %q, want empty", cfg.Environment, cfg.EnvConfigFile)
	}
	if cfg.EnvFile != filepath.Join(cwd, EnvFileName) {
		t.Fatalf("EnvFile = %q, want active .env path", cfg.EnvFile)
	}
	if cfg.Base == nil {
		t.Fatal("Base = nil, want YAML-only baseline config")
	}
	if cfg.Base.Base != nil {
		t.Fatalf("Base.Base = %#v, want nil", cfg.Base.Base)
	}
	if cfg.Base.JWT.SecretKey != "" {
		t.Fatalf("Base.JWT.SecretKey = %q, want default YAML value", cfg.Base.JWT.SecretKey)
	}
	if cfg.JWT.SecretKey != "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=" {
		t.Fatalf("JWT.SecretKey = %q, want env value", cfg.JWT.SecretKey)
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
	if cfg.LogHTTP.RequestBodyLimit != 4096 || cfg.LogHTTP.ResponseBodyLimit != 4096 {
		t.Fatalf("LogHTTP = %#v", cfg.LogHTTP)
	}
	wantStateDir := filepath.Join(cwd, ".termbridge")
	if filepath.Clean(cfg.Runtime.StateDir) != filepath.Clean(wantStateDir) {
		t.Fatalf("StateDir = %q, want %q", cfg.Runtime.StateDir, wantStateDir)
	}
	if cfg.History.MaxLines != 10000 || cfg.History.MaxBytes != 5242880 || cfg.History.MaxLineBytes != 65536 {
		t.Fatalf("History = %#v", cfg.History)
	}
	if !reflect.DeepEqual(cfg.Gate.Browser.AllowedOrigins, []string{"http://127.0.0.1:9031", "http://localhost:9031"}) {
		t.Fatalf("Gate.Browser.AllowedOrigins = %#v", cfg.Gate.Browser.AllowedOrigins)
	}
	if cfg.Gate.API.ExposeErrors {
		t.Fatal("Gate.API.ExposeErrors = true, want false")
	}
	if cfg.Agent.ListenUrl != "http://127.0.0.1:9030" || cfg.Agent.ConnectUrl != "http://127.0.0.1:9030" {
		t.Fatalf("Agent listen/connect = %q/%q, want default listen URL", cfg.Agent.ListenUrl, cfg.Agent.ConnectUrl)
	}
	if cfg.Agent.PublicUrl != "http://localhost:9031" {
		t.Fatalf("Agent.PublicUrl = %q, want local frontend URL", cfg.Agent.PublicUrl)
	}
	if cfg.Agent.DeviceId != "" || cfg.Agent.DeviceName != "" {
		t.Fatalf("Agent = %#v, want empty identity", cfg.Agent)
	}
	if cfg.Cloud.GateUrl != "" {
		t.Fatalf("Cloud.GateUrl = %q, want empty by default", cfg.Cloud.GateUrl)
	}
	if cfg.Cloud.OAuth.ClientID != "termbridge-local" || cfg.Cloud.OAuth.ClientSecret != "" || cfg.Cloud.OAuth.RedirectURL != "http://localhost:9031/cloud/oauth/callback" {
		t.Fatalf("Cloud.OAuth = %#v", cfg.Cloud.OAuth)
	}
	if !reflect.DeepEqual(cfg.Cloud.OAuth.Scopes, []string{"openid", "email", "profile"}) {
		t.Fatalf("Cloud.OAuth.Scopes = %#v", cfg.Cloud.OAuth.Scopes)
	}
	if cfg.Auth.Username != DefaultAuthUsername || cfg.Auth.Password != DefaultAuthPassword {
		t.Fatalf("Auth = %#v, want default PoC auth", cfg.Auth)
	}
	if cfg.Web.StaticDir != "" {
		t.Fatalf("Web.StaticDir = %q, want empty", cfg.Web.StaticDir)
	}
	if !reflect.DeepEqual(cfg.Command, []string{"pwsh"}) {
		t.Fatalf("Command = %#v", cfg.Command)
	}
}

func TestLoadMergesEnvironmentConfig(t *testing.T) {
	isolateHome(t)
	configureRemoteAuth(t)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	logDir := filepath.Join(cwd, "configured-logs")
	stateDir := filepath.Join(cwd, "configured-state")
	writeEnvConfig(t, cwd, "develop", "log:\n  level: debug\n  format: json\n  dir: "+filepath.ToSlash(logDir)+"\n  http:\n    request_body_limit: 128\n    response_body_limit: 256\nhistory:\n  max_lines: 42\n  max_bytes: 2048\n  max_line_bytes: 128\nruntime:\n  state_dir: "+filepath.ToSlash(stateDir)+"\nweb:\n  static_dir: web/dist\ngate:\n  browser:\n    allowed_origins:\n      - \" http://127.0.0.1:9031 \"\n      - http://127.0.0.1:9031\n  api:\n    expose_errors: true\nagent:\n  listen_url: http://0.0.0.0:9090\n  public_url: https://configured.example.com/app/\n  connect_url: https://gate.example.com:9443/\n  device_id: dev-1\n  device_name: local-mac\n")
	t.Setenv(EnvNameVariable, "develop")

	cfg, err := Load(Options{Cwd: cwd})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	wantEnvConfig := filepath.Join(cwd, ConfigDirName, envConfigFileName("develop"))
	if cfg.Environment != "develop" || cfg.EnvConfigFile != wantEnvConfig {
		t.Fatalf("Environment = %q EnvConfigFile = %q", cfg.Environment, cfg.EnvConfigFile)
	}
	if cfg.LogLevel != "debug" || cfg.LogFormat != "json" {
		t.Fatalf("log config = %s/%s", cfg.LogLevel, cfg.LogFormat)
	}
	if filepath.Clean(cfg.LogDir) != filepath.Clean(logDir) {
		t.Fatalf("LogDir = %q, want %q", cfg.LogDir, logDir)
	}
	if cfg.LogHTTP.RequestBodyLimit != 128 || cfg.LogHTTP.ResponseBodyLimit != 256 {
		t.Fatalf("LogHTTP = %#v", cfg.LogHTTP)
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
	if cfg.Agent.ListenUrl != "http://0.0.0.0:9090" || cfg.Agent.ConnectUrl != "https://gate.example.com:9443" {
		t.Fatalf("Agent listen/connect = %q/%q", cfg.Agent.ListenUrl, cfg.Agent.ConnectUrl)
	}
	if !reflect.DeepEqual(cfg.Gate.Browser.AllowedOrigins, []string{"http://127.0.0.1:9031", "http://127.0.0.1:9031"}) {
		t.Fatalf("Gate.Browser.AllowedOrigins = %#v", cfg.Gate.Browser.AllowedOrigins)
	}
	if !cfg.Gate.API.ExposeErrors {
		t.Fatal("Gate.API.ExposeErrors = false, want true")
	}
	if cfg.Agent.DeviceId != "dev-1" || cfg.Agent.DeviceName != "local-mac" {
		t.Fatalf("Agent = %#v", cfg.Agent)
	}
	if cfg.Base == nil || cfg.Base.Agent.PublicUrl != cfg.Agent.PublicUrl {
		t.Fatalf("Base.Agent.PublicUrl = %#v, want YAML-only env config value", cfg.Base)
	}
}

func TestLoadDotEnvOverridesDefaultYAMLAndBaseIgnoresEnv(t *testing.T) {
	isolateHome(t)
	configureRemoteAuth(t)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	logDir := filepath.Join(cwd, "dotenv-logs")
	stateDir := filepath.Join(cwd, "dotenv-state")
	writeDotEnv(t, cwd, "TERMBRIDGE_LOG__LEVEL=debug\nTERMBRIDGE_LOG__FORMAT=json\nTERMBRIDGE_LOG__DIR="+filepath.ToSlash(logDir)+"\nTERMBRIDGE_LOG__HTTP__REQUEST_BODY_LIMIT=512\nTERMBRIDGE_LOG__HTTP__RESPONSE_BODY_LIMIT=1024\nTERMBRIDGE_HISTORY__MAX_LINES=20\nTERMBRIDGE_HISTORY__MAX_BYTES=4096\nTERMBRIDGE_HISTORY__MAX_LINE_BYTES=256\nTERMBRIDGE_RUNTIME__STATE_DIR="+filepath.ToSlash(stateDir)+"\nTERMBRIDGE_WEB__STATIC_DIR=/opt/termbridge/web/dist\nTERMBRIDGE_AGENT__PUBLIC_URL=https://dotenv.example.com\nTERMBRIDGE_AGENT__LISTEN_URL=http://127.0.0.1:9091\nTERMBRIDGE_GATE__BROWSER__ALLOWED_ORIGINS=http://127.0.0.1:9031, http://127.0.0.1:9031,,\nTERMBRIDGE_GATE__API__EXPOSE_ERRORS=true\nTERMBRIDGE_AGENT__CONNECT_URL=https://gate.example.com\nTERMBRIDGE_AGENT__DEVICE_ID=dotenv-device\nTERMBRIDGE_AGENT__DEVICE_NAME=dotenv-name\n")

	cfg, err := Load(Options{Cwd: cwd})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.LogLevel != "debug" || cfg.LogFormat != "json" {
		t.Fatalf("log config = %s/%s", cfg.LogLevel, cfg.LogFormat)
	}
	if filepath.Clean(cfg.LogDir) != filepath.Clean(logDir) || filepath.Clean(cfg.Runtime.StateDir) != filepath.Clean(stateDir) {
		t.Fatalf("paths = %q/%q", cfg.LogDir, cfg.Runtime.StateDir)
	}
	if cfg.LogHTTP.RequestBodyLimit != 512 || cfg.LogHTTP.ResponseBodyLimit != 1024 {
		t.Fatalf("LogHTTP = %#v", cfg.LogHTTP)
	}
	if cfg.History.MaxLines != 20 || cfg.History.MaxBytes != 4096 || cfg.History.MaxLineBytes != 256 {
		t.Fatalf("History = %#v", cfg.History)
	}
	if cfg.Web.StaticDir != filepath.Clean("/opt/termbridge/web/dist") {
		t.Fatalf("Web.StaticDir = %q", cfg.Web.StaticDir)
	}
	if cfg.Agent.PublicUrl != "https://dotenv.example.com" || cfg.Agent.ListenUrl != "http://127.0.0.1:9091" || cfg.Agent.ConnectUrl != "https://gate.example.com" {
		t.Fatalf("Agent = %#v", cfg.Agent)
	}
	if !reflect.DeepEqual(cfg.Gate.Browser.AllowedOrigins, []string{"http://127.0.0.1:9031", "http://127.0.0.1:9031"}) {
		t.Fatalf("Gate.Browser.AllowedOrigins = %#v", cfg.Gate.Browser.AllowedOrigins)
	}
	if !cfg.Gate.API.ExposeErrors {
		t.Fatal("Gate.API.ExposeErrors = false, want true")
	}
	if cfg.Agent.DeviceId != "dotenv-device" || cfg.Agent.DeviceName != "dotenv-name" {
		t.Fatalf("Agent identity = %#v", cfg.Agent)
	}
	if cfg.Base == nil {
		t.Fatal("Base = nil, want YAML-only baseline config")
	}
	if cfg.Base.LogLevel != "info" || cfg.Base.Agent.PublicUrl != "http://localhost:9031" || cfg.Base.Agent.DeviceId != "" {
		t.Fatalf("Base = %#v, want default YAML values", cfg.Base)
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
	if cfg.Agent.ListenUrl != "http://127.0.0.1:9092" || cfg.Agent.ConnectUrl != "https://gate.example.com" {
		t.Fatalf("Agent listen/connect = %q/%q", cfg.Agent.ListenUrl, cfg.Agent.ConnectUrl)
	}
	if !cfg.Gate.API.ExposeErrors {
		t.Fatal("Gate.API.ExposeErrors = false, want true")
	}
}

func TestLoadEnvironmentDotEnvOverridesYAMLAndSkipsDefaultDotEnv(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	writeEnvConfig(t, cwd, "develop", "log:\n  level: warn\nagent:\n  listen_url: http://127.0.0.1:9040\n  connect_url: http://127.0.0.1:9040\n")
	writeDotEnvFile(t, cwd, EnvFileName, "TERMBRIDGE_LOG__LEVEL=error\nTERMBRIDGE_AGENT__LISTEN_URL=http://127.0.0.1:8080\nTERMBRIDGE_AGENT__CONNECT_URL=http://127.0.0.1:8080\n")
	writeDotEnvFile(t, cwd, EnvFileName+".develop", "TERMBRIDGE_LOG__LEVEL=debug\nTERMBRIDGE_AGENT__LISTEN_URL=http://127.0.0.1:9093\nTERMBRIDGE_AGENT__CONNECT_URL=http://127.0.0.1:9093\n")
	t.Setenv(EnvNameVariable, "develop")

	cfg, err := Load(Options{Cwd: cwd})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.EnvFile != filepath.Join(cwd, EnvFileName+".develop") {
		t.Fatalf("EnvFile = %q", cfg.EnvFile)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("LogLevel = %q, want .env.develop override", cfg.LogLevel)
	}
	if cfg.Agent.ListenUrl != "http://127.0.0.1:9093" || cfg.Agent.ConnectUrl != "http://127.0.0.1:9093" {
		t.Fatalf("Agent listen/connect = %q/%q, want .env.develop override", cfg.Agent.ListenUrl, cfg.Agent.ConnectUrl)
	}
	if cfg.Base == nil || cfg.Base.LogLevel != "warn" || cfg.Base.Agent.ListenUrl != "http://127.0.0.1:9040" {
		t.Fatalf("Base = %#v, want YAML-only env config value", cfg.Base)
	}
}

func TestLoadEnvironmentNameComesOnlyFromOSEnv(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	writeEnvConfig(t, cwd, "develop", "log:\n  level: debug\n")
	writeDotEnvFile(t, cwd, EnvFileName, EnvNameVariable+"=develop\nTERMBRIDGE_LOG__LEVEL=warn\n")

	cfg, err := Load(Options{Cwd: cwd})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Environment != "" || cfg.EnvConfigFile != "" {
		t.Fatalf("Environment = %q EnvConfigFile = %q, want empty", cfg.Environment, cfg.EnvConfigFile)
	}
	if cfg.EnvFile != filepath.Join(cwd, EnvFileName) {
		t.Fatalf("EnvFile = %q, want default .env", cfg.EnvFile)
	}
	if cfg.LogLevel != "warn" {
		t.Fatalf("LogLevel = %q, want default .env value", cfg.LogLevel)
	}
	if got := os.Getenv(EnvNameVariable); got != "develop" {
		t.Fatalf("%s = %q, want .env value loaded into process env", EnvNameVariable, got)
	}
}

func TestLoadOSEnvOverridesEnvironmentDotEnv(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	writeDotEnvFile(t, cwd, EnvFileName+".develop", "TERMBRIDGE_AGENT__LISTEN_URL=http://127.0.0.1:8080\nTERMBRIDGE_AGENT__CONNECT_URL=http://127.0.0.1:8080\n")
	t.Setenv(EnvNameVariable, "develop")
	t.Setenv("TERMBRIDGE_AGENT__LISTEN_URL", "http://127.0.0.1:9094")
	t.Setenv("TERMBRIDGE_AGENT__CONNECT_URL", "http://127.0.0.1:9094")

	cfg, err := Load(Options{Cwd: cwd})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Agent.ListenUrl != "http://127.0.0.1:9094" || cfg.Agent.ConnectUrl != "http://127.0.0.1:9094" {
		t.Fatalf("Agent listen/connect = %q/%q, want OS env override", cfg.Agent.ListenUrl, cfg.Agent.ConnectUrl)
	}
}

func TestLoadIgnoresMissingEnvironmentConfigAndDotEnv(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	t.Setenv(EnvNameVariable, "develop")

	loadedFiles := []string{}
	cfg, err := Load(Options{Cwd: cwd, LoadedConfigFile: func(path string) {
		loadedFiles = append(loadedFiles, path)
	}})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Environment != "develop" {
		t.Fatalf("Environment = %q, want develop", cfg.Environment)
	}
	if cfg.EnvConfigFile != "" {
		t.Fatalf("EnvConfigFile = %q, want empty", cfg.EnvConfigFile)
	}
	if cfg.EnvFile != filepath.Join(cwd, EnvFileName+".develop") {
		t.Fatalf("EnvFile = %q, want .env.develop path", cfg.EnvFile)
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("LogLevel = %q, want default", cfg.LogLevel)
	}
	wantLoadedFiles := []string{filepath.Join(cwd, ConfigDirName, DefaultFileName)}
	if !reflect.DeepEqual(loadedFiles, wantLoadedFiles) {
		t.Fatalf("loaded files = %#v, want %#v", loadedFiles, wantLoadedFiles)
	}
}

func TestLoadReportsActuallyLoadedConfigFilesInOrder(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	writeEnvConfig(t, cwd, "develop", "log:\n  level: warn\n")
	writeDotEnvFile(t, cwd, EnvFileName+".develop", "TERMBRIDGE_LOG__LEVEL=debug\n")
	t.Setenv(EnvNameVariable, "develop")

	loadedFiles := []string{}
	_, err := Load(Options{Cwd: cwd, LoadedConfigFile: func(path string) {
		loadedFiles = append(loadedFiles, path)
	}})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	wantLoadedFiles := []string{
		filepath.Join(cwd, ConfigDirName, DefaultFileName),
		filepath.Join(cwd, ConfigDirName, envConfigFileName("develop")),
		filepath.Join(cwd, EnvFileName+".develop"),
	}
	if !reflect.DeepEqual(loadedFiles, wantLoadedFiles) {
		t.Fatalf("loaded files = %#v, want %#v", loadedFiles, wantLoadedFiles)
	}
}

func TestLoadRejectsInvalidEnvironmentDotEnv(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	writeDotEnvFile(t, cwd, EnvFileName+".develop", "TERMBRIDGE_LOG__LEVEL='unterminated")
	t.Setenv(EnvNameVariable, "develop")

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

func TestLoadRejectsInvalidLogLevel(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	writeEnvConfig(t, cwd, "develop", "log:\n  level: trace\n")
	t.Setenv(EnvNameVariable, "develop")

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
	writeDefaultConfig(t, cwd)
	writeEnvConfig(t, cwd, "develop", "history:\n  max_lines: 0\n")
	t.Setenv(EnvNameVariable, "develop")

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
	writeDefaultConfig(t, cwd)
	writeEnvConfig(t, cwd, "develop", "agent:\n  listen_url: not-a-url\n")
	t.Setenv(EnvNameVariable, "develop")

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
	writeDefaultConfig(t, cwd)
	writeEnvConfig(t, cwd, "develop", "agent:\n  public_url: ftp://example.com\n")
	t.Setenv(EnvNameVariable, "develop")

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
	writeDefaultConfig(t, cwd)
	writeEnvConfig(t, cwd, "develop", "agent:\n  connect_url: ftp://example.com\n")
	t.Setenv(EnvNameVariable, "develop")

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
	writeDefaultConfig(t, cwd)
	writeEnvConfig(t, cwd, "develop", "log:\n  http:\n    request_body_limit: 0\n    response_body_limit: -1\n")
	t.Setenv(EnvNameVariable, "develop")

	cfg, err := Load(Options{Cwd: cwd})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.LogHTTP.RequestBodyLimit != 0 || cfg.LogHTTP.ResponseBodyLimit != 0 {
		t.Fatalf("LogHTTP = %#v", cfg.LogHTTP)
	}
}

func TestLoadGeneratesMissingJWTSecretKey(t *testing.T) {
	isolateHome(t)
	if err := os.Unsetenv("TERMBRIDGE_JWT__SECRET_KEY"); err != nil {
		t.Fatalf("Unsetenv(TERMBRIDGE_JWT__SECRET_KEY) error = %v", err)
	}
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)

	cfg, err := Load(Options{Cwd: cwd})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if _, err := security.ParseBase64Key(cfg.JWT.SecretKey, 32); err != nil {
		t.Fatalf("JWT.SecretKey = %q, want generated fernet key: %v", cfg.JWT.SecretKey, err)
	}
	data, err := os.ReadFile(filepath.Join(cwd, EnvFileName))
	if err != nil {
		t.Fatalf("ReadFile(.env) error = %v", err)
	}
	if !strings.Contains(string(data), "TERMBRIDGE_JWT__SECRET_KEY="+quoteEnvValue(cfg.JWT.SecretKey)) {
		t.Fatalf("env file missing generated jwt secret key: %s", string(data))
	}
}

func TestLoadGeneratesMissingJWTSecretKeyIntoEnvironmentConfig(t *testing.T) {
	isolateHome(t)
	if err := os.Unsetenv("TERMBRIDGE_JWT__SECRET_KEY"); err != nil {
		t.Fatalf("Unsetenv(TERMBRIDGE_JWT__SECRET_KEY) error = %v", err)
	}
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	t.Setenv(EnvNameVariable, "develop")

	cfg, err := Load(Options{Cwd: cwd})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if _, err := security.ParseBase64Key(cfg.JWT.SecretKey, 32); err != nil {
		t.Fatalf("JWT.SecretKey = %q, want generated fernet key: %v", cfg.JWT.SecretKey, err)
	}
	if _, err := os.Stat(filepath.Join(cwd, EnvFileName+".develop")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("environment .env file exists err = %v, want not exist", err)
	}
	path := filepath.Join(cwd, ConfigDirName, envConfigFileName("develop"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(config.develop.yaml) error = %v", err)
	}
	if !strings.Contains(string(data), "secret_key: "+cfg.JWT.SecretKey) {
		t.Fatalf("env config file missing generated jwt secret key: %s", string(data))
	}
}

func TestLoadUpsertsGeneratedValuesIntoExistingEnvironmentConfig(t *testing.T) {
	isolateHome(t)
	if err := os.Unsetenv("TERMBRIDGE_JWT__SECRET_KEY"); err != nil {
		t.Fatalf("Unsetenv(TERMBRIDGE_JWT__SECRET_KEY) error = %v", err)
	}
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	writeEnvConfig(t, cwd, "develop", "log:\n  level: debug\nagent:\n  listen_url: http://127.0.0.1:9040\n")
	t.Setenv(EnvNameVariable, "develop")

	loadedFiles := []string{}
	cfg, err := Load(Options{Cwd: cwd, LoadedConfigFile: func(path string) {
		loadedFiles = append(loadedFiles, path)
	}})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	updated, _, err := EnsureLocalIdentity(cfg)
	if err != nil {
		t.Fatalf("EnsureLocalIdentity() error = %v", err)
	}
	path := filepath.Join(cwd, ConfigDirName, envConfigFileName("develop"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(config.develop.yaml) error = %v", err)
	}
	content := string(data)
	for _, want := range []string{
		"level: debug",
		"listen_url: http://127.0.0.1:9040",
		"secret_key: " + cfg.JWT.SecretKey,
		"device_id: " + updated.Agent.DeviceId,
		"device_name: " + updated.Agent.DeviceName,
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("env config file missing %q: %s", want, content)
		}
	}
	wantLoadedFiles := []string{
		filepath.Join(cwd, ConfigDirName, DefaultFileName),
		path,
	}
	if !reflect.DeepEqual(loadedFiles, wantLoadedFiles) {
		t.Fatalf("loaded files = %#v, want %#v", loadedFiles, wantLoadedFiles)
	}
}

func TestLoadRejectsInvalidJWTSecretKey(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	t.Setenv("TERMBRIDGE_JWT__SECRET_KEY", "test-secret")

	_, err := Load(Options{Cwd: cwd})
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "invalid jwt.secret_key") {
		t.Fatalf("Load() error = %v, want jwt secret key error", err)
	}
}

func TestLoadRejectsIncompleteIntegrationConfig(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	t.Setenv("TERMBRIDGE_RESEND__API_KEY", "resend-key")

	_, err := Load(Options{Cwd: cwd})
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "TERMBRIDGE_RESEND__FROM_EMAIL") {
		t.Fatalf("Load() error = %v, want missing resend from email", err)
	}
}

func TestLoadCloudModeRequiresRegisteredLocalOAuthClient(t *testing.T) {
	isolateHome(t)
	configureRemoteAuth(t)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	writeEnvConfig(t, cwd, "cloud", "web:\n  mode: cloud\ncloud:\n  oauth:\n    client_id: ''\n    redirect_url: ''\n")
	t.Setenv(EnvNameVariable, "cloud")

	_, err := Load(Options{Cwd: cwd})
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "TERMBRIDGE_CLOUD__OAUTH__CLIENT_ID") || !strings.Contains(err.Error(), "TERMBRIDGE_CLOUD__OAUTH__REDIRECT_URL") {
		t.Fatalf("Load() error = %v, want missing cloud OAuth client configuration", err)
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

func TestLoadIgnoresUnknownConfigAndEnvironmentKeys(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	writeEnvConfig(t, cwd, "develop", "unknown_root:\n  value: ignored\nagent:\n  listen_url: http://127.0.0.1:9045\n  connect_url: http://127.0.0.1:9045\n  unknown_field: ignored\n")
	writeDotEnvFile(t, cwd, EnvFileName+".develop", "TERMBRIDGE_UNKNOWN__FIELD=ignored\nTERMBRIDGE_LOG__LEVEL=debug\n")
	t.Setenv(EnvNameVariable, "develop")

	cfg, err := Load(Options{Cwd: cwd})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Agent.ListenUrl != "http://127.0.0.1:9045" || cfg.Agent.ConnectUrl != "http://127.0.0.1:9045" {
		t.Fatalf("Agent listen/connect = %q/%q", cfg.Agent.ListenUrl, cfg.Agent.ConnectUrl)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("LogLevel = %q, want known .env override", cfg.LogLevel)
	}
}

func TestEnsureLocalIdentityWritesAgentIdentityToActiveEnvFile(t *testing.T) {
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
	if bootstrap.GeneratedConfigFile != filepath.Join(cwd, EnvFileName) {
		t.Fatalf("BootstrapResult.GeneratedConfigFile = %q", bootstrap.GeneratedConfigFile)
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
	data, err := os.ReadFile(filepath.Join(cwd, EnvFileName))
	if err != nil {
		t.Fatalf("ReadFile(.env) error = %v", err)
	}
	content := string(data)
	for _, want := range []string{
		"TERMBRIDGE_AGENT__DEVICE_ID=" + quoteEnvValue(updated.Agent.DeviceId),
		"TERMBRIDGE_AGENT__DEVICE_NAME=" + quoteEnvValue(updated.Agent.DeviceName),
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("env file missing %q: %s", want, content)
		}
	}
	if strings.Contains(content, "TERMBRIDGE_AUTH") {
		t.Fatalf("env file unexpectedly contains auth block: %s", content)
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

func TestEnsureLocalIdentityWritesEnvironmentIdentityToEnvironmentConfig(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	t.Setenv(EnvNameVariable, "develop")

	cfg, err := Load(Options{Cwd: cwd})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	updated, bootstrap, err := EnsureLocalIdentity(cfg)
	if err != nil {
		t.Fatalf("EnsureLocalIdentity() error = %v", err)
	}
	want := filepath.Join(cwd, ConfigDirName, envConfigFileName("develop"))
	if bootstrap.GeneratedConfigFile != want {
		t.Fatalf("GeneratedConfigFile = %q, want %q", bootstrap.GeneratedConfigFile, want)
	}
	if updated.EnvFile != filepath.Join(cwd, EnvFileName+".develop") {
		t.Fatalf("EnvFile = %q, want active .env.develop path", updated.EnvFile)
	}
	if _, err := os.Stat(updated.EnvFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("environment .env file exists err = %v, want not exist", err)
	}
	data, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("ReadFile(config.develop.yaml) error = %v", err)
	}
	content := string(data)
	for _, want := range []string{
		"device_id: " + updated.Agent.DeviceId,
		"device_name: " + updated.Agent.DeviceName,
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("env config file missing %q: %s", want, content)
		}
	}
}

func TestEnsureLocalIdentityUpsertsExistingActiveEnvFile(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	writeDotEnv(t, cwd, "TERMBRIDGE_LOG__LEVEL=debug\nTERMBRIDGE_AGENT__DEVICE_ID=stale-device\n")

	cfg, err := Load(Options{Cwd: cwd})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	cfg.Agent.DeviceId = ""
	updated, bootstrap, err := EnsureLocalIdentity(cfg)
	if err != nil {
		t.Fatalf("EnsureLocalIdentity() error = %v", err)
	}
	if bootstrap.GeneratedConfigFile != filepath.Join(cwd, EnvFileName) {
		t.Fatalf("BootstrapResult.GeneratedConfigFile = %q", bootstrap.GeneratedConfigFile)
	}
	data, err := os.ReadFile(filepath.Join(cwd, EnvFileName))
	if err != nil {
		t.Fatalf("ReadFile(.env) error = %v", err)
	}
	content := string(data)
	for _, want := range []string{
		"TERMBRIDGE_LOG__LEVEL=debug",
		"TERMBRIDGE_AGENT__DEVICE_ID=" + quoteEnvValue(updated.Agent.DeviceId),
		"TERMBRIDGE_AGENT__DEVICE_NAME=" + quoteEnvValue(updated.Agent.DeviceName),
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("env file missing %q: %s", want, content)
		}
	}
	if strings.Contains(content, "stale-device") {
		t.Fatalf("env file still contains stale identity: %s", content)
	}
}

func TestDefaultConfigFileExists(t *testing.T) {
	if _, err := os.ReadFile(repoDefaultConfigPath(t)); err != nil {
		t.Fatalf("ReadFile(default config) error = %v", err)
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

func isolateHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	clearTermBridgeEnv(t)
	t.Setenv("TERMBRIDGE_JWT__SECRET_KEY", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
	return home
}

func clearTermBridgeEnv(t *testing.T) {
	t.Helper()
	previousEnvironment, environmentExisted := os.LookupEnv(EnvNameVariable)
	if err := os.Unsetenv(EnvNameVariable); err != nil {
		t.Fatalf("Unsetenv(%s) error = %v", EnvNameVariable, err)
	}
	t.Cleanup(func() {
		if environmentExisted {
			_ = os.Setenv(EnvNameVariable, previousEnvironment)
			return
		}
		_ = os.Unsetenv(EnvNameVariable)
	})
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

func writeEnvConfig(t *testing.T, dir string, environment string, content string) {
	t.Helper()
	writeConfigFile(t, dir, filepath.Join(ConfigDirName, envConfigFileName(environment)), content)
}

func writeConfigFile(t *testing.T, dir string, name string, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", name, err)
	}
}

func writeDotEnv(t *testing.T, dir string, content string) {
	t.Helper()
	writeDotEnvFile(t, dir, EnvFileName, content)
}

func writeDotEnvFile(t *testing.T, dir string, name string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", name, err)
	}
}

func writeDefaultConfig(t *testing.T, dir string) {
	t.Helper()
	content, err := os.ReadFile(repoDefaultConfigPath(t))
	if err != nil {
		t.Fatalf("ReadFile(default config) error = %v", err)
	}
	writeConfigFile(t, dir, filepath.Join(ConfigDirName, DefaultFileName), string(content))
}

func repoDefaultConfigPath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ConfigDirName, DefaultFileName))
}
