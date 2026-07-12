package config

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
	"github.com/subosito/gotenv"
	"go.yaml.in/yaml/v3"

	apperrors "gitee.com/leoninew/TermBridge-go/internal/shared/common/errors"
	"gitee.com/leoninew/TermBridge-go/internal/shared/common/security"
)

const (
	ConfigDirName   = "configs"
	DefaultFileName = "config.yaml"
	EnvFileName     = ".env"
	EnvPrefix       = "TERMBRIDGE"
	EnvNameVariable = EnvPrefix + "_ENV"

	DefaultAuthUsername = "admin"
	DefaultAuthPassword = "admin"

	DefaultTerminalReplayMaxBytes      int64 = 1024 * 1024
	DefaultTerminalReplayChunkBytes          = 64 * 1024
	DefaultTerminalClientQueueMessages       = 64
	DefaultTerminalClientQueueBytes          = 4 * 1024 * 1024
)

type Config struct {
	Cwd               string
	Command           []string
	Environment       string
	LogLevel          string
	LogFormat         string
	LogDir            string
	LogHTTP           LogHTTPConfig
	History           HistoryConfig
	Terminal          TerminalConfig
	Runtime           RuntimeConfig
	Local             LocalConfig
	Cloud             CloudConfig
	Auth              AuthConfig
	Jwt               JwtConfig
	Resend            ResendConfig
	DefaultConfigFile string
	EnvConfigFile     string
	EnvFile           string
	Base              *Config
}

type JwtConfig struct {
	SecretKey string `json:"secret_key"`
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

type AuthConfig struct {
	Username       string
	Password       string
	LocalAdmin     LocalAdminConfig
	JwtTTL         time.Duration
	PasswordPolicy PasswordPolicy
	Code           CodePolicy
	Google         GoogleConfig
}

type LocalAdminConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type PasswordPolicy struct {
	MinLength int
	MaxLength int
}

type CodePolicy struct {
	Length         int
	Ttl            time.Duration
	ResendCooldown time.Duration
	MaxAttempts    int
}

type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	RedirectUrl  string
}

type ResendConfig struct {
	ApiKey    string
	FromEmail string
}

type LogHTTPConfig struct {
	RequestBodyLimit    int
	ResponseBodyLimit   int
	SkipAssetEnabled    bool
	SkipAssetExtensions []string
}

type HistoryConfig struct {
	MaxLines     int
	MaxBytes     int64
	MaxLineBytes int
}

type TerminalConfig struct {
	Replay TerminalReplayConfig
	Client TerminalClientConfig
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

type RuntimeConfig struct {
	StateDir string
}

type LocalConfig struct {
	ListenUrl          string
	StaticDir          string
	PublicUrl          string
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

type Options struct {
	Cwd              string
	Command          []string
	LoadedConfigFile func(path string)
}

func Load(options Options) (Config, error) {
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
	agentDatabase, err := loadDatabaseConfig(cwd, v, "local.database")
	if err != nil {
		return Config{}, err
	}
	cloudDatabase, err := loadDatabaseConfig(cwd, v, "cloud.database")
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Cwd:         cwd,
		Command:     append([]string(nil), options.Command...),
		Environment: environment,
		LogLevel:    strings.ToLower(v.GetString("log.level")),
		LogFormat:   strings.ToLower(v.GetString("log.format")),
		LogDir:      logDir,
		LogHTTP: LogHTTPConfig{
			RequestBodyLimit:    v.GetInt("log.http.request_body_limit"),
			ResponseBodyLimit:   v.GetInt("log.http.response_body_limit"),
			SkipAssetEnabled:    v.GetBool("log.http.skip_asset_enabled"),
			SkipAssetExtensions: getStringSlice(v, "log.http.skip_asset_extensions"),
		},
		History: HistoryConfig{
			MaxLines:     v.GetInt("history.max_lines"),
			MaxBytes:     v.GetInt64("history.max_bytes"),
			MaxLineBytes: v.GetInt("history.max_line_bytes"),
		},
		Terminal: TerminalConfig{
			Replay: TerminalReplayConfig{
				MaxBytes:   v.GetInt64("terminal.replay.max_bytes"),
				ChunkBytes: v.GetInt("terminal.replay.chunk_bytes"),
			},
			Client: TerminalClientConfig{Queue: TerminalClientQueueConfig{
				MaxMessages: v.GetInt("terminal.client.queue.max_messages"),
				MaxBytes:    v.GetInt("terminal.client.queue.max_bytes"),
			}},
		},
		Runtime: RuntimeConfig{StateDir: stateDir},
		Local: LocalConfig{
			ListenUrl:          strings.TrimSpace(v.GetString("local.listen_url")),
			StaticDir:          agentStaticDir,
			PublicUrl:          strings.TrimSpace(v.GetString("local.public_url")),
			CorsAllowedOrigins: getStringSlice(v, "local.cors_allowed_origins"),
			ExposeErrors:       v.GetBool("local.expose_errors"),
			OAuth: LocalOAuthConfig{
				ClientId:     strings.TrimSpace(v.GetString("local.oauth.client_id")),
				ClientSecret: strings.TrimSpace(v.GetString("local.oauth.client_secret")),
				RedirectUrl:  strings.TrimSpace(v.GetString("local.oauth.redirect_url")),
				Scopes:       getStringSlice(v, "local.oauth.scopes"),
			},
			Database: agentDatabase,
		},
		Cloud: CloudConfig{
			ListenUrl:          strings.TrimSpace(v.GetString("cloud.listen_url")),
			StaticDir:          cloudStaticDir,
			PublicUrl:          strings.TrimSpace(v.GetString("cloud.public_url")),
			ApiBaseUrl:         strings.TrimSpace(v.GetString("cloud.api_base_url")),
			CorsAllowedOrigins: getStringSlice(v, "cloud.cors_allowed_origins"),
			ExposeErrors:       v.GetBool("cloud.expose_errors"),
			Turnstile: TurnstileConfig{
				SiteKey:   strings.TrimSpace(v.GetString("cloud.turnstile.site_key")),
				SecretKey: strings.TrimSpace(v.GetString("cloud.turnstile.secret_key")),
			},
			OAuth:    loadCloudOAuthConfig(v),
			Database: cloudDatabase,
		},
		Auth: AuthConfig{
			Username: strings.TrimSpace(v.GetString("auth.local_admin.username")),
			Password: strings.TrimSpace(v.GetString("auth.local_admin.password")),
			LocalAdmin: LocalAdminConfig{
				Username: strings.TrimSpace(v.GetString("auth.local_admin.username")),
				Password: strings.TrimSpace(v.GetString("auth.local_admin.password")),
			},
			JwtTTL: v.GetDuration("auth.jwt_ttl"),
			PasswordPolicy: PasswordPolicy{
				MinLength: v.GetInt("auth.password.min_length"),
				MaxLength: v.GetInt("auth.password.max_length"),
			},
			Code: CodePolicy{
				Length:         v.GetInt("auth.code.length"),
				Ttl:            v.GetDuration("auth.code.ttl"),
				ResendCooldown: v.GetDuration("auth.code.resend_cooldown"),
				MaxAttempts:    v.GetInt("auth.code.max_attempts"),
			},
			Google: GoogleConfig{
				ClientID:     strings.TrimSpace(v.GetString("auth.google.client_id")),
				ClientSecret: strings.TrimSpace(v.GetString("auth.google.client_secret")),
				RedirectUrl:  strings.TrimSpace(v.GetString("auth.google.redirect_url")),
			},
		},
		Jwt: JwtConfig{
			SecretKey: v.GetString("jwt.secret_key"),
		},
		Resend: ResendConfig{
			ApiKey:    strings.TrimSpace(v.GetString("resend.api_key")),
			FromEmail: strings.TrimSpace(v.GetString("resend.from_email")),
		},
		DefaultConfigFile: defaultConfigFile,
		EnvConfigFile:     envConfigFile,
		EnvFile:           loadedEnvFile,
	}

	normalizeLogHTTPConfig(&cfg)
	normalizeTerminalConfig(&cfg)
	normalizeLocalConfig(&cfg)
	normalizeCloudConfig(&cfg)
	if ensureDirs {
		var err error
		cfg, err = ensureJWTSecretKey(cfg)
		if err != nil {
			return Config{}, err
		}
	}
	if !ensureDirs {
		return cfg, nil
	}

	if err := validateLogLevel(cfg.LogLevel); err != nil {
		return Config{}, err
	}
	if err := validateLogFormat(cfg.LogFormat); err != nil {
		return Config{}, err
	}
	if err := validateHistory(cfg.History); err != nil {
		return Config{}, err
	}
	if err := validateTerminal(cfg.Terminal); err != nil {
		return Config{}, err
	}
	if err := validateLocal(cfg.Local); err != nil {
		return Config{}, err
	}
	if err := validateCloud(cfg); err != nil {
		return Config{}, err
	}
	if err := validateJwt(cfg.Jwt); err != nil {
		return Config{}, err
	}
	if err := validateAuth(cfg.Auth); err != nil {
		return Config{}, err
	}
	if err := validateIntegrationConfig(cfg); err != nil {
		return Config{}, err
	}
	if err := ensureLogDir(cfg.LogDir); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func IsGoogleAuthEnabled(cfg GoogleConfig) bool {
	return strings.TrimSpace(cfg.ClientID) != ""
}

func IsResendEnabled(cfg ResendConfig) bool {
	return strings.TrimSpace(cfg.ApiKey) != ""
}

func GenerateFernetKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(key), nil
}

func loadCloudOAuthConfig(v *viper.Viper) CloudOAuthConfig {
	var clients []CloudOAuthClientConfig
	if err := v.UnmarshalKey("cloud.oauth.clients", &clients); err != nil {
		return CloudOAuthConfig{}
	}
	return CloudOAuthConfig{Clients: clients}
}

func loadDatabaseConfig(cwd string, v *viper.Viper, prefix string) (DatabaseConfig, error) {
	driverKey := prefix + ".driver"
	pathKey := prefix + ".sqlite.path"
	dsnKey := prefix + ".mysql.dsn"
	driver := strings.ToLower(strings.TrimSpace(v.GetString(driverKey)))
	cfg := DatabaseConfig{
		Driver: driver,
		SQLite: SQLiteConfig{Path: strings.TrimSpace(v.GetString(pathKey))},
		MySQL:  MySQLConfig{Dsn: strings.TrimSpace(v.GetString(dsnKey))},
	}
	if cfg.Driver == "" {
		return DatabaseConfig{}, apperrors.Config("invalid "+driverKey, fmt.Errorf("empty driver"))
	}
	switch cfg.Driver {
	case "sqlite":
		if cfg.SQLite.Path == "" {
			return DatabaseConfig{}, apperrors.Config("invalid "+pathKey, fmt.Errorf("empty path"))
		}
		if !isConfigAbsPath(cfg.SQLite.Path) {
			cfg.SQLite.Path = filepath.Join(cwd, cfg.SQLite.Path)
		}
	case "mysql":
		if cfg.MySQL.Dsn == "" {
			return DatabaseConfig{}, apperrors.Config("invalid "+dsnKey, fmt.Errorf("empty Dsn"))
		}
	default:
		return DatabaseConfig{}, apperrors.Config("invalid "+driverKey, fmt.Errorf("must be sqlite or mysql"))
	}
	return cfg, nil
}

func ensureJWTSecretKey(cfg Config) (Config, error) {
	if strings.TrimSpace(cfg.Jwt.SecretKey) != "" {
		return cfg, nil
	}
	secretKey, err := GenerateFernetKey()
	if err != nil {
		return Config{}, apperrors.Config("generate jwt.secret_key", err)
	}
	cfg.Jwt.SecretKey = secretKey
	cfg, err = upsertGeneratedConfigValues(cfg, map[string]string{"jwt.secret_key": secretKey})
	if err != nil {
		return Config{}, apperrors.Config("save jwt.secret_key", err)
	}
	return cfg, nil
}

func validateJwt(cfg JwtConfig) error {
	if _, err := security.ParseBase64Key(cfg.SecretKey, 32); err != nil {
		return apperrors.Config("invalid jwt.secret_key", err)
	}
	return nil
}

func validateAuth(cfg AuthConfig) error {
	if cfg.JwtTTL <= 0 {
		return apperrors.Config("invalid auth.jwt_ttl", fmt.Errorf("must be positive"))
	}
	if cfg.PasswordPolicy.MinLength < 1 || cfg.PasswordPolicy.MaxLength < cfg.PasswordPolicy.MinLength {
		return apperrors.Config("invalid auth.password", fmt.Errorf("invalid length range"))
	}
	if cfg.Code.Length != 6 {
		return apperrors.Config("invalid auth.code.length", fmt.Errorf("must be 6"))
	}
	if cfg.Code.Ttl <= 0 || cfg.Code.ResendCooldown <= 0 || cfg.Code.MaxAttempts < 1 {
		return apperrors.Config("invalid auth.code", fmt.Errorf("ttl, resend cooldown and max attempts must be positive"))
	}
	return nil
}

func validateIntegrationConfig(cfg Config) error {
	if cfg.Auth.Google.ClientID != "" || cfg.Auth.Google.ClientSecret != "" || cfg.Auth.Google.RedirectUrl != "" {
		missing := []string{}
		if cfg.Auth.Google.ClientID == "" {
			missing = append(missing, envNameForKey("auth.google.client_id"))
		}
		if cfg.Auth.Google.ClientSecret == "" {
			missing = append(missing, envNameForKey("auth.google.client_secret"))
		}
		if cfg.Auth.Google.RedirectUrl == "" {
			missing = append(missing, envNameForKey("auth.google.redirect_url"))
		}
		if len(missing) > 0 {
			return apperrors.Config("incomplete google auth configuration", errors.New(strings.Join(missing, ", ")))
		}
	}
	if cfg.Resend.ApiKey != "" || cfg.Resend.FromEmail != "" {
		missing := []string{}
		if cfg.Resend.ApiKey == "" {
			missing = append(missing, envNameForKey("resend.api_key"))
		}
		if cfg.Resend.FromEmail == "" {
			missing = append(missing, envNameForKey("resend.from_email"))
		}
		if len(missing) > 0 {
			return apperrors.Config("incomplete resend configuration", errors.New(strings.Join(missing, ", ")))
		}
	}
	return nil
}

func upsertGeneratedConfigValues(cfg Config, updates map[string]string) (Config, error) {
	if cfg.Environment == "" {
		envUpdates := make(map[string]string, len(updates))
		for key, value := range updates {
			envUpdates[envNameForKey(key)] = value
		}
		return cfg, upsertEnvFileValues(cfg.EnvFile, envUpdates)
	}
	path := envConfigWritePath(cfg)
	if err := upsertYAMLConfigValues(path, updates); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func envConfigWritePath(cfg Config) string {
	if strings.TrimSpace(cfg.EnvConfigFile) != "" {
		return cfg.EnvConfigFile
	}
	return filepath.Join(cfg.Cwd, ConfigDirName, envConfigFileName(cfg.Environment))
}

func upsertYAMLConfigValues(path string, updates map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	root := map[string]any{}
	if exists, err := fileExists(path); err != nil {
		return err
	} else if exists {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.TrimSpace(string(data)) != "" {
			if err := yaml.Unmarshal(data, &root); err != nil {
				return err
			}
		}
	}
	for key, value := range updates {
		setNestedYAMLValue(root, strings.Split(key, "."), value)
	}
	data, err := yaml.Marshal(root)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func setNestedYAMLValue(root map[string]any, path []string, value string) {
	if len(path) == 0 {
		return
	}
	if len(path) == 1 {
		root[path[0]] = value
		return
	}
	child, ok := root[path[0]].(map[string]any)
	if !ok {
		child = map[string]any{}
		root[path[0]] = child
	}
	setNestedYAMLValue(child, path[1:], value)
}

func upsertEnvFileValues(path string, updates map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var lines []string
	if exists, err := fileExists(path); err != nil {
		return err
	} else if exists {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lines = strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
	}
	seen := map[string]struct{}{}
	for index, line := range lines {
		key, ok := envLineKey(line)
		if !ok {
			continue
		}
		value, exists := updates[key]
		if !exists {
			continue
		}
		lines[index] = key + "=" + quoteEnvValue(value)
		seen[key] = struct{}{}
	}
	for _, key := range envUpdateKeys(updates) {
		if _, ok := seen[key]; ok {
			continue
		}
		lines = append(lines, key+"="+quoteEnvValue(updates[key]))
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600)
}

func envUpdateKeys(updates map[string]string) []string {
	preferred := []string{envNameForKey("jwt.secret_key")}
	keys := make([]string, 0, len(updates))
	seen := map[string]struct{}{}
	for _, key := range preferred {
		if _, ok := updates[key]; ok {
			keys = append(keys, key)
			seen[key] = struct{}{}
		}
	}
	for key := range updates {
		if _, ok := seen[key]; !ok {
			keys = append(keys, key)
		}
	}
	return keys
}

func envLineKey(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", false
	}
	if value, ok := strings.CutPrefix(trimmed, "export "); ok {
		trimmed = strings.TrimSpace(value)
	}
	index := strings.Index(trimmed, "=")
	if index <= 0 {
		return "", false
	}
	key := strings.TrimSpace(trimmed[:index])
	if key == "" {
		return "", false
	}
	return key, true
}

func quoteEnvValue(value string) string {
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
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
		"log.http.request_body_limit",
		"log.http.response_body_limit",
		"log.http.skip_asset_enabled",
		"log.http.skip_asset_extensions",
		"history.max_lines",
		"history.max_bytes",
		"history.max_line_bytes",
		"terminal.replay.max_bytes",
		"terminal.replay.chunk_bytes",
		"terminal.client.queue.max_messages",
		"terminal.client.queue.max_bytes",
		"runtime.state_dir",
		"local.listen_url",
		"local.static_dir",
		"local.public_url",
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
		"cloud.turnstile.site_key",
		"cloud.turnstile.secret_key",
		"cloud.database.driver",
		"cloud.database.sqlite.path",
		"cloud.database.mysql.dsn",
		"auth.local_admin.username",
		"auth.local_admin.password",
		"auth.jwt_ttl",
		"auth.password.min_length",
		"auth.password.max_length",
		"auth.code.length",
		"auth.code.ttl",
		"auth.code.resend_cooldown",
		"auth.code.max_attempts",
		"auth.google.client_id",
		"auth.google.client_secret",
		"auth.google.redirect_url",
		"jwt.secret_key",
		"resend.api_key",
		"resend.from_email",
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
	cfg.LogHTTP.SkipAssetExtensions = normalizeLogHTTPAssetExtensions(cfg.LogHTTP.SkipAssetExtensions)
}

func normalizeLogHTTPAssetExtensions(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if !strings.HasPrefix(value, ".") {
			value = "." + value
		}
		if value == "." {
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

func validateHistory(cfg HistoryConfig) error {
	if cfg.MaxLines < 1 {
		return apperrors.Config("invalid history.max_lines", fmt.Errorf("must be positive"))
	}
	if cfg.MaxBytes < 1 {
		return apperrors.Config("invalid history.max_bytes", fmt.Errorf("must be positive"))
	}
	if cfg.MaxLineBytes < 1 {
		return apperrors.Config("invalid history.max_line_bytes", fmt.Errorf("must be positive"))
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

func normalizeLocalConfig(cfg *Config) {
	cfg.Local.ListenUrl = strings.TrimRight(strings.TrimSpace(cfg.Local.ListenUrl), "/")
	cfg.Local.PublicUrl = strings.TrimRight(strings.TrimSpace(cfg.Local.PublicUrl), "/")
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
	return validateLocalOAuth(cfg.OAuth)
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
