package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
	"github.com/subosito/gotenv"

	apperrors "gitee.com/leoninew/TermBridge-go/internal/shared/common/errors"
	"gitee.com/leoninew/TermBridge-go/internal/shared/common/security"
)

const (
	ConfigDirName   = "configs"
	DefaultFileName = "config.yaml"
	EnvFileName     = ".env"
	EnvPrefix       = "TERMBRIDGE"
	EnvNameVariable = EnvPrefix + "_ENV"

	DefaultTerminalReplayMaxBytes      int64 = 256 * 1024
	DefaultTerminalReplayChunkBytes          = 64 * 1024
	DefaultTerminalClientQueueMessages       = 64
	DefaultTerminalClientQueueBytes          = 4 * 1024 * 1024
	MaxGitCommandTimeout                     = 10 * time.Second
)

type Config struct {
	Cwd               string
	Command           []string
	Environment       string
	LogLevel          string
	LogFormat         string
	LogDir            string
	LogHTTP           LogHTTPConfig
	Terminal          TerminalConfig
	File              FileConfig
	Git               GitConfig
	Runtime           RuntimeConfig
	Local             LocalConfig
	Cloud             CloudConfig
	DefaultConfigFile string
	EnvConfigFile     string
	EnvFile           string
	Base              *Config
}

type JwtConfig struct {
	SecretKey string `json:"secret_key"`
	Ttl       time.Duration
}

type DatabaseConfig struct {
	Driver string
	SQLite SQLiteConfig
	MySQL  MySQLConfig
}

type SQLiteConfig struct {
	Path string
}

type MySQLConfig struct {
	Dsn string
}

type GoogleConfig struct {
	ClientId     string
	ClientSecret string
	RedirectUrl  string
}

type GitHubConfig struct {
	ClientId     string
	ClientSecret string
	RedirectUrl  string
}

type ResendConfig struct {
	ApiKey    string
	FromEmail string
}

type LogHTTPConfig struct {
	Enabled           bool
	RequestBodyLimit  int
	ResponseBodyLimit int
	SkipAssetEnabled  bool
}

type HistoryConfig struct {
	MaxLines     int
	MaxBytes     int64
	MaxLineBytes int
}

type TerminalConfig struct {
	History HistoryConfig
	Replay  TerminalReplayConfig
	Client  TerminalClientConfig
}

type TerminalReplayConfig struct {
	MaxBytes   int64
	ChunkBytes int
}

type TerminalClientConfig struct {
	Queue TerminalClientQueueConfig
}

type TerminalClientQueueConfig struct {
	MaxMessages int
	MaxBytes    int
}

type FileConfig struct {
	MaxTextBytes              int64
	MaxDirectoryEntries       int
	MaxRecursiveDeleteEntries int
	OperationTimeout          time.Duration
	WatchSubscriberQueueSize  int
}

type GitConfig struct {
	Executable     string
	CommandTimeout time.Duration
	MaxStdoutBytes int64
	MaxStderrBytes int64
	MaxTextBytes   int64
}

type RuntimeConfig struct {
	StateDir string
}

type LocalConfig struct {
	ListenUrl          string
	StaticDir          string
	PublicUrl          string
	ApiBasePath        string
	CorsAllowedOrigins []string
	ExposeErrors       bool
	OAuth              LocalOAuthConfig
	Database           DatabaseConfig
}

type LocalOAuthConfig struct {
	ClientId     string
	ClientSecret string
	RedirectUrl  string
	Scopes       []string
}

type CloudConfig struct {
	ListenUrl          string
	StaticDir          string
	PublicUrl          string
	ApiBaseUrl         string
	CorsAllowedOrigins []string
	ExposeErrors       bool
	Jwt                JwtConfig
	Google             GoogleConfig
	GitHub             GitHubConfig
	Resend             ResendConfig
	Turnstile          TurnstileConfig
	OAuth              CloudOAuthConfig
	Database           DatabaseConfig
}

type TurnstileConfig struct {
	SiteKey   string
	SecretKey string
}

type CloudOAuthConfig struct {
	Clients []CloudOAuthClientConfig
}

type CloudOAuthClientConfig struct {
	ClientId     string   `mapstructure:"client_id"`
	ClientSecret string   `mapstructure:"client_secret"`
	RedirectUrl  string   `mapstructure:"redirect_url"`
	Scopes       []string `mapstructure:"scopes"`
}

type ValidationScope string

const (
	ValidationScopeAgent        ValidationScope = "agent"
	ValidationScopeCloud        ValidationScope = "cloud"
	ValidationScopeMigrateAgent ValidationScope = "migrate_agent"
	ValidationScopeMigrateCloud ValidationScope = "migrate_cloud"
)

type Options struct {
	Cwd              string
	Command          []string
	ValidationScope  ValidationScope
	LoadedConfigFile func(path string)
}

func Load(options Options) (Config, error) {
	if !validValidationScope(options.ValidationScope) {
		return Config{}, apperrors.Config("invalid configuration validation scope", fmt.Errorf("%q is not supported", options.ValidationScope))
	}
	cwd, err := resolveCwd(options.Cwd)
	if err != nil {
		return Config{}, err
	}

	environment := environmentName()
	defaultConfigFile := filepath.Join(cwd, ConfigDirName, DefaultFileName)
	envConfigFile, err := discoverEnvConfig(cwd, environment)
	if err != nil {
		return Config{}, err
	}
	envFile := envFilePath(cwd, environment)

	baseLoader, err := loadYAMLConfig(defaultConfigFile, envConfigFile, false)
	if err != nil {
		return Config{}, err
	}
	base, err := buildConfig(cwd, options, environment, defaultConfigFile, envConfigFile, "", baseLoader, false)
	if err != nil {
		return Config{}, err
	}

	notifyLoadedConfigFile(options.LoadedConfigFile, defaultConfigFile)
	notifyLoadedConfigFile(options.LoadedConfigFile, envConfigFile)
	if err := loadEnvFile(envFile, options.LoadedConfigFile); err != nil {
		return Config{}, err
	}

	v, err := loadYAMLConfig(defaultConfigFile, envConfigFile, true)
	if err != nil {
		return Config{}, err
	}
	cfg, err := buildConfig(cwd, options, environment, defaultConfigFile, envConfigFile, envFile, v, true)
	if err != nil {
		return Config{}, err
	}
	cfg.Base = &base

	return cfg, nil
}

func buildConfig(cwd string, options Options, environment string, defaultConfigFile string, envConfigFile string, loadedEnvFile string, v *viper.Viper, ensureDirs bool) (Config, error) {
	logDir, err := resolveLogDir(cwd, v.GetString("log.dir"))
	if err != nil {
		return Config{}, err
	}
	stateDir, err := resolveStateDir(cwd, v.GetString("runtime.state_dir"))
	if err != nil {
		return Config{}, err
	}
	agentStaticDir, err := resolveOptionalDir(cwd, v.GetString("local.static_dir"))
	if err != nil {
		return Config{}, apperrors.Config("invalid local.static_dir", err)
	}
	cloudStaticDir, err := resolveOptionalDir(cwd, v.GetString("cloud.static_dir"))
	if err != nil {
		return Config{}, apperrors.Config("invalid cloud.static_dir", err)
	}
	localDatabase := loadDatabaseConfig(cwd, v, "local.database")
	cloudDatabase := loadDatabaseConfig(cwd, v, "cloud.database")

	cfg := Config{
		Cwd:         cwd,
		Command:     append([]string(nil), options.Command...),
		Environment: environment,
		LogLevel:    strings.ToLower(v.GetString("log.level")),
		LogFormat:   strings.ToLower(v.GetString("log.format")),
		LogDir:      logDir,
		LogHTTP: LogHTTPConfig{
			Enabled:           v.GetBool("log.http.enabled"),
			RequestBodyLimit:  v.GetInt("log.http.request_body_limit"),
			ResponseBodyLimit: v.GetInt("log.http.response_body_limit"),
			SkipAssetEnabled:  v.GetBool("log.http.skip_asset_enabled"),
		},
		Terminal: TerminalConfig{
			History: HistoryConfig{
				MaxLines:     v.GetInt("terminal.history.max_lines"),
				MaxBytes:     v.GetInt64("terminal.history.max_bytes"),
				MaxLineBytes: v.GetInt("terminal.history.max_line_bytes"),
			},
			Replay: TerminalReplayConfig{
				MaxBytes:   v.GetInt64("terminal.replay.max_bytes"),
				ChunkBytes: v.GetInt("terminal.replay.chunk_bytes"),
			},
			Client: TerminalClientConfig{Queue: TerminalClientQueueConfig{
				MaxMessages: v.GetInt("terminal.client.queue.max_messages"),
				MaxBytes:    v.GetInt("terminal.client.queue.max_bytes"),
			}},
		},
		File: FileConfig{
			MaxTextBytes:              v.GetInt64("file.max_text_bytes"),
			MaxDirectoryEntries:       v.GetInt("file.max_directory_entries"),
			MaxRecursiveDeleteEntries: v.GetInt("file.max_recursive_delete_entries"),
			OperationTimeout:          v.GetDuration("file.operation_timeout"),
			WatchSubscriberQueueSize:  v.GetInt("file.watch_subscriber_queue_size"),
		},
		Git: GitConfig{
			Executable:     strings.TrimSpace(v.GetString("git.executable")),
			CommandTimeout: v.GetDuration("git.command_timeout"),
			MaxStdoutBytes: v.GetInt64("git.max_stdout_bytes"),
			MaxStderrBytes: v.GetInt64("git.max_stderr_bytes"),
			MaxTextBytes:   v.GetInt64("git.max_text_bytes"),
		},
		Runtime: RuntimeConfig{StateDir: stateDir},
		Local: LocalConfig{
			ListenUrl:          strings.TrimSpace(v.GetString("local.listen_url")),
			StaticDir:          agentStaticDir,
			PublicUrl:          strings.TrimSpace(v.GetString("local.public_url")),
			ApiBasePath:        strings.TrimSpace(v.GetString("local.api_base_path")),
			CorsAllowedOrigins: getStringSlice(v, "local.cors_allowed_origins"),
			ExposeErrors:       v.GetBool("local.expose_errors"),
			OAuth: LocalOAuthConfig{
				ClientId:     strings.TrimSpace(v.GetString("local.oauth.client_id")),
				ClientSecret: strings.TrimSpace(v.GetString("local.oauth.client_secret")),
				RedirectUrl:  strings.TrimSpace(v.GetString("local.oauth.redirect_url")),
				Scopes:       getStringSlice(v, "local.oauth.scopes"),
			},
			Database: localDatabase,
		},
		Cloud: CloudConfig{
			ListenUrl:          strings.TrimSpace(v.GetString("cloud.listen_url")),
			StaticDir:          cloudStaticDir,
			PublicUrl:          strings.TrimSpace(v.GetString("cloud.public_url")),
			ApiBaseUrl:         strings.TrimSpace(v.GetString("cloud.api_base_url")),
			CorsAllowedOrigins: getStringSlice(v, "cloud.cors_allowed_origins"),
			ExposeErrors:       v.GetBool("cloud.expose_errors"),
			Jwt: JwtConfig{
				SecretKey: v.GetString("cloud.jwt.secret_key"),
				Ttl:       v.GetDuration("cloud.jwt.ttl"),
			},
			Google: GoogleConfig{
				ClientId:     strings.TrimSpace(v.GetString("cloud.google.client_id")),
				ClientSecret: strings.TrimSpace(v.GetString("cloud.google.client_secret")),
				RedirectUrl:  strings.TrimSpace(v.GetString("cloud.google.redirect_url")),
			},
			GitHub: GitHubConfig{
				ClientId:     strings.TrimSpace(v.GetString("cloud.github.client_id")),
				ClientSecret: strings.TrimSpace(v.GetString("cloud.github.client_secret")),
				RedirectUrl:  strings.TrimSpace(v.GetString("cloud.github.redirect_url")),
			},
			Resend: ResendConfig{
				ApiKey:    strings.TrimSpace(v.GetString("cloud.resend.api_key")),
				FromEmail: strings.TrimSpace(v.GetString("cloud.resend.from_email")),
			},
			Turnstile: TurnstileConfig{
				SiteKey:   strings.TrimSpace(v.GetString("cloud.turnstile.site_key")),
				SecretKey: strings.TrimSpace(v.GetString("cloud.turnstile.secret_key")),
			},
			OAuth:    loadCloudOAuthConfig(v),
			Database: cloudDatabase,
		},
		DefaultConfigFile: defaultConfigFile,
		EnvConfigFile:     envConfigFile,
		EnvFile:           loadedEnvFile,
	}

	normalizeLogHTTPConfig(&cfg)
	normalizeTerminalConfig(&cfg)
	normalizeFileConfig(&cfg)
	normalizeGitConfig(&cfg)
	normalizeLocalConfig(&cfg)
	normalizeCloudConfig(&cfg)
	if !ensureDirs {
		return cfg, nil
	}

	if err := validateCommonConfig(cfg); err != nil {
		return Config{}, err
	}
	if err := validateScopeConfig(options.ValidationScope, cfg); err != nil {
		return Config{}, err
	}
	if err := ensureLogDir(cfg.LogDir); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func validValidationScope(scope ValidationScope) bool {
	switch scope {
	case ValidationScopeAgent, ValidationScopeCloud, ValidationScopeMigrateAgent, ValidationScopeMigrateCloud:
		return true
	default:
		return false
	}
}

func validateCommonConfig(cfg Config) error {
	if err := validateLogLevel(cfg.LogLevel); err != nil {
		return err
	}
	if err := validateLogFormat(cfg.LogFormat); err != nil {
		return err
	}
	return nil
}

func validateScopeConfig(scope ValidationScope, cfg Config) error {
	switch scope {
	case ValidationScopeAgent:
		return validateAgentConfig(cfg)
	case ValidationScopeCloud:
		return validateCloudRuntimeConfig(cfg)
	case ValidationScopeMigrateAgent:
		return validateDatabaseConfig("local.database", cfg.Local.Database)
	case ValidationScopeMigrateCloud:
		return validateDatabaseConfig("cloud.database", cfg.Cloud.Database)
	default:
		return apperrors.Config("invalid configuration validation scope", fmt.Errorf("%q is not supported", scope))
	}
}

func validateAgentConfig(cfg Config) error {
	if err := validateHistory(cfg.Terminal.History); err != nil {
		return err
	}
	if err := validateTerminal(cfg.Terminal); err != nil {
		return err
	}
	if err := validateFile(cfg.File); err != nil {
		return err
	}
	if err := validateGit(cfg.Git); err != nil {
		return err
	}
	if err := validateDatabaseConfig("local.database", cfg.Local.Database); err != nil {
		return err
	}
	if err := validateLocal(cfg.Local); err != nil {
		return apperrors.Config("invalid local configuration", err)
	}
	return validateAgentCloudTarget(cfg.Cloud)
}

func validateCloudRuntimeConfig(cfg Config) error {
	if err := validateDatabaseConfig("cloud.database", cfg.Cloud.Database); err != nil {
		return err
	}
	if err := validateCloud(cfg); err != nil {
		return err
	}
	if err := validateJwt(cfg.Cloud.Jwt); err != nil {
		return err
	}
	return validateIntegrationConfig(cfg)
}

func validateAgentCloudTarget(cfg CloudConfig) error {
	if cfg.PublicUrl != "" {
		if err := validateHTTPURL("cloud.public_url", cfg.PublicUrl); err != nil {
			return err
		}
	}
	return validateHTTPURL("cloud.api_base_url", cfg.ApiBaseUrl)
}

func IsGoogleAuthEnabled(cfg GoogleConfig) bool {
	return strings.TrimSpace(cfg.ClientId) != ""
}

func IsResendEnabled(cfg ResendConfig) bool {
	return strings.TrimSpace(cfg.ApiKey) != ""
}

func loadCloudOAuthConfig(v *viper.Viper) CloudOAuthConfig {
	var clients []CloudOAuthClientConfig
	if err := v.UnmarshalKey("cloud.oauth.clients", &clients); err != nil {
		return CloudOAuthConfig{}
	}
	return CloudOAuthConfig{Clients: clients}
}

func loadDatabaseConfig(cwd string, v *viper.Viper, prefix string) DatabaseConfig {
	path := strings.TrimSpace(v.GetString(prefix + ".sqlite.path"))
	if path != "" && !isConfigAbsPath(path) {
		path = filepath.Join(cwd, path)
	}
	return DatabaseConfig{
		Driver: strings.ToLower(strings.TrimSpace(v.GetString(prefix + ".driver"))),
		SQLite: SQLiteConfig{Path: path},
		MySQL:  MySQLConfig{Dsn: strings.TrimSpace(v.GetString(prefix + ".mysql.dsn"))},
	}
}

func validateDatabaseConfig(prefix string, cfg DatabaseConfig) error {
	switch cfg.Driver {
	case "sqlite":
		if cfg.SQLite.Path == "" {
			return apperrors.Config("invalid "+prefix+".sqlite.path", fmt.Errorf("empty path"))
		}
	case "mysql":
		if cfg.MySQL.Dsn == "" {
			return apperrors.Config("invalid "+prefix+".mysql.dsn", fmt.Errorf("empty Dsn"))
		}
	case "":
		return apperrors.Config("invalid "+prefix+".driver", fmt.Errorf("empty driver"))
	default:
		return apperrors.Config("invalid "+prefix+".driver", fmt.Errorf("must be sqlite or mysql"))
	}
	return nil
}

func validateJwt(cfg JwtConfig) error {
	if _, err := security.ParseBase64Key(cfg.SecretKey, 32); err != nil {
		return apperrors.Config("invalid cloud.jwt.secret_key", err)
	}
	if cfg.Ttl <= 0 {
		return apperrors.Config("invalid cloud.jwt.ttl", fmt.Errorf("must be positive"))
	}
	return nil
}

func validateOAuthProviderConfig(provider, clientId, clientSecret, redirectUrl string) error {
	if clientId == "" && clientSecret == "" && redirectUrl == "" {
		return nil
	}
	missing := []string{}
	if clientId == "" {
		missing = append(missing, envNameForKey("cloud."+provider+".client_id"))
	}
	if clientSecret == "" {
		missing = append(missing, envNameForKey("cloud."+provider+".client_secret"))
	}
	if redirectUrl == "" {
		missing = append(missing, envNameForKey("cloud."+provider+".redirect_url"))
	}
	if len(missing) > 0 {
		return apperrors.Config("incomplete "+provider+" auth configuration", errors.New(strings.Join(missing, ", ")))
	}
	return nil
}

func validateIntegrationConfig(cfg Config) error {
	if err := validateOAuthProviderConfig("google", cfg.Cloud.Google.ClientId, cfg.Cloud.Google.ClientSecret, cfg.Cloud.Google.RedirectUrl); err != nil {
		return err
	}
	if err := validateOAuthProviderConfig("github", cfg.Cloud.GitHub.ClientId, cfg.Cloud.GitHub.ClientSecret, cfg.Cloud.GitHub.RedirectUrl); err != nil {
		return err
	}
	if cfg.Cloud.Resend.ApiKey != "" || cfg.Cloud.Resend.FromEmail != "" {
		missing := []string{}
		if cfg.Cloud.Resend.ApiKey == "" {
			missing = append(missing, envNameForKey("cloud.resend.api_key"))
		}
		if cfg.Cloud.Resend.FromEmail == "" {
			missing = append(missing, envNameForKey("cloud.resend.from_email"))
		}
		if len(missing) > 0 {
			return apperrors.Config("incomplete resend configuration", errors.New(strings.Join(missing, ", ")))
		}
	}
	return nil
}

func resolveCwd(path string) (string, error) {
	if path == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", apperrors.Internal("get current directory", err)
		}
		return cwd, nil
	}

	cwd, err := filepath.Abs(path)
	if err != nil {
		return "", apperrors.Config(fmt.Sprintf("resolve cwd %q", path), err)
	}
	info, err := os.Stat(cwd)
	if err != nil {
		return "", apperrors.Config(fmt.Sprintf("invalid cwd %q", path), err)
	}
	if !info.IsDir() {
		return "", apperrors.Config(fmt.Sprintf("invalid cwd %q", path), fmt.Errorf("not a directory"))
	}
	return cwd, nil
}

func environmentName() string {
	return strings.TrimSpace(os.Getenv(EnvNameVariable))
}

func discoverEnvConfig(effectiveCwd string, environment string) (string, error) {
	if environment == "" {
		return "", nil
	}
	path := filepath.Join(effectiveCwd, ConfigDirName, envConfigFileName(environment))
	if exists, err := fileExists(path); err != nil {
		return "", apperrors.Config("check env config file", err)
	} else if exists {
		return path, nil
	}
	return "", nil
}

func envConfigFileName(environment string) string {
	return "config." + environment + ".yaml"
}

func envFilePath(cwd string, environment string) string {
	if environment == "" {
		return filepath.Join(cwd, EnvFileName)
	}
	return filepath.Join(cwd, EnvFileName+"."+environment)
}

func loadYAMLConfig(defaultConfigFile string, envConfigFile string, bindEnvironment bool) (*viper.Viper, error) {
	v := newLoader(bindEnvironment)

	v.SetConfigFile(defaultConfigFile)
	if err := v.ReadInConfig(); err != nil {
		return nil, apperrors.Config("read default config file", err)
	}

	if envConfigFile != "" {
		v.SetConfigFile(envConfigFile)
		if err := v.MergeInConfig(); err != nil {
			return nil, apperrors.Config("read env config file", err)
		}
	}
	return v, nil
}

func fileExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return false, fmt.Errorf("%s is a directory", path)
		}
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func loadEnvFile(path string, loadedConfigFile func(path string)) error {
	exists, err := fileExists(path)
	if err != nil {
		return apperrors.Config("check env file", err)
	}
	if !exists {
		return nil
	}
	if err := gotenv.Load(path); err != nil {
		return apperrors.Config("read env file", err)
	}
	notifyLoadedConfigFile(loadedConfigFile, path)
	return nil
}

func notifyLoadedConfigFile(loadedConfigFile func(path string), path string) {
	if loadedConfigFile == nil || strings.TrimSpace(path) == "" {
		return
	}
	loadedConfigFile(path)
}

func newLoader(bindEnvironment bool) *viper.Viper {
	loader := viper.New()
	loader.SetConfigType("yaml")
	if bindEnvironment {
		bindEnv(loader)
	}
	return loader
}

func bindEnv(loader *viper.Viper) {
	for _, key := range configKeys() {
		_ = loader.BindEnv(key, envNameForKey(key))
	}
}

func envNameForKey(key string) string {
	return EnvPrefix + "_" + strings.ToUpper(strings.ReplaceAll(key, ".", "__"))
}

func configKeys() []string {
	return []string{
		"log.level",
		"log.format",
		"log.dir",
		"log.http.enabled",
		"log.http.request_body_limit",
		"log.http.response_body_limit",
		"log.http.skip_asset_enabled",
		"terminal.history.max_lines",
		"terminal.history.max_bytes",
		"terminal.history.max_line_bytes",
		"terminal.replay.max_bytes",
		"terminal.replay.chunk_bytes",
		"terminal.client.queue.max_messages",
		"terminal.client.queue.max_bytes",
		"file.max_text_bytes",
		"file.max_directory_entries",
		"file.max_recursive_delete_entries",
		"file.operation_timeout",
		"file.watch_subscriber_queue_size",
		"git.executable",
		"git.command_timeout",
		"git.max_stdout_bytes",
		"git.max_stderr_bytes",
		"git.max_text_bytes",
		"runtime.state_dir",
		"local.listen_url",
		"local.static_dir",
		"local.public_url",
		"local.api_base_path",
		"local.cors_allowed_origins",
		"local.expose_errors",
		"local.oauth.client_id",
		"local.oauth.client_secret",
		"local.oauth.redirect_url",
		"local.oauth.scopes",
		"local.database.driver",
		"local.database.sqlite.path",
		"local.database.mysql.dsn",
		"cloud.listen_url",
		"cloud.static_dir",
		"cloud.public_url",
		"cloud.api_base_url",
		"cloud.cors_allowed_origins",
		"cloud.expose_errors",
		"cloud.jwt.secret_key",
		"cloud.jwt.ttl",
		"cloud.google.client_id",
		"cloud.google.client_secret",
		"cloud.google.redirect_url",
		"cloud.github.client_id",
		"cloud.github.client_secret",
		"cloud.github.redirect_url",
		"cloud.resend.api_key",
		"cloud.resend.from_email",
		"cloud.turnstile.site_key",
		"cloud.turnstile.secret_key",
		"cloud.database.driver",
		"cloud.database.sqlite.path",
		"cloud.database.mysql.dsn",
	}
}

func resolveLogDir(cwd string, path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", apperrors.Config("invalid log dir", fmt.Errorf("empty path"))
	}
	if isConfigAbsPath(path) {
		return filepath.Clean(path), nil
	}
	return filepath.Join(cwd, path), nil
}

func resolveStateDir(cwd string, path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", apperrors.Config("invalid runtime state dir", fmt.Errorf("empty path"))
	}
	if isConfigAbsPath(path) {
		return filepath.Clean(path), nil
	}
	return filepath.Join(cwd, path), nil
}

func resolveOptionalDir(cwd string, path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	if isConfigAbsPath(path) {
		return filepath.Clean(path), nil
	}
	return filepath.Join(cwd, path), nil
}

func isConfigAbsPath(path string) bool {
	return filepath.IsAbs(path) || strings.HasPrefix(path, "/")
}

func getStringSlice(v *viper.Viper, key string) []string {
	if value, ok := v.Get(key).(string); ok {
		return cleanStringSlice(strings.Split(value, ","))
	}
	return cleanStringSlice(v.GetStringSlice(key))
}

func cleanStringSlice(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func validateLogLevel(level string) error {
	switch level {
	case "debug", "info", "warn", "error":
		return nil
	default:
		return apperrors.Config("invalid log level", fmt.Errorf("%q", level))
	}
}

func validateLogFormat(format string) error {
	switch format {
	case "text", "json":
		return nil
	default:
		return apperrors.Config("invalid log format", fmt.Errorf("%q", format))
	}
}

func normalizeLogHTTPConfig(cfg *Config) {
	if cfg.LogHTTP.RequestBodyLimit <= 0 {
		cfg.LogHTTP.RequestBodyLimit = 0
	}
	if cfg.LogHTTP.ResponseBodyLimit <= 0 {
		cfg.LogHTTP.ResponseBodyLimit = 0
	}
}

func validateHistory(cfg HistoryConfig) error {
	if cfg.MaxLines < 1 {
		return apperrors.Config("invalid terminal.history.max_lines", fmt.Errorf("must be positive"))
	}
	if cfg.MaxBytes < 1 {
		return apperrors.Config("invalid terminal.history.max_bytes", fmt.Errorf("must be positive"))
	}
	if cfg.MaxLineBytes < 1 {
		return apperrors.Config("invalid terminal.history.max_line_bytes", fmt.Errorf("must be positive"))
	}
	return nil
}

func normalizeTerminalConfig(cfg *Config) {
	if cfg.Terminal.Replay.MaxBytes <= 0 {
		cfg.Terminal.Replay.MaxBytes = DefaultTerminalReplayMaxBytes
	}
	if cfg.Terminal.Replay.ChunkBytes <= 0 {
		cfg.Terminal.Replay.ChunkBytes = DefaultTerminalReplayChunkBytes
	}
	if cfg.Terminal.Client.Queue.MaxMessages <= 0 {
		cfg.Terminal.Client.Queue.MaxMessages = DefaultTerminalClientQueueMessages
	}
	if cfg.Terminal.Client.Queue.MaxBytes <= 0 {
		cfg.Terminal.Client.Queue.MaxBytes = DefaultTerminalClientQueueBytes
	}
}

func validateTerminal(cfg TerminalConfig) error {
	if cfg.Replay.MaxBytes < 1 {
		return apperrors.Config("invalid terminal.replay.max_bytes", fmt.Errorf("must be positive"))
	}
	if cfg.Replay.ChunkBytes < 1 {
		return apperrors.Config("invalid terminal.replay.chunk_bytes", fmt.Errorf("must be positive"))
	}
	if int64(cfg.Replay.ChunkBytes) > cfg.Replay.MaxBytes {
		return apperrors.Config("invalid terminal.replay.chunk_bytes", fmt.Errorf("must not exceed terminal.replay.max_bytes"))
	}
	if cfg.Client.Queue.MaxMessages < 1 {
		return apperrors.Config("invalid terminal.client.queue.max_messages", fmt.Errorf("must be positive"))
	}
	if cfg.Client.Queue.MaxBytes < 1 {
		return apperrors.Config("invalid terminal.client.queue.max_bytes", fmt.Errorf("must be positive"))
	}
	if cfg.Replay.MaxBytes >= int64(cfg.Client.Queue.MaxBytes) {
		return apperrors.Config("invalid terminal.replay.max_bytes", fmt.Errorf("must be less than terminal.client.queue.max_bytes"))
	}
	return nil
}

func normalizeFileConfig(cfg *Config) {
	if cfg.File.MaxTextBytes <= 0 {
		cfg.File.MaxTextBytes = 1024 * 1024
	}
	if cfg.File.MaxDirectoryEntries <= 0 {
		cfg.File.MaxDirectoryEntries = 1000
	}
	if cfg.File.MaxRecursiveDeleteEntries <= 0 {
		cfg.File.MaxRecursiveDeleteEntries = 10000
	}
	if cfg.File.OperationTimeout <= 0 {
		cfg.File.OperationTimeout = 10 * time.Second
	}
	if cfg.File.WatchSubscriberQueueSize <= 0 {
		cfg.File.WatchSubscriberQueueSize = 64
	}
}

func validateFile(cfg FileConfig) error {
	if cfg.MaxTextBytes < 1 {
		return apperrors.Config("invalid file.max_text_bytes", fmt.Errorf("must be positive"))
	}
	if cfg.MaxDirectoryEntries < 1 {
		return apperrors.Config("invalid file.max_directory_entries", fmt.Errorf("must be positive"))
	}
	if cfg.MaxRecursiveDeleteEntries < 1 {
		return apperrors.Config("invalid file.max_recursive_delete_entries", fmt.Errorf("must be positive"))
	}
	if cfg.OperationTimeout <= 0 {
		return apperrors.Config("invalid file.operation_timeout", fmt.Errorf("must be positive"))
	}
	if cfg.WatchSubscriberQueueSize < 1 {
		return apperrors.Config("invalid file.watch_subscriber_queue_size", fmt.Errorf("must be positive"))
	}
	return nil
}

func normalizeGitConfig(cfg *Config) {
	if cfg.Git.Executable == "" {
		cfg.Git.Executable = "git"
	}
	if cfg.Git.CommandTimeout <= 0 {
		cfg.Git.CommandTimeout = 10 * time.Second
	}
	if cfg.Git.MaxStdoutBytes <= 0 {
		cfg.Git.MaxStdoutBytes = 4 * 1024 * 1024
	}
	if cfg.Git.MaxStderrBytes <= 0 {
		cfg.Git.MaxStderrBytes = 64 * 1024
	}
	if cfg.Git.MaxTextBytes <= 0 {
		cfg.Git.MaxTextBytes = 1024 * 1024
	}
}

func validateGit(cfg GitConfig) error {
	if strings.TrimSpace(cfg.Executable) == "" {
		return apperrors.Config("invalid git.executable", fmt.Errorf("empty executable"))
	}
	if cfg.CommandTimeout <= 0 || cfg.CommandTimeout > MaxGitCommandTimeout {
		return apperrors.Config("invalid git.command_timeout", fmt.Errorf("must be between 1ns and %s", MaxGitCommandTimeout))
	}
	if cfg.MaxStdoutBytes < 1 {
		return apperrors.Config("invalid git.max_stdout_bytes", fmt.Errorf("must be positive"))
	}
	if cfg.MaxStderrBytes < 1 {
		return apperrors.Config("invalid git.max_stderr_bytes", fmt.Errorf("must be positive"))
	}
	if cfg.MaxTextBytes < 1 || cfg.MaxTextBytes > cfg.MaxStdoutBytes {
		return apperrors.Config("invalid git.max_text_bytes", fmt.Errorf("must be positive and not exceed git.max_stdout_bytes"))
	}
	return nil
}

func normalizeLocalConfig(cfg *Config) {
	cfg.Local.ListenUrl = strings.TrimRight(strings.TrimSpace(cfg.Local.ListenUrl), "/")
	cfg.Local.PublicUrl = strings.TrimRight(strings.TrimSpace(cfg.Local.PublicUrl), "/")
	cfg.Local.ApiBasePath = strings.TrimRight(strings.TrimSpace(cfg.Local.ApiBasePath), "/")
	cfg.Local.CorsAllowedOrigins = normalizeHttpOrigins(cfg.Local.CorsAllowedOrigins)
	cfg.Local.OAuth.ClientId = strings.TrimSpace(cfg.Local.OAuth.ClientId)
	cfg.Local.OAuth.ClientSecret = strings.TrimSpace(cfg.Local.OAuth.ClientSecret)
	cfg.Local.OAuth.RedirectUrl = strings.TrimRight(strings.TrimSpace(cfg.Local.OAuth.RedirectUrl), "/")
	cfg.Local.OAuth.Scopes = cleanStringSlice(cfg.Local.OAuth.Scopes)
	if cfg.Local.OAuth.ClientId != "" && len(cfg.Local.OAuth.Scopes) == 0 {
		cfg.Local.OAuth.Scopes = []string{"openid", "email", "profile"}
	}
}

func normalizeHttpOrigins(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimRight(strings.TrimSpace(value), "/")
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func normalizeCloudConfig(cfg *Config) {
	if cfg.Cloud.Jwt.Ttl <= 0 {
		cfg.Cloud.Jwt.Ttl = 24 * time.Hour
	}
	cfg.Cloud.ListenUrl = strings.TrimRight(strings.TrimSpace(cfg.Cloud.ListenUrl), "/")
	cfg.Cloud.PublicUrl = strings.TrimRight(strings.TrimSpace(cfg.Cloud.PublicUrl), "/")
	cfg.Cloud.ApiBaseUrl = strings.TrimRight(strings.TrimSpace(cfg.Cloud.ApiBaseUrl), "/")
	cfg.Cloud.CorsAllowedOrigins = normalizeHttpOrigins(cfg.Cloud.CorsAllowedOrigins)
	cfg.Cloud.Turnstile.SiteKey = strings.TrimSpace(cfg.Cloud.Turnstile.SiteKey)
	cfg.Cloud.Turnstile.SecretKey = strings.TrimSpace(cfg.Cloud.Turnstile.SecretKey)
	for index := range cfg.Cloud.OAuth.Clients {
		client := &cfg.Cloud.OAuth.Clients[index]
		client.ClientId = strings.TrimSpace(client.ClientId)
		client.ClientSecret = strings.TrimSpace(client.ClientSecret)
		client.RedirectUrl = strings.TrimRight(strings.TrimSpace(client.RedirectUrl), "/")
		client.Scopes = cleanStringSlice(client.Scopes)
		if len(client.Scopes) == 0 {
			client.Scopes = []string{"openid", "email", "profile"}
		}
	}
}

func validateLocal(cfg LocalConfig) error {
	if err := validateHTTPServerConfig("local", cfg.ListenUrl, cfg.PublicUrl, cfg.CorsAllowedOrigins); err != nil {
		return err
	}
	if err := validateAPIBasePath("local.api_base_path", cfg.ApiBasePath); err != nil {
		return err
	}
	return validateLocalOAuth(cfg.OAuth)
}

func validateAPIBasePath(key string, value string) error {
	if value == "" || value == "/" || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return fmt.Errorf("%s must be a non-root absolute path", key)
	}
	return nil
}

func validateCloud(cfg Config) error {
	if err := validateHTTPServerConfig("cloud", cfg.Cloud.ListenUrl, cfg.Cloud.PublicUrl, cfg.Cloud.CorsAllowedOrigins); err != nil {
		return err
	}
	if err := validateHTTPURL("cloud.api_base_url", cfg.Cloud.ApiBaseUrl); err != nil {
		return err
	}
	if err := validateTurnstile(cfg.Environment, cfg.Cloud); err != nil {
		return err
	}
	for index, client := range cfg.Cloud.OAuth.Clients {
		if err := validateCloudOAuthClient(index, client); err != nil {
			return err
		}
	}
	return nil
}

func validateTurnstile(environment string, cloud CloudConfig) error {
	if environment != "production" {
		return nil
	}
	if cloud.Turnstile.SiteKey == "" {
		return apperrors.Config("invalid cloud.turnstile.site_key", fmt.Errorf("required in production"))
	}
	if cloud.Turnstile.SecretKey == "" {
		return apperrors.Config("invalid cloud.turnstile.secret_key", fmt.Errorf("required in production"))
	}
	parsed, err := url.Parse(cloud.PublicUrl)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" {
		return apperrors.Config("invalid cloud.public_url", fmt.Errorf("must be an absolute HTTPS URL for Turnstile hostname validation in production"))
	}
	return nil
}

func validateLocalOAuth(cfg LocalOAuthConfig) error {
	if cfg.ClientId == "" && cfg.ClientSecret == "" && cfg.RedirectUrl == "" {
		return nil
	}
	missing := []string{}
	if cfg.ClientId == "" {
		missing = append(missing, envNameForKey("local.oauth.client_id"))
	}
	if cfg.ClientSecret == "" {
		missing = append(missing, envNameForKey("local.oauth.client_secret"))
	}
	if cfg.RedirectUrl == "" {
		missing = append(missing, envNameForKey("local.oauth.redirect_url"))
	}
	if len(missing) > 0 {
		return apperrors.Config("incomplete local OAuth configuration", errors.New(strings.Join(missing, ", ")))
	}
	return validateHTTPURL("local.oauth.redirect_url", cfg.RedirectUrl)
}

func validateCloudOAuthClient(index int, client CloudOAuthClientConfig) error {
	prefix := fmt.Sprintf("cloud.oauth.clients[%d]", index)
	if client.ClientId == "" {
		return apperrors.Config("invalid "+prefix+".client_id", fmt.Errorf("empty client id"))
	}
	if client.ClientSecret == "" {
		return apperrors.Config("invalid "+prefix+".client_secret", fmt.Errorf("empty client secret"))
	}
	if client.RedirectUrl == "" {
		return apperrors.Config("invalid "+prefix+".redirect_url", fmt.Errorf("empty redirect URL"))
	}
	return validateHTTPURL(prefix+".redirect_url", client.RedirectUrl)
}

func validateHTTPServerConfig(prefix string, listenURL string, publicURL string, corsAllowedOrigins []string) error {
	if err := validateHTTPURL(prefix+".listen_url", listenURL); err != nil {
		return err
	}
	if publicURL != "" {
		if err := validateHTTPURL(prefix+".public_url", publicURL); err != nil {
			return err
		}
	}
	for _, origin := range corsAllowedOrigins {
		if err := validateHTTPURL(prefix+".cors_allowed_origins", origin); err != nil {
			return err
		}
	}
	return nil
}

func validateHTTPURL(key string, value string) error {
	if strings.TrimSpace(value) == "" {
		return apperrors.Config("invalid "+key, fmt.Errorf("empty URL"))
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return apperrors.Config("invalid "+key, fmt.Errorf("must be an absolute URL"))
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return apperrors.Config("invalid "+key, fmt.Errorf("scheme must be http or https"))
	}
	return nil
}

func ensureLogDir(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return apperrors.Config("create log dir", err)
	}
	return nil
}
