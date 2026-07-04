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

	apperrors "termbridge-go/internal/shared/common/errors"
	"termbridge-go/internal/shared/common/security"
)

const (
	ConfigDirName   = "configs"
	DefaultFileName = "config.yaml"
	EnvFileName     = ".env"
	EnvPrefix       = "TERMBRIDGE"
	EnvNameVariable = EnvPrefix + "_ENV"

	DefaultAuthUsername = "admin"
	DefaultAuthPassword = "admin"
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
	Runtime           RuntimeConfig
	Agent             AgentConfig
	Cloud             CloudConfig
	Auth              AuthConfig
	JWT               JWTConfig
	Resend            ResendConfig
	DefaultConfigFile string
	EnvConfigFile     string
	EnvFile           string
	Base              *Config
}

type JWTConfig struct {
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
	DSN string
}

type AuthConfig struct {
	Username       string
	Password       string
	LocalAdmin     LocalAdminConfig
	JWTTTL         time.Duration
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
	TTL            time.Duration
	ResendCooldown time.Duration
	MaxAttempts    int
}

type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

type ResendConfig struct {
	APIKey    string
	FromEmail string
}

type LogHTTPConfig struct {
	RequestBodyLimit  int
	ResponseBodyLimit int
}

type HistoryConfig struct {
	MaxLines     int
	MaxBytes     int64
	MaxLineBytes int
}

type RuntimeConfig struct {
	StateDir string
}

type AgentConfig struct {
	ListenUrl          string
	StaticDir          string
	PublicUrl          string
	APIBaseURL         string
	CORSAllowedOrigins []string
	ExposeErrors       bool
	Database           DatabaseConfig
}

type CloudConfig struct {
	ListenUrl          string
	StaticDir          string
	PublicUrl          string
	APIBaseURL         string
	CORSAllowedOrigins []string
	ExposeErrors       bool
	GateUrl            string
	OAuth              CloudOAuthConfig
	Database           DatabaseConfig
}

type CloudOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
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
	agentStaticDir, err := resolveOptionalDir(cwd, v.GetString("agent.static_dir"))
	if err != nil {
		return Config{}, apperrors.Config("invalid agent.static_dir", err)
	}
	cloudStaticDir, err := resolveOptionalDir(cwd, v.GetString("cloud.static_dir"))
	if err != nil {
		return Config{}, apperrors.Config("invalid cloud.static_dir", err)
	}
	agentDatabase, err := loadDatabaseConfig(cwd, v, "agent.database")
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
			RequestBodyLimit:  v.GetInt("log.http.request_body_limit"),
			ResponseBodyLimit: v.GetInt("log.http.response_body_limit"),
		},
		History: HistoryConfig{
			MaxLines:     v.GetInt("history.max_lines"),
			MaxBytes:     v.GetInt64("history.max_bytes"),
			MaxLineBytes: v.GetInt("history.max_line_bytes"),
		},
		Runtime: RuntimeConfig{StateDir: stateDir},
		Agent: AgentConfig{
			ListenUrl:          strings.TrimSpace(v.GetString("agent.listen_url")),
			StaticDir:          agentStaticDir,
			PublicUrl:          strings.TrimSpace(v.GetString("agent.public_url")),
			APIBaseURL:         strings.TrimSpace(v.GetString("agent.api_base_url")),
			CORSAllowedOrigins: getStringSlice(v, "agent.cors_allowed_origins"),
			ExposeErrors:       v.GetBool("agent.expose_errors"),
			Database:           agentDatabase,
		},
		Cloud: CloudConfig{
			ListenUrl:          strings.TrimSpace(v.GetString("cloud.listen_url")),
			StaticDir:          cloudStaticDir,
			PublicUrl:          strings.TrimSpace(v.GetString("cloud.public_url")),
			APIBaseURL:         strings.TrimSpace(v.GetString("cloud.api_base_url")),
			CORSAllowedOrigins: getStringSlice(v, "cloud.cors_allowed_origins"),
			ExposeErrors:       v.GetBool("cloud.expose_errors"),
			GateUrl:            strings.TrimSpace(v.GetString("cloud.gate_url")),
			OAuth: CloudOAuthConfig{
				ClientID:     strings.TrimSpace(v.GetString("cloud.oauth.client_id")),
				ClientSecret: strings.TrimSpace(v.GetString("cloud.oauth.client_secret")),
				RedirectURL:  strings.TrimSpace(v.GetString("cloud.oauth.redirect_url")),
				Scopes:       getStringSlice(v, "cloud.oauth.scopes"),
			},
			Database: cloudDatabase,
		},
		Auth: AuthConfig{
			Username: strings.TrimSpace(v.GetString("auth.local_admin.username")),
			Password: strings.TrimSpace(v.GetString("auth.local_admin.password")),
			LocalAdmin: LocalAdminConfig{
				Username: strings.TrimSpace(v.GetString("auth.local_admin.username")),
				Password: strings.TrimSpace(v.GetString("auth.local_admin.password")),
			},
			JWTTTL: v.GetDuration("auth.jwt_ttl"),
			PasswordPolicy: PasswordPolicy{
				MinLength: v.GetInt("auth.password.min_length"),
				MaxLength: v.GetInt("auth.password.max_length"),
			},
			Code: CodePolicy{
				Length:         v.GetInt("auth.code.length"),
				TTL:            v.GetDuration("auth.code.ttl"),
				ResendCooldown: v.GetDuration("auth.code.resend_cooldown"),
				MaxAttempts:    v.GetInt("auth.code.max_attempts"),
			},
			Google: GoogleConfig{
				ClientID:     strings.TrimSpace(v.GetString("auth.google.client_id")),
				ClientSecret: strings.TrimSpace(v.GetString("auth.google.client_secret")),
				RedirectURL:  strings.TrimSpace(v.GetString("auth.google.redirect_url")),
			},
		},
		JWT: JWTConfig{
			SecretKey: v.GetString("jwt.secret_key"),
		},
		Resend: ResendConfig{
			APIKey:    strings.TrimSpace(v.GetString("resend.api_key")),
			FromEmail: strings.TrimSpace(v.GetString("resend.from_email")),
		},
		DefaultConfigFile: defaultConfigFile,
		EnvConfigFile:     envConfigFile,
		EnvFile:           loadedEnvFile,
	}

	normalizeLogBodyLimits(&cfg)
	normalizeAgentConfig(&cfg)
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
	if err := validateAgent(cfg.Agent); err != nil {
		return Config{}, err
	}
	if err := validateCloud(cfg); err != nil {
		return Config{}, err
	}
	if err := validateJWT(cfg.JWT); err != nil {
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
	return strings.TrimSpace(cfg.APIKey) != ""
}

func GenerateFernetKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(key), nil
}

func loadDatabaseConfig(cwd string, v *viper.Viper, prefix string) (DatabaseConfig, error) {
	driverKey := prefix + ".driver"
	pathKey := prefix + ".sqlite.path"
	dsnKey := prefix + ".mysql.dsn"
	driver := strings.ToLower(strings.TrimSpace(v.GetString(driverKey)))
	cfg := DatabaseConfig{
		Driver: driver,
		SQLite: SQLiteConfig{Path: strings.TrimSpace(v.GetString(pathKey))},
		MySQL:  MySQLConfig{DSN: strings.TrimSpace(v.GetString(dsnKey))},
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
		if cfg.MySQL.DSN == "" {
			return DatabaseConfig{}, apperrors.Config("invalid "+dsnKey, fmt.Errorf("empty DSN"))
		}
	default:
		return DatabaseConfig{}, apperrors.Config("invalid "+driverKey, fmt.Errorf("must be sqlite or mysql"))
	}
	return cfg, nil
}

func ensureJWTSecretKey(cfg Config) (Config, error) {
	if strings.TrimSpace(cfg.JWT.SecretKey) != "" {
		return cfg, nil
	}
	secretKey, err := GenerateFernetKey()
	if err != nil {
		return Config{}, apperrors.Config("generate jwt.secret_key", err)
	}
	cfg.JWT.SecretKey = secretKey
	cfg, err = upsertGeneratedConfigValues(cfg, map[string]string{"jwt.secret_key": secretKey})
	if err != nil {
		return Config{}, apperrors.Config("save jwt.secret_key", err)
	}
	return cfg, nil
}

func validateJWT(cfg JWTConfig) error {
	if _, err := security.ParseBase64Key(cfg.SecretKey, 32); err != nil {
		return apperrors.Config("invalid jwt.secret_key", err)
	}
	return nil
}

func validateAuth(cfg AuthConfig) error {
	if cfg.JWTTTL <= 0 {
		return apperrors.Config("invalid auth.jwt_ttl", fmt.Errorf("must be positive"))
	}
	if cfg.PasswordPolicy.MinLength < 1 || cfg.PasswordPolicy.MaxLength < cfg.PasswordPolicy.MinLength {
		return apperrors.Config("invalid auth.password", fmt.Errorf("invalid length range"))
	}
	if cfg.Code.Length != 6 {
		return apperrors.Config("invalid auth.code.length", fmt.Errorf("must be 6"))
	}
	if cfg.Code.TTL <= 0 || cfg.Code.ResendCooldown <= 0 || cfg.Code.MaxAttempts < 1 {
		return apperrors.Config("invalid auth.code", fmt.Errorf("ttl, resend cooldown and max attempts must be positive"))
	}
	return nil
}

func validateIntegrationConfig(cfg Config) error {
	if cfg.Auth.Google.ClientID != "" || cfg.Auth.Google.ClientSecret != "" || cfg.Auth.Google.RedirectURL != "" {
		missing := []string{}
		if cfg.Auth.Google.ClientID == "" {
			missing = append(missing, envNameForKey("auth.google.client_id"))
		}
		if cfg.Auth.Google.ClientSecret == "" {
			missing = append(missing, envNameForKey("auth.google.client_secret"))
		}
		if cfg.Auth.Google.RedirectURL == "" {
			missing = append(missing, envNameForKey("auth.google.redirect_url"))
		}
		if len(missing) > 0 {
			return apperrors.Config("incomplete google auth configuration", errors.New(strings.Join(missing, ", ")))
		}
	}
	if cfg.Resend.APIKey != "" || cfg.Resend.FromEmail != "" {
		missing := []string{}
		if cfg.Resend.APIKey == "" {
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
	if strings.HasPrefix(trimmed, "export ") {
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "export "))
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
		"history.max_lines",
		"history.max_bytes",
		"history.max_line_bytes",
		"runtime.state_dir",
		"agent.listen_url",
		"agent.static_dir",
		"agent.public_url",
		"agent.api_base_url",
		"agent.cors_allowed_origins",
		"agent.expose_errors",
		"agent.database.driver",
		"agent.database.sqlite.path",
		"agent.database.mysql.dsn",
		"cloud.listen_url",
		"cloud.static_dir",
		"cloud.public_url",
		"cloud.api_base_url",
		"cloud.cors_allowed_origins",
		"cloud.expose_errors",
		"cloud.gate_url",
		"cloud.oauth.client_id",
		"cloud.oauth.client_secret",
		"cloud.oauth.redirect_url",
		"cloud.oauth.scopes",
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

func normalizeLogBodyLimits(cfg *Config) {
	if cfg.LogHTTP.RequestBodyLimit <= 0 {
		cfg.LogHTTP.RequestBodyLimit = 0
	}
	if cfg.LogHTTP.ResponseBodyLimit <= 0 {
		cfg.LogHTTP.ResponseBodyLimit = 0
	}
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

func normalizeAgentConfig(cfg *Config) {
	cfg.Agent.ListenUrl = strings.TrimRight(strings.TrimSpace(cfg.Agent.ListenUrl), "/")
	cfg.Agent.PublicUrl = strings.TrimRight(strings.TrimSpace(cfg.Agent.PublicUrl), "/")
	cfg.Agent.APIBaseURL = strings.TrimRight(strings.TrimSpace(cfg.Agent.APIBaseURL), "/")
	cfg.Agent.CORSAllowedOrigins = normalizeHTTPOrigins(cfg.Agent.CORSAllowedOrigins)
}

func normalizeHTTPOrigins(values []string) []string {
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
	cfg.Cloud.APIBaseURL = strings.TrimRight(strings.TrimSpace(cfg.Cloud.APIBaseURL), "/")
	cfg.Cloud.CORSAllowedOrigins = normalizeHTTPOrigins(cfg.Cloud.CORSAllowedOrigins)
	cfg.Cloud.GateUrl = strings.TrimRight(strings.TrimSpace(cfg.Cloud.GateUrl), "/")
	cfg.Cloud.OAuth.ClientID = strings.TrimSpace(cfg.Cloud.OAuth.ClientID)
	cfg.Cloud.OAuth.ClientSecret = strings.TrimSpace(cfg.Cloud.OAuth.ClientSecret)
	cfg.Cloud.OAuth.RedirectURL = strings.TrimRight(strings.TrimSpace(cfg.Cloud.OAuth.RedirectURL), "/")
	if len(cfg.Cloud.OAuth.Scopes) == 0 {
		cfg.Cloud.OAuth.Scopes = []string{"openid", "email", "profile"}
	}
}

func validateAgent(cfg AgentConfig) error {
	return validateHTTPServerConfig("agent", cfg.ListenUrl, cfg.PublicUrl, cfg.APIBaseURL, cfg.CORSAllowedOrigins)
}

func validateCloud(cfg Config) error {
	if err := validateHTTPServerConfig("cloud", cfg.Cloud.ListenUrl, cfg.Cloud.PublicUrl, cfg.Cloud.APIBaseURL, cfg.Cloud.CORSAllowedOrigins); err != nil {
		return err
	}
	if cfg.Cloud.GateUrl != "" {
		if err := validateHTTPURL("cloud.gate_url", cfg.Cloud.GateUrl, false); err != nil {
			return err
		}
	}
	if cfg.Cloud.OAuth.RedirectURL != "" {
		if err := validateHTTPURL("cloud.oauth.redirect_url", cfg.Cloud.OAuth.RedirectURL, false); err != nil {
			return err
		}
	}
	return nil
}

func validateHTTPServerConfig(prefix string, listenURL string, publicURL string, apiBaseURL string, corsAllowedOrigins []string) error {
	if err := validateHTTPURL(prefix+".listen_url", listenURL, true); err != nil {
		return err
	}
	if publicURL != "" {
		if err := validateHTTPURL(prefix+".public_url", publicURL, false); err != nil {
			return err
		}
	}
	if apiBaseURL != "" {
		if err := validateAPIBaseURL(prefix+".api_base_url", apiBaseURL); err != nil {
			return err
		}
	}
	for _, origin := range corsAllowedOrigins {
		if err := validateHTTPURL(prefix+".cors_allowed_origins", origin, false); err != nil {
			return err
		}
	}
	return nil
}

func validateAPIBaseURL(key string, value string) error {
	if strings.HasPrefix(value, "/") {
		if strings.HasPrefix(value, "//") {
			return apperrors.Config("invalid "+key, fmt.Errorf("must not be a protocol-relative URL"))
		}
		return nil
	}
	return validateHTTPURL(key, value, false)
}

func validateHTTPURL(key string, value string, requirePort bool) error {
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
	if requirePort && parsed.Port() == "" {
		return apperrors.Config("invalid "+key, fmt.Errorf("port is required"))
	}
	return nil
}

func ensureLogDir(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return apperrors.Config("create log dir", err)
	}
	return nil
}
