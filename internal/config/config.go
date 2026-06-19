package config

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"

	apperrors "termbridge-go/internal/errors"
)

const (
	EnvPrefix = "TERMBRIDGE"
	FileName  = ".termbridge.yaml"
)

//go:embed termbridge.default.yaml
var defaultConfig []byte

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
	CwdAllowlist   []string
	EnvDenylist    []string
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

	configFile, err := discoverConfig(cwd)
	if err != nil {
		return Config{}, err
	}

	v := viper.New()
	v.SetConfigType("yaml")
	v.SetEnvPrefix(EnvPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	v.AutomaticEnv()

	if err := v.ReadConfig(bytes.NewReader(defaultConfig)); err != nil {
		return Config{}, apperrors.Internal("read default config", err)
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

	cwdAllowlist, err := resolvePathList(cwd, v.GetStringSlice("web.cwd_allowlist"))
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
			CwdAllowlist:   cwdAllowlist,
			EnvDenylist:    cleanStringSlice(v.GetStringSlice("web.env.denylist")),
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
	if err := ensureLogDir(cfg.LogDir); err != nil {
		return Config{}, err
	}

	return cfg, nil
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

	home, err := os.UserHomeDir()
	if err != nil {
		return "", apperrors.Config("resolve home directory", err)
	}
	homeConfig := filepath.Join(home, FileName)
	if exists, err := fileExists(homeConfig); err != nil {
		return "", apperrors.Config("check home config file", err)
	} else if exists {
		return homeConfig, nil
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
		"web.cwd_allowlist":       {},
		"web.env.denylist":        {},
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

func resolvePathList(cwd string, paths []string) ([]string, error) {
	cleaned := cleanStringSlice(paths)
	if len(cleaned) == 0 {
		cleaned = []string{cwd}
	}
	out := make([]string, 0, len(cleaned))
	for _, path := range cleaned {
		var err error
		path, err = expandHome(path)
		if err != nil {
			return nil, apperrors.Config("resolve web cwd allowlist", err)
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(cwd, path)
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return nil, apperrors.Config("resolve web cwd allowlist", err)
		}
		out = append(out, filepath.Clean(abs))
	}
	return out, nil
}

func expandHome(path string) (string, error) {
	if path == "~" {
		return os.UserHomeDir()
	}
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~\\") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, path[2:]), nil
	}
	if strings.HasPrefix(path, "~") {
		return "", fmt.Errorf("unsupported home path %q", path)
	}
	return path, nil
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
	if requestLimit <= 0 {
		return apperrors.Config("invalid log.request_body_limit", fmt.Errorf("must be positive"))
	}
	if responseLimit <= 0 {
		return apperrors.Config("invalid log.response_body_limit", fmt.Errorf("must be positive"))
	}
	return nil
}

func validateHistory(config HistoryConfig) error {
	if config.MaxLines <= 0 {
		return apperrors.Config("invalid history max_lines", fmt.Errorf("must be positive"))
	}
	if config.MaxBytes <= 0 {
		return apperrors.Config("invalid history max_bytes", fmt.Errorf("must be positive"))
	}
	if config.MaxLineBytes <= 0 {
		return apperrors.Config("invalid history max_line_bytes", fmt.Errorf("must be positive"))
	}
	return nil
}

func ensureLogDir(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return apperrors.Config("create log dir", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return apperrors.Config("stat log dir", err)
	}
	if !info.IsDir() {
		return apperrors.Config("invalid log dir", fmt.Errorf("not a directory: %s", path))
	}

	probe, err := os.CreateTemp(path, ".termbridge-log-probe-*")
	if err != nil {
		return apperrors.Config("check log dir writable", err)
	}
	name := probe.Name()
	if err := probe.Close(); err != nil {
		return apperrors.Config("close log dir probe", err)
	}
	if err := os.Remove(name); err != nil {
		return apperrors.Config("remove log dir probe", err)
	}
	return nil
}
