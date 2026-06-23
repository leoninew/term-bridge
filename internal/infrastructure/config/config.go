package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"go.yaml.in/yaml/v3"

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
	Username string
	Password string
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
			ServerUrl:  strings.TrimSpace(v.GetString("agent.server_url")),
			DeviceId:   strings.TrimSpace(v.GetString("agent.device_id")),
			DeviceName: strings.TrimSpace(v.GetString("agent.device_name")),
		},
		Auth: AuthConfig{
			Username: strings.TrimSpace(v.GetString("auth.username")),
			Password: strings.TrimSpace(v.GetString("auth.password")),
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
	configFile := filepath.Join(cfg.Cwd, FileName)
	document := map[string]any{}
	if data, err := os.ReadFile(configFile); err == nil {
		if len(strings.TrimSpace(string(data))) > 0 {
			if err := yaml.Unmarshal(data, &document); err != nil {
				return Config{}, BootstrapResult{}, apperrors.Config("read local config file", err)
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Config{}, BootstrapResult{}, apperrors.Config("read local config file", err)
	}

	changed := false
	generatedPassword := ""
	username := strings.TrimSpace(cfg.Auth.Username)
	if username == "" {
		username = currentUsername()
		setNestedString(document, "auth", "username", username)
		changed = true
	}
	password := strings.TrimSpace(cfg.Auth.Password)
	if password == "" {
		generated, err := randomHex(24)
		if err != nil {
			return Config{}, BootstrapResult{}, apperrors.Config("generate auth password", err)
		}
		password = generated
		generatedPassword = generated
		setNestedString(document, "auth", "password", password)
		changed = true
	}
	deviceId := strings.TrimSpace(cfg.Agent.DeviceId)
	if deviceId == "" {
		generated, err := randomHex(16)
		if err != nil {
			return Config{}, BootstrapResult{}, apperrors.Config("generate device id", err)
		}
		deviceId = generated
		setNestedString(document, "agent", "device_id", deviceId)
		changed = true
	}
	deviceName := strings.TrimSpace(cfg.Agent.DeviceName)
	if deviceName == "" || legacyDefaultDeviceName(deviceName, deviceId) {
		deviceName = defaultDeviceName()
		setNestedString(document, "agent", "device_name", deviceName)
		changed = true
	}

	if changed {
		data, err := yaml.Marshal(document)
		if err != nil {
			return Config{}, BootstrapResult{}, apperrors.Config("marshal local config file", err)
		}
		if err := os.WriteFile(configFile, data, 0o600); err != nil {
			return Config{}, BootstrapResult{}, apperrors.Config("write local config file", err)
		}
	}

	loaded, err := Load(Options{Cwd: cfg.Cwd, Command: cfg.Command})
	if err != nil {
		return Config{}, BootstrapResult{}, err
	}
	return loaded, BootstrapResult{ConfigFile: configFile, Generated: generatedPassword != "", Username: username, Password: generatedPassword}, nil
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
		"agent.device_id":         {},
		"agent.device_name":       {},
		"auth.username":           {},
		"auth.password":           {},
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

func setNestedString(document map[string]any, section string, key string, value string) {
	nested, ok := document[section].(map[string]any)
	if !ok {
		if generic, ok := document[section].(map[any]any); ok {
			nested = make(map[string]any, len(generic))
			for k, v := range generic {
				if text, ok := k.(string); ok {
					nested[text] = v
				}
			}
		} else {
			nested = map[string]any{}
		}
		document[section] = nested
	}
	nested[key] = value
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

func legacyDefaultDeviceName(deviceName string, deviceId string) bool {
	deviceName = strings.TrimSpace(deviceName)
	deviceId = strings.TrimSpace(deviceId)
	if deviceName == "" || deviceId == "" {
		return false
	}
	short := deviceId
	if len(short) > 8 {
		short = short[:8]
	}
	if short == "" || !strings.HasSuffix(deviceName, "-"+short) {
		return false
	}
	prefix := strings.TrimSuffix(deviceName, "-"+short)
	return prefix == defaultDeviceName()
}

func sanitizeName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.ReplaceAll(value, "_", "-")
	return strings.Trim(value, "-")
}
