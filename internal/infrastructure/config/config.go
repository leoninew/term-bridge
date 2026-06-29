package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
	"github.com/subosito/gotenv"

	apperrors "termbridge-go/internal/infrastructure/errors"
)

const (
	DefaultFileName = ".termbridge.default.yaml"
	FileName        = ".termbridge.yaml"
	EnvFileName     = ".env"
	EnvPrefix       = "TERMBRIDGE"

	WebModeLocal = "local"
	WebModeCloud = "cloud"

	DefaultAuthUsername = "admin"
	DefaultAuthPassword = "admin"
	DefaultListenURL    = "http://127.0.0.1:9030"
)

type Config struct {
	Cwd        string
	Command    []string
	LogLevel   string
	LogFormat  string
	LogDir     string
	LogHTTP    LogHTTPConfig
	History    HistoryConfig
	Runtime    RuntimeConfig
	Web        WebConfig
	Gate       GateConfig
	Agent      AgentConfig
	Cloud      CloudConfig
	Database   DatabaseConfig
	Auth       AuthConfig
	JWT        JWTConfig
	Resend     ResendConfig
	ConfigFile string
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

type WebConfig struct {
	StaticDir string
	Mode      string
}

type GateConfig struct {
	Browser GateBrowserConfig
	API     GateAPIConfig
}

type GateBrowserConfig struct {
	AllowedOrigins []string
}

type GateAPIConfig struct {
	ExposeErrors bool
}

type AgentConfig struct {
	ListenUrl  string
	PublicUrl  string
	ConnectUrl string
	DeviceId   string
	DeviceName string
}

type CloudConfig struct {
	GateUrl string
	OAuth   CloudOAuthConfig
}

type CloudOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}

type BootstrapResult struct {
	ConfigFile string
	Generated  bool
	Username   string
	Password   string
}

type Options struct {
	Cwd     string
	Command []string
}

func Load(options Options) (Config, error) {
	cwd, err := resolveCwd(options.Cwd)
	if err != nil {
		return Config{}, err
	}

	defaultConfigFile := filepath.Join(cwd, DefaultFileName)
	configFile, err := discoverConfig(cwd)
	if err != nil {
		return Config{}, err
	}
	if err := loadEnvFile(filepath.Join(cwd, EnvFileName)); err != nil {
		return Config{}, err
	}

	v := newLoader()

	v.SetConfigFile(defaultConfigFile)
	if err := v.ReadInConfig(); err != nil {
		return Config{}, apperrors.Config("read default config file", err)
	}

	if configFile != "" {
		v.SetConfigFile(configFile)
		if err := v.MergeInConfig(); err != nil {
			return Config{}, apperrors.Config("read config file", err)
		}
	}
	if err := rejectUnknownKeys(v); err != nil {
		return Config{}, err
	}

	logDir, err := resolveLogDir(cwd, v.GetString("log.dir"))
	if err != nil {
		return Config{}, err
	}
	stateDir, err := resolveStateDir(cwd, v.GetString("runtime.state_dir"))
	if err != nil {
		return Config{}, err
	}
	staticDir, err := resolveOptionalDir(cwd, v.GetString("web.static_dir"))
	if err != nil {
		return Config{}, apperrors.Config("invalid web.static_dir", err)
	}
	database, err := loadDatabaseConfig(cwd, v)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Cwd:       cwd,
		Command:   append([]string(nil), options.Command...),
		LogLevel:  strings.ToLower(v.GetString("log.level")),
		LogFormat: strings.ToLower(v.GetString("log.format")),
		LogDir:    logDir,
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
		Web: WebConfig{
			StaticDir: staticDir,
			Mode:      strings.ToLower(strings.TrimSpace(v.GetString("web.mode"))),
		},
		Database: database,
		Gate: GateConfig{
			Browser: GateBrowserConfig{
				AllowedOrigins: getStringSlice(v, "gate.browser.allowed_origins"),
			},
			API: GateAPIConfig{
				ExposeErrors: v.GetBool("gate.api.expose_errors"),
			},
		},
		Agent: AgentConfig{
			ListenUrl:  strings.TrimSpace(v.GetString("agent.listen_url")),
			PublicUrl:  strings.TrimSpace(v.GetString("agent.public_url")),
			ConnectUrl: strings.TrimSpace(v.GetString("agent.connect_url")),
			DeviceId:   strings.TrimSpace(v.GetString("agent.device_id")),
			DeviceName: strings.TrimSpace(v.GetString("agent.device_name")),
		},
		Cloud: CloudConfig{
			GateUrl: strings.TrimSpace(v.GetString("cloud.gate_url")),
			OAuth: CloudOAuthConfig{
				ClientID:     strings.TrimSpace(v.GetString("cloud.oauth.client_id")),
				ClientSecret: strings.TrimSpace(v.GetString("cloud.oauth.client_secret")),
				RedirectURL:  strings.TrimSpace(v.GetString("cloud.oauth.redirect_url")),
				Scopes:       getStringSlice(v, "cloud.oauth.scopes"),
			},
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
			SecretKey: strings.TrimSpace(v.GetString("jwt.secret_key")),
		},
		Resend: ResendConfig{
			APIKey:    strings.TrimSpace(v.GetString("resend.api_key")),
			FromEmail: strings.TrimSpace(v.GetString("resend.from_email")),
		},
		ConfigFile: configFile,
	}

	if err := validateLogLevel(cfg.LogLevel); err != nil {
		return Config{}, err
	}
	if err := validateLogFormat(cfg.LogFormat); err != nil {
		return Config{}, err
	}
	normalizeLogBodyLimits(&cfg)
	normalizeWebConfig(&cfg)
	if err := validateWeb(cfg.Web); err != nil {
		return Config{}, err
	}
	if err := validateHistory(cfg.History); err != nil {
		return Config{}, err
	}
	normalizeAgentConfig(&cfg)
	if err := validateGate(cfg.Gate); err != nil {
		return Config{}, err
	}
	if err := validateAgent(cfg.Agent); err != nil {
		return Config{}, err
	}
	normalizeCloudConfig(&cfg)
	if err := validateCloud(cfg); err != nil {
		return Config{}, err
	}
	if cfg.JWT.SecretKey == "" {
		return Config{}, apperrors.Config("invalid jwt.secret_key", fmt.Errorf("JWT secret key is required"))
	}
	if err := validateAuth(cfg.Auth); err != nil {
		return Config{}, err
	}
	if err := validateModeRequirements(cfg); err != nil {
		return Config{}, err
	}
	if err := ensureLogDir(cfg.LogDir); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func EnsureLocalIdentity(cfg Config) (Config, BootstrapResult, error) {
	updated, err := ensureAgentIdentityInConfig(cfg)
	if err != nil {
		return Config{}, BootstrapResult{}, err
	}
	return updated, BootstrapResult{ConfigFile: updated.ConfigFile}, nil
}

func IsSelfConnectedAgent(cfg Config) bool {
	return normalizeModeURL(cfg.Agent.ConnectUrl) == normalizeModeURL(cfg.Agent.ListenUrl)
}

func IsLocalMode(cfg Config) bool {
	return cfg.Web.Mode == WebModeLocal
}

func IsCloudMode(cfg Config) bool {
	return cfg.Web.Mode == WebModeCloud
}

func Mode(cfg Config) string {
	return cfg.Web.Mode
}

func loadDatabaseConfig(cwd string, v *viper.Viper) (DatabaseConfig, error) {
	driver := strings.ToLower(strings.TrimSpace(v.GetString("database.driver")))
	cfg := DatabaseConfig{
		Driver: driver,
		SQLite: SQLiteConfig{Path: strings.TrimSpace(v.GetString("database.sqlite.path"))},
		MySQL:  MySQLConfig{DSN: strings.TrimSpace(v.GetString("database.mysql.dsn"))},
	}
	if cfg.Driver == "" {
		return DatabaseConfig{}, apperrors.Config("invalid database.driver", fmt.Errorf("empty driver"))
	}
	switch cfg.Driver {
	case "sqlite":
		if cfg.SQLite.Path == "" {
			return DatabaseConfig{}, apperrors.Config("invalid database.sqlite.path", fmt.Errorf("empty path"))
		}
		if !isConfigAbsPath(cfg.SQLite.Path) {
			cfg.SQLite.Path = filepath.Join(cwd, cfg.SQLite.Path)
		}
	case "mysql":
		if cfg.MySQL.DSN == "" {
			return DatabaseConfig{}, apperrors.Config("invalid database.mysql.dsn", fmt.Errorf("empty DSN"))
		}
	default:
		return DatabaseConfig{}, apperrors.Config("invalid database.driver", fmt.Errorf("must be sqlite or mysql"))
	}
	return cfg, nil
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

func validateModeRequirements(cfg Config) error {
	if !IsCloudMode(cfg) {
		return nil
	}
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
	if cfg.Resend.APIKey == "" {
		missing = append(missing, envNameForKey("resend.api_key"))
	}
	if cfg.Resend.FromEmail == "" {
		missing = append(missing, envNameForKey("resend.from_email"))
	}
	if len(missing) > 0 {
		return apperrors.Config("remote gate missing required auth configuration", errors.New(strings.Join(missing, ", ")))
	}
	return nil
}

func normalizeModeURL(value string) string {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(value), "/"))
	if err != nil {
		return strings.TrimRight(strings.TrimSpace(value), "/")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed.String()
}

func ensureAgentIdentityInConfig(cfg Config) (Config, error) {
	changed := false
	deviceId := strings.TrimSpace(cfg.Agent.DeviceId)
	if deviceId == "" {
		generated, err := randomHex(16)
		if err != nil {
			return Config{}, apperrors.Config("generate device id", err)
		}
		deviceId = generated
		changed = true
	}
	deviceName := strings.TrimSpace(cfg.Agent.DeviceName)
	if deviceName == "" {
		deviceName = defaultDeviceName()
		changed = true
	}
	cfg.Agent.DeviceId = deviceId
	cfg.Agent.DeviceName = deviceName
	if !changed {
		return cfg, nil
	}
	configFile := cfg.ConfigFile
	if configFile == "" {
		configFile = filepath.Join(cfg.Cwd, FileName)
	}
	if err := saveAgentIdentityToConfigFile(configFile, cfg.Agent); err != nil {
		return Config{}, apperrors.Config("save agent identity", err)
	}
	cfg.ConfigFile = configFile
	return cfg, nil
}

func saveAgentIdentityToConfigFile(path string, agent AgentConfig) error {
	local := viper.New()
	local.SetConfigType("yaml")
	local.SetConfigFile(path)
	if exists, err := fileExists(path); err != nil {
		return err
	} else if exists {
		if err := local.ReadInConfig(); err != nil {
			return err
		}
	}
	local.Set("agent.device_id", strings.TrimSpace(agent.DeviceId))
	local.Set("agent.device_name", strings.TrimSpace(agent.DeviceName))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return local.WriteConfigAs(path)
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

func discoverConfig(effectiveCwd string) (string, error) {
	local := filepath.Join(effectiveCwd, FileName)
	if exists, err := fileExists(local); err != nil {
		return "", apperrors.Config("check local config file", err)
	} else if exists {
		return local, nil
	}
	return "", nil
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

func loadEnvFile(path string) error {
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
	return nil
}

func newLoader() *viper.Viper {
	loader := viper.New()
	loader.SetConfigType("yaml")
	bindEnv(loader)
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
		"web.static_dir",
		"web.mode",
		"gate.browser.allowed_origins",
		"gate.api.expose_errors",
		"agent.listen_url",
		"agent.public_url",
		"agent.connect_url",
		"agent.device_id",
		"agent.device_name",
		"cloud.gate_url",
		"cloud.oauth.client_id",
		"cloud.oauth.client_secret",
		"cloud.oauth.redirect_url",
		"cloud.oauth.scopes",
		"database.driver",
		"database.sqlite.path",
		"database.mysql.dsn",
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

func rejectUnknownKeys(v *viper.Viper) error {
	allowed := map[string]struct{}{}
	for _, key := range configKeys() {
		allowed[key] = struct{}{}
	}
	for _, key := range v.AllKeys() {
		if _, ok := allowed[key]; !ok {
			return apperrors.Config("unknown config key", fmt.Errorf("%q", key))
		}
	}
	return nil
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

func validateGate(cfg GateConfig) error {
	return nil
}

func normalizeWebConfig(cfg *Config) {
	cfg.Web.Mode = strings.ToLower(strings.TrimSpace(cfg.Web.Mode))
	if cfg.Web.Mode == "" {
		cfg.Web.Mode = WebModeLocal
	}
}

func validateWeb(cfg WebConfig) error {
	switch cfg.Mode {
	case WebModeLocal, WebModeCloud:
		return nil
	default:
		return apperrors.Config("invalid web.mode", fmt.Errorf("must be local or cloud"))
	}
}

func normalizeAgentConfig(cfg *Config) {
	cfg.Agent.ListenUrl = strings.TrimRight(strings.TrimSpace(cfg.Agent.ListenUrl), "/")
	cfg.Agent.PublicUrl = strings.TrimRight(strings.TrimSpace(cfg.Agent.PublicUrl), "/")
	if strings.TrimSpace(cfg.Agent.ConnectUrl) == "" {
		cfg.Agent.ConnectUrl = cfg.Agent.ListenUrl
		return
	}
	cfg.Agent.ConnectUrl = strings.TrimRight(strings.TrimSpace(cfg.Agent.ConnectUrl), "/")
}

func normalizeCloudConfig(cfg *Config) {
	cfg.Cloud.GateUrl = strings.TrimRight(strings.TrimSpace(cfg.Cloud.GateUrl), "/")
	cfg.Cloud.OAuth.ClientID = strings.TrimSpace(cfg.Cloud.OAuth.ClientID)
	cfg.Cloud.OAuth.ClientSecret = strings.TrimSpace(cfg.Cloud.OAuth.ClientSecret)
	cfg.Cloud.OAuth.RedirectURL = strings.TrimRight(strings.TrimSpace(cfg.Cloud.OAuth.RedirectURL), "/")
	if len(cfg.Cloud.OAuth.Scopes) == 0 {
		cfg.Cloud.OAuth.Scopes = []string{"openid", "email", "profile"}
	}
}

func validateAgent(cfg AgentConfig) error {
	if err := validateHTTPURL("agent.listen_url", cfg.ListenUrl, true); err != nil {
		return err
	}
	if cfg.PublicUrl != "" {
		if err := validateHTTPURL("agent.public_url", cfg.PublicUrl, false); err != nil {
			return err
		}
	}
	return validateHTTPURL("agent.connect_url", cfg.ConnectUrl, false)
}

func validateCloud(cfg Config) error {
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

func randomHex(byteCount int) (string, error) {
	data := make([]byte, byteCount)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

func currentUsername() string {
	for _, key := range []string{"USERNAME", "USER"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	current, err := user.Current()
	if err == nil && strings.TrimSpace(current.Username) != "" {
		name := current.Username
		if index := strings.LastIndexAny(name, `\\/`); index >= 0 {
			name = name[index+1:]
		}
		if strings.TrimSpace(name) != "" {
			return name
		}
	}
	return "termbridge"
}

func defaultDeviceName() string {
	hostname, err := os.Hostname()
	if err != nil || strings.TrimSpace(hostname) == "" {
		hostname = "termbridge-device"
	}
	hostname = sanitizeName(hostname)
	if hostname == "" {
		return "termbridge-device"
	}
	return hostname
}

func sanitizeName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.ReplaceAll(value, "_", "-")
	return strings.Trim(value, "-")
}
