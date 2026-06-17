package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	apperrors "termbridge-go/internal/errors"
)

func TestLoadDefaults(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()

	cfg, err := Load(Options{Cwd: cwd, Command: []string{"pwsh"}})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
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
	if cfg.ConfigFile != "" {
		t.Fatalf("ConfigFile = %q, want empty", cfg.ConfigFile)
	}
	if !reflect.DeepEqual(cfg.Command, []string{"pwsh"}) {
		t.Fatalf("Command = %#v", cfg.Command)
	}
}

func TestLoadRejectsInvalidLogLevel(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeConfig(t, cwd, "log:\n  level: trace\n")

	_, err := Load(Options{Cwd: cwd})
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
	if !apperrors.IsConfig(err) {
		t.Fatalf("Load() error = %T, want config error", err)
	}
}

func TestLoadRejectsUnknownConfigKey(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeConfig(t, cwd, "log:\n  level: info\nunknown: true\n")

	_, err := Load(Options{Cwd: cwd})
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
	if !apperrors.IsConfig(err) {
		t.Fatalf("Load() error = %T, want config error", err)
	}
}

func TestLoadReadsLocalConfigFile(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	configPath := filepath.Join(cwd, FileName)
	logDir := filepath.Join(cwd, "configured-logs")
	content := "log:\n  level: debug\n  format: json\n  dir: " + filepath.ToSlash(logDir) + "\n"
	writeConfig(t, cwd, content)

	cfg, err := Load(Options{Cwd: cwd})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ConfigFile != configPath {
		t.Fatalf("ConfigFile = %q, want %q", cfg.ConfigFile, configPath)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("LogLevel = %q", cfg.LogLevel)
	}
	if cfg.LogFormat != "json" {
		t.Fatalf("LogFormat = %q", cfg.LogFormat)
	}
	if filepath.Clean(cfg.LogDir) != filepath.Clean(logDir) {
		t.Fatalf("LogDir = %q, want %q", cfg.LogDir, logDir)
	}
}

func TestLoadPrefersLocalConfigOverHome(t *testing.T) {
	home := isolateHome(t)
	if err := os.WriteFile(filepath.Join(home, FileName), []byte("log:\n  level: error\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(home) error = %v", err)
	}

	cwd := t.TempDir()
	localConfig := filepath.Join(cwd, FileName)
	writeConfig(t, cwd, "log:\n  level: debug\n")

	cfg, err := Load(Options{Cwd: cwd})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ConfigFile != localConfig {
		t.Fatalf("ConfigFile = %q, want %q", cfg.ConfigFile, localConfig)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("LogLevel = %q, want debug", cfg.LogLevel)
	}
}

func TestLoadReadsHomeConfigWhenLocalMissing(t *testing.T) {
	home := isolateHome(t)
	logDir := filepath.Join(home, "home-logs")
	homeConfig := filepath.Join(home, FileName)
	content := []byte("log:\n  level: warn\n  dir: " + filepath.ToSlash(logDir) + "\n")
	if err := os.WriteFile(homeConfig, content, 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	cwd := t.TempDir()
	cfg, err := Load(Options{Cwd: cwd})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ConfigFile != homeConfig {
		t.Fatalf("ConfigFile = %q, want %q", cfg.ConfigFile, homeConfig)
	}
	if cfg.LogLevel != "warn" {
		t.Fatalf("LogLevel = %q, want warn", cfg.LogLevel)
	}
}

func isolateHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}

func writeConfig(t *testing.T, dir string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(content), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
}
