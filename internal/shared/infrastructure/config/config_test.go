package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	apperrors "gitee.com/leoninew/TermBridge-go/internal/shared/common/errors"
	"gitee.com/leoninew/TermBridge-go/internal/shared/common/security"
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
	if cfg.Base.Jwt.SecretKey != "" {
		t.Fatalf("Base.Jwt.SecretKey = %q, want default YAML value", cfg.Base.Jwt.SecretKey)
	}
	if cfg.Jwt.SecretKey != "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=" {
		t.Fatalf("Jwt.SecretKey = %q, want env value", cfg.Jwt.SecretKey)
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
	wantStateDir := filepath.Join(cwd, "data")
	if filepath.Clean(cfg.Runtime.StateDir) != filepath.Clean(wantStateDir) {
		t.Fatalf("StateDir = %q, want %q", cfg.Runtime.StateDir, wantStateDir)
	}
	if cfg.History.MaxLines != 10000 || cfg.History.MaxBytes != 5242880 || cfg.History.MaxLineBytes != 65536 {
		t.Fatalf("History = %#v", cfg.History)
	}
	if cfg.Terminal.Replay.MaxBytes != 1048576 || cfg.Terminal.Replay.ChunkBytes != 65536 {
		t.Fatalf("Terminal.Replay = %#v", cfg.Terminal.Replay)
	}
	if cfg.Terminal.Client.Queue.MaxMessages != 64 || cfg.Terminal.Client.Queue.MaxBytes != 4194304 {
		t.Fatalf("Terminal.Client.Queue = %#v", cfg.Terminal.Client.Queue)
	}
	if cfg.Local.ExposeErrors {
		t.Fatal("Local.ExposeErrors = true, want false")
	}
	if cfg.Local.ListenUrl != "http://127.0.0.1:9030" {
		t.Fatalf("Local.ListenUrl = %q, want default listen URL", cfg.Local.ListenUrl)
	}
	if cfg.Local.PublicUrl != "http://localhost:9030" {
		t.Fatalf("Local.PublicUrl = %q, want local frontend URL", cfg.Local.PublicUrl)
	}
	if cfg.Local.ApiBaseUrl != "" {
		t.Fatalf("Local.ApiBaseUrl = %q, want empty", cfg.Local.ApiBaseUrl)
	}
	if len(cfg.Local.CorsAllowedOrigins) != 0 {
		t.Fatalf("Local.CorsAllowedOrigins = %#v, want empty", cfg.Local.CorsAllowedOrigins)
	}
	if cfg.Cloud.ExposeErrors {
		t.Fatal("Cloud.ExposeErrors = true, want false")
	}
	if cfg.Cloud.ListenUrl != "http://127.0.0.1:9030" {
		t.Fatalf("Cloud.ListenUrl = %q, want default listen URL", cfg.Cloud.ListenUrl)
	}
	if cfg.Cloud.PublicUrl != "" {
		t.Fatalf("Cloud.PublicUrl = %q, want empty by default", cfg.Cloud.PublicUrl)
	}
	if cfg.Auth.LocalAdmin.Username != DefaultAuthUsername || cfg.Auth.LocalAdmin.Password != DefaultAuthPassword {
		t.Fatalf("Auth.LocalAdmin = %#v, want default PoC auth", cfg.Auth.LocalAdmin)
	}
	if cfg.Local.StaticDir != "" {
		t.Fatalf("Local.StaticDir = %q, want empty", cfg.Local.StaticDir)
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
	writeEnvConfig(t, cwd, "develop", "log:\n  level: debug\n  format: json\n  dir: "+filepath.ToSlash(logDir)+"\n  http:\n    request_body_limit: 128\n    response_body_limit: 256\nhistory:\n  max_lines: 42\n  max_bytes: 2048\n  max_line_bytes: 128\nruntime:\n  state_dir: "+filepath.ToSlash(stateDir)+"\nlocal:\n  expose_errors: true\n  listen_url: http://0.0.0.0:9090\n  static_dir: web/dist\n  public_url: https://configured.example.com/app/\n  api_base_url: https://api.configured.example.com/\n  cors_allowed_origins:\n    - https://configured.example.com/\n    - https://preview.configured.example.com\n")
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
	if filepath.Clean(cfg.Local.StaticDir) != filepath.Join(cwd, "web", "dist") {
		t.Fatalf("Local.StaticDir = %q", cfg.Local.StaticDir)
	}
	if cfg.Local.PublicUrl != "https://configured.example.com/app" {
		t.Fatalf("Local.PublicUrl = %q", cfg.Local.PublicUrl)
	}
	if cfg.Local.ApiBaseUrl != "https://api.configured.example.com" {
		t.Fatalf("Local.ApiBaseUrl = %q", cfg.Local.ApiBaseUrl)
	}
	wantOrigins := []string{"https://configured.example.com", "https://preview.configured.example.com"}
	if !reflect.DeepEqual(cfg.Local.CorsAllowedOrigins, wantOrigins) {
		t.Fatalf("Local.CorsAllowedOrigins = %#v, want %#v", cfg.Local.CorsAllowedOrigins, wantOrigins)
	}
	if cfg.History.MaxLines != 42 || cfg.History.MaxBytes != 2048 || cfg.History.MaxLineBytes != 128 {
		t.Fatalf("History = %#v", cfg.History)
	}
	if cfg.Local.ListenUrl != "http://0.0.0.0:9090" {
		t.Fatalf("Local.ListenUrl = %q", cfg.Local.ListenUrl)
	}
	if !cfg.Local.ExposeErrors {
		t.Fatal("Local.ExposeErrors = false, want true")
	}
	if cfg.Base == nil || cfg.Base.Local.PublicUrl != cfg.Local.PublicUrl {
		t.Fatalf("Base.Local = %#v, want YAML-only env config value", cfg.Base)
	}
}

func TestLoadDotEnvOverridesDefaultYAMLAndBaseIgnoresEnv(t *testing.T) {
	isolateHome(t)
	configureRemoteAuth(t)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	logDir := filepath.Join(cwd, "dotenv-logs")
	stateDir := filepath.Join(cwd, "dotenv-state")
	writeDotEnv(t, cwd, "TERMBRIDGE_LOG__LEVEL=debug\nTERMBRIDGE_LOG__FORMAT=json\nTERMBRIDGE_LOG__DIR="+filepath.ToSlash(logDir)+"\nTERMBRIDGE_LOG__HTTP__REQUEST_BODY_LIMIT=512\nTERMBRIDGE_LOG__HTTP__RESPONSE_BODY_LIMIT=1024\nTERMBRIDGE_HISTORY__MAX_LINES=20\nTERMBRIDGE_HISTORY__MAX_BYTES=4096\nTERMBRIDGE_HISTORY__MAX_LINE_BYTES=256\nTERMBRIDGE_RUNTIME__STATE_DIR="+filepath.ToSlash(stateDir)+"\nTERMBRIDGE_LOCAL__STATIC_DIR=/opt/termbridge/web/dist\nTERMBRIDGE_LOCAL__PUBLIC_URL=https://dotenv.example.com\nTERMBRIDGE_LOCAL__API_BASE_URL=https://api.dotenv.example.com\nTERMBRIDGE_LOCAL__CORS_ALLOWED_ORIGINS=https://dotenv.example.com,https://preview.dotenv.example.com\nTERMBRIDGE_LOCAL__LISTEN_URL=http://127.0.0.1:9091\nTERMBRIDGE_LOCAL__EXPOSE_ERRORS=true\n")

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
	if cfg.Local.StaticDir != filepath.Clean("/opt/termbridge/web/dist") {
		t.Fatalf("Local.StaticDir = %q", cfg.Local.StaticDir)
	}
	if cfg.Local.PublicUrl != "https://dotenv.example.com" || cfg.Local.ListenUrl != "http://127.0.0.1:9091" {
		t.Fatalf("Local = %#v", cfg.Local)
	}
	if cfg.Local.ApiBaseUrl != "https://api.dotenv.example.com" {
		t.Fatalf("Local.ApiBaseUrl = %q", cfg.Local.ApiBaseUrl)
	}
	wantOrigins := []string{"https://dotenv.example.com", "https://preview.dotenv.example.com"}
	if !reflect.DeepEqual(cfg.Local.CorsAllowedOrigins, wantOrigins) {
		t.Fatalf("Local.CorsAllowedOrigins = %#v, want %#v", cfg.Local.CorsAllowedOrigins, wantOrigins)
	}
	if !cfg.Local.ExposeErrors {
		t.Fatal("Local.ExposeErrors = false, want true")
	}
	if cfg.Base == nil {
		t.Fatal("Base = nil, want YAML-only baseline config")
	}
	if cfg.Base.LogLevel != "info" || cfg.Base.Local.PublicUrl != "http://localhost:9030" {
		t.Fatalf("Base = %#v, want default YAML values", cfg.Base)
	}
}

func TestLoadOSEnvOverridesDotEnv(t *testing.T) {
	isolateHome(t)
	configureRemoteAuth(t)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	writeDotEnv(t, cwd, "TERMBRIDGE_LOCAL__LISTEN_URL=http://127.0.0.1:8080\nTERMBRIDGE_LOCAL__EXPOSE_ERRORS=false\n")
	t.Setenv("TERMBRIDGE_LOCAL__LISTEN_URL", "http://127.0.0.1:9092")
	t.Setenv("TERMBRIDGE_LOCAL__EXPOSE_ERRORS", "true")

	cfg, err := Load(Options{Cwd: cwd})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Local.ListenUrl != "http://127.0.0.1:9092" {
		t.Fatalf("Local.ListenUrl = %q", cfg.Local.ListenUrl)
	}
	if !cfg.Local.ExposeErrors {
		t.Fatal("Local.ExposeErrors = false, want true")
	}
}

func TestLoadEnvironmentDotEnvOverridesYAMLAndSkipsDefaultDotEnv(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	writeEnvConfig(t, cwd, "develop", "log:\n  level: warn\nlocal:\n  listen_url: http://127.0.0.1:9040\n")
	writeDotEnvFile(t, cwd, EnvFileName, "TERMBRIDGE_LOG__LEVEL=error\nTERMBRIDGE_LOCAL__LISTEN_URL=http://127.0.0.1:8080\n")
	writeDotEnvFile(t, cwd, EnvFileName+".develop", "TERMBRIDGE_LOG__LEVEL=debug\nTERMBRIDGE_LOCAL__LISTEN_URL=http://127.0.0.1:9093\n")
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
	if cfg.Local.ListenUrl != "http://127.0.0.1:9093" {
		t.Fatalf("Local.ListenUrl = %q, want .env.develop override", cfg.Local.ListenUrl)
	}
	if cfg.Base == nil || cfg.Base.LogLevel != "warn" || cfg.Base.Local.ListenUrl != "http://127.0.0.1:9040" {
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
	writeDotEnvFile(t, cwd, EnvFileName+".develop", "TERMBRIDGE_LOCAL__LISTEN_URL=http://127.0.0.1:8080\n")
	t.Setenv(EnvNameVariable, "develop")
	t.Setenv("TERMBRIDGE_LOCAL__LISTEN_URL", "http://127.0.0.1:9094")

	cfg, err := Load(Options{Cwd: cwd})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Local.ListenUrl != "http://127.0.0.1:9094" {
		t.Fatalf("Local.ListenUrl = %q, want OS env override", cfg.Local.ListenUrl)
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
	writeEnvConfig(t, cwd, "develop", "local:\n  listen_url: not-a-url\n")
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
	writeEnvConfig(t, cwd, "develop", "local:\n  public_url: ftp://example.com\n")
	t.Setenv(EnvNameVariable, "develop")

	_, err := Load(Options{Cwd: cwd})
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
	if !apperrors.IsConfig(err) {
		t.Fatalf("Load() error = %T, want config error", err)
	}
}

func TestLoadRejectsInvalidAgentAPIBaseURL(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	writeEnvConfig(t, cwd, "develop", "local:\n  api_base_url: ftp://api.example.com\n")
	t.Setenv(EnvNameVariable, "develop")

	_, err := Load(Options{Cwd: cwd})
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
	if !apperrors.IsConfig(err) {
		t.Fatalf("Load() error = %T, want config error", err)
	}
}

func TestLoadAllowsPathAPIBaseURLForDevProxy(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	writeEnvConfig(t, cwd, "develop", "local:\n  listen_url: http://127.0.0.1:9031\n  public_url: http://localhost:9030\n  api_base_url: /local-api/\ncloud:\n  listen_url: http://127.0.0.1:9032\n  public_url: http://localhost:9030\n  api_base_url: /cloud-api/\n")
	t.Setenv(EnvNameVariable, "develop")

	cfg, err := Load(Options{Cwd: cwd})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Local.ListenUrl != "http://127.0.0.1:9031" || cfg.Local.ApiBaseUrl != "/local-api" {
		t.Fatalf("Local dev proxy config = %#v", cfg.Local)
	}
	if cfg.Cloud.ListenUrl != "http://127.0.0.1:9032" || cfg.Cloud.ApiBaseUrl != "/cloud-api" {
		t.Fatalf("Cloud dev proxy config = %#v", cfg.Cloud)
	}
}

func TestLoadRejectsInvalidAgentCORSAllowedOrigin(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	writeEnvConfig(t, cwd, "develop", "local:\n  cors_allowed_origins:\n    - chrome-extension://example\n")
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
	if _, err := security.ParseBase64Key(cfg.Jwt.SecretKey, 32); err != nil {
		t.Fatalf("Jwt.SecretKey = %q, want generated fernet key: %v", cfg.Jwt.SecretKey, err)
	}
	data, err := os.ReadFile(filepath.Join(cwd, EnvFileName))
	if err != nil {
		t.Fatalf("ReadFile(.env) error = %v", err)
	}
	if !strings.Contains(string(data), "TERMBRIDGE_JWT__SECRET_KEY="+quoteEnvValue(cfg.Jwt.SecretKey)) {
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
	if _, err := security.ParseBase64Key(cfg.Jwt.SecretKey, 32); err != nil {
		t.Fatalf("Jwt.SecretKey = %q, want generated fernet key: %v", cfg.Jwt.SecretKey, err)
	}
	if _, err := os.Stat(filepath.Join(cwd, EnvFileName+".develop")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("environment .env file exists err = %v, want not exist", err)
	}
	path := filepath.Join(cwd, ConfigDirName, envConfigFileName("develop"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(config.develop.yaml) error = %v", err)
	}
	if !strings.Contains(string(data), "secret_key: "+cfg.Jwt.SecretKey) {
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
	writeEnvConfig(t, cwd, "develop", "log:\n  level: debug\nlocal:\n  listen_url: http://127.0.0.1:9040\n")
	t.Setenv(EnvNameVariable, "develop")

	loadedFiles := []string{}
	cfg, err := Load(Options{Cwd: cwd, LoadedConfigFile: func(path string) {
		loadedFiles = append(loadedFiles, path)
	}})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
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
		"secret_key: " + cfg.Jwt.SecretKey,
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

func TestLoadLocalOAuthConfig(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	writeEnvConfig(t, cwd, "develop", "local:\n  oauth:\n    client_id: termbridge-agent\n    client_secret: agent-secret\n    redirect_url: http://localhost:9030/oauth/callback/\n    scopes: openid,email,profile\n")
	t.Setenv(EnvNameVariable, "develop")

	cfg, err := Load(Options{Cwd: cwd})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Local.OAuth.ClientId != "termbridge-agent" {
		t.Fatalf("Local.OAuth.ClientId = %q", cfg.Local.OAuth.ClientId)
	}
	if cfg.Local.OAuth.ClientSecret != "agent-secret" {
		t.Fatalf("Local.OAuth.ClientSecret = %q", cfg.Local.OAuth.ClientSecret)
	}
	if cfg.Local.OAuth.RedirectUrl != "http://localhost:9030/oauth/callback" {
		t.Fatalf("Local.OAuth.RedirectUrl = %q", cfg.Local.OAuth.RedirectUrl)
	}
	wantScopes := []string{"openid", "email", "profile"}
	if !reflect.DeepEqual(cfg.Local.OAuth.Scopes, wantScopes) {
		t.Fatalf("Local.OAuth.Scopes = %#v, want %#v", cfg.Local.OAuth.Scopes, wantScopes)
	}
}

func TestLoadRejectsIncompleteLocalOAuthConfig(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeDefaultConfig(t, cwd)
	writeEnvConfig(t, cwd, "develop", "local:\n  oauth:\n    client_id: termbridge-agent\n    client_secret: \"\"\n    redirect_url: \"\"\n")
	t.Setenv(EnvNameVariable, "develop")

	_, err := Load(Options{Cwd: cwd})
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "incomplete local OAuth configuration") {
		t.Fatalf("Load() error = %v, want incomplete local OAuth configuration", err)
	}
	if !strings.Contains(err.Error(), "TERMBRIDGE_LOCAL__OAUTH__CLIENT_SECRET") || !strings.Contains(err.Error(), "TERMBRIDGE_LOCAL__OAUTH__REDIRECT_URL") {
		t.Fatalf("Load() error = %v, want missing local OAuth env names", err)
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
	if cfg.Local.ListenUrl != "http://127.0.0.1:9030" {
		t.Fatalf("Local.ListenUrl = %q, want http://127.0.0.1:9030", cfg.Local.ListenUrl)
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
	writeEnvConfig(t, cwd, "develop", "unknown_root:\n  value: ignored\nlocal:\n  listen_url: http://127.0.0.1:9045\n  unknown_field: ignored\n")
	writeDotEnvFile(t, cwd, EnvFileName+".develop", "TERMBRIDGE_UNKNOWN__FIELD=ignored\nTERMBRIDGE_LOG__LEVEL=debug\n")
	t.Setenv(EnvNameVariable, "develop")

	cfg, err := Load(Options{Cwd: cwd})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Local.ListenUrl != "http://127.0.0.1:9045" {
		t.Fatalf("Local.ListenUrl = %q", cfg.Local.ListenUrl)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("LogLevel = %q, want known .env override", cfg.LogLevel)
	}
}

func TestDefaultConfigFileExists(t *testing.T) {
	if _, err := os.ReadFile(repoDefaultConfigPath(t)); err != nil {
		t.Fatalf("ReadFile(default config) error = %v", err)
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
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ConfigDirName, DefaultFileName))
}
