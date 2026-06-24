package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"

	apperrors "termbridge-go/internal/infrastructure/errors"
)

const (
	DefaultFileName = ".termbridge.default.yaml"
	FileName        = ".termbridge.yaml"
)

type Config struct {
	Cwd                  string
	Command              []string
	LogLevel             string
	LogFormat            string
	LogDir               string
	LogRequestBodyLimit  int
	LogResponseBodyLimit int
	History              HistoryConfig
	Runtime              RuntimeConfig
	Web                  WebConfig
	Serve                ServeConfig
	Agent                AgentConfig
	Auth                 AuthConfig
	ConfigFile           string
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
	AllowedOrigins []string
	EnvDenylist    []string
	Error          WebErrorConfig
}

type WebErrorConfig struct {
	Debug bool
}

type ServeConfig struct {
	Host string
	Port int
	Open bool
	Dev  bool
}

type AgentConfig struct {
	ServerUrl  string
	DeviceId   string
	DeviceName string
}

type AuthConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type BootstrapResult struct {
	ConfigFile string
	Generated  bool
	Username   string
	Password   string
}

// LocalIdentityFileName is the JSON file stored in state_dir for local identity.
const LocalIdentityFileName = "device.json"

type LocalIdentity struct {
	Auth  AuthConfig          `json:"auth"`
	Agent AgentIdentityConfig `json:"agent"`
}

type AgentIdentityConfig struct {
	DeviceId   string `json:"device_id"`
	DeviceName string `json:"device_name"`
}

// loadLocalIdentityFromFile reads local auth and agent identity from device.json.
// Returns a zero-value identity if the file does not exist.
func loadLocalIdentityFromFile(path string) (LocalIdentity, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return LocalIdentity{}, nil
		}
		return LocalIdentity{}, err
	}
	var identity LocalIdentity
	if err := json.Unmarshal(data, &identity); err != nil {
		return LocalIdentity{}, err
	}
	return identity, nil
}

// saveLocalIdentityToFile writes local auth and agent identity to state_dir/device.json.
func saveLocalIdentityToFile(stateDir string, identity LocalIdentity) error {
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(identity, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(filepath.Join(stateDir, LocalIdentityFileName), data, 0o600)
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

	v := viper.New()
	v.SetConfigType("yaml")

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

	cfg := Config{
		Cwd:                  cwd,
		Command:              append([]string(nil), options.Command...),
		LogLevel:             strings.ToLower(v.GetString("log.level")),
		LogFormat:            strings.ToLower(v.GetString("log.format")),
		LogDir:               logDir,
		LogRequestBodyLimit:  v.GetInt("log.request_body_limit"),
		LogResponseBodyLimit: v.GetInt("log.response_body_limit"),
		History: HistoryConfig{
			MaxLines:     v.GetInt("history.max_lines"),
			MaxBytes:     v.GetInt64("history.max_bytes"),
			MaxLineBytes: v.GetInt("history.max_line_bytes"),
		},
		Runtime: RuntimeConfig{StateDir: stateDir},
		Web: WebConfig{
			AllowedOrigins: cleanStringSlice(v.GetStringSlice("web.allowed_origins")),
			EnvDenylist:    cleanStringSlice(v.GetStringSlice("web.env.denylist")),
			Error:          WebErrorConfig{Debug: v.GetBool("web.error.debug")},
		},
		Serve: ServeConfig{
			Host: strings.TrimSpace(v.GetString("serve.host")),
			Port: v.GetInt("serve.port"),
			Open: v.GetBool("serve.open"),
			Dev:  v.GetBool("serve.dev"),
		},
		Agent: AgentConfig{
			ServerUrl: strings.TrimSpace(v.GetString("agent.server_url")),
		},
		ConfigFile: configFile,
	}

	if err := validateLogLevel(cfg.LogLevel); err != nil {
		return Config{}, err
	}
	if err := validateLogFormat(cfg.LogFormat); err != nil {
		return Config{}, err
	}
	if err := validateLogBodyLimits(cfg.LogRequestBodyLimit, cfg.LogResponseBodyLimit); err != nil {
		return Config{}, err
	}
	if err := validateHistory(cfg.History); err != nil {
		return Config{}, err
	}
	if err := validateServe(cfg.Serve); err != nil {
		return Config{}, err
	}
	if err := validateAgent(cfg.Agent); err != nil {
		return Config{}, err
	}
	if err := ensureLogDir(cfg.LogDir); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func EnsureLocalIdentity(cfg Config) (Config, BootstrapResult, error) {
	identityPath := filepath.Join(cfg.Runtime.StateDir, LocalIdentityFileName)
	identity, err := loadLocalIdentityFromFile(identityPath)
	if err != nil {
		return Config{}, BootstrapResult{}, apperrors.Config("load local identity", err)
	}

	changed := false
	generatedPassword := ""
	username := strings.TrimSpace(identity.Auth.Username)
	if username == "" {
		username = currentUsername()
		identity.Auth.Username = username
		changed = true
	}
	password := strings.TrimSpace(identity.Auth.Password)
	if password == "" {
		generated, err := randomHex(24)
		if err != nil {
			return Config{}, BootstrapResult{}, apperrors.Config("generate auth password", err)
		}
		password = generated
		generatedPassword = generated
		identity.Auth.Password = password
		changed = true
	}
	deviceId := strings.TrimSpace(identity.Agent.DeviceId)
	if deviceId == "" {
		generated, err := randomHex(16)
		if err != nil {
			return Config{}, BootstrapResult{}, apperrors.Config("generate device id", err)
		}
		deviceId = generated
		identity.Agent.DeviceId = deviceId
		changed = true
	}
	deviceName := strings.TrimSpace(identity.Agent.DeviceName)
	if deviceName == "" {
		deviceName = defaultDeviceName()
		identity.Agent.DeviceName = deviceName
		changed = true
	}
	if changed {
		if err := saveLocalIdentityToFile(cfg.Runtime.StateDir, identity); err != nil {
			return Config{}, BootstrapResult{}, apperrors.Config("save local identity", err)
		}
	}

	cfg.Auth = AuthConfig{Username: username, Password: password}
	cfg.Agent.DeviceId = deviceId
	cfg.Agent.DeviceName = deviceName
	return cfg, BootstrapResult{ConfigFile: identityPath, Generated: generatedPassword != "", Username: username, Password: generatedPassword}, nil
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

func rejectUnknownKeys(v *viper.Viper) error {
	allowed := map[string]struct{}{
		"log.level":               {},
		"log.format":              {},
		"log.dir":                 {},
		"log.request_body_limit":  {},
		"log.response_body_limit": {},
		"history.max_lines":       {},
		"history.max_bytes":       {},
		"history.max_line_bytes":  {},
		"runtime.state_dir":       {},
		"web.allowed_origins":     {},
		"web.env.denylist":        {},
		"web.error.debug":         {},
		"serve.host":              {},
		"serve.port":              {},
		"serve.open":              {},
		"serve.dev":               {},
		"agent.server_url":        {},
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
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	return filepath.Join(cwd, path), nil
}

func resolveStateDir(cwd string, path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", apperrors.Config("invalid runtime state dir", fmt.Errorf("empty path"))
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	return filepath.Join(cwd, path), nil
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

func validateLogBodyLimits(requestLimit int, responseLimit int) error {
	if requestLimit < 1 {
		return apperrors.Config("invalid log.request_body_limit", fmt.Errorf("must be positive"))
	}
	if responseLimit < 1 {
		return apperrors.Config("invalid log.response_body_limit", fmt.Errorf("must be positive"))
	}
	return nil
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

func validateServe(cfg ServeConfig) error {
	if cfg.Port < 0 || cfg.Port > 65535 {
		return apperrors.Config("invalid serve.port", fmt.Errorf("must be between 0 and 65535"))
	}
	return nil
}

func validateAgent(cfg AgentConfig) error {
	if strings.TrimSpace(cfg.ServerUrl) == "" {
		return apperrors.Config("invalid agent.server_url", fmt.Errorf("empty Url"))
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
