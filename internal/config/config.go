package config

import (
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

type Config struct {
	Cwd        string
	Command    []string
	LogLevel   string
	LogFormat  string
	LogDir     string
	ConfigFile string
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

	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "text")
	v.SetDefault("log.dir", "logs")

	if configFile != "" {
		v.SetConfigFile(configFile)
		if err := v.ReadInConfig(); err != nil {
			return Config{}, apperrors.Config("read config file", err)
		}
		if err := rejectUnknownKeys(v); err != nil {
			return Config{}, err
		}
	}

	logDir, err := resolveLogDir(cwd, v.GetString("log.dir"))
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Cwd:        cwd,
		Command:    append([]string(nil), options.Command...),
		LogLevel:   strings.ToLower(v.GetString("log.level")),
		LogFormat:  strings.ToLower(v.GetString("log.format")),
		LogDir:     logDir,
		ConfigFile: configFile,
	}

	if err := validateLogLevel(cfg.LogLevel); err != nil {
		return Config{}, err
	}
	if err := validateLogFormat(cfg.LogFormat); err != nil {
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
		"log.level":  {},
		"log.format": {},
		"log.dir":    {},
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
