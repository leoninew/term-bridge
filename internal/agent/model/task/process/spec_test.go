package process

import (
	"strings"
	"testing"
)

func TestNewSpecPreservesCommandText(t *testing.T) {
	commandText := `ccs list --filter "my project"`
	spec, err := NewSpec(`D:\project`, commandText, TerminalSize{Cols: 100, Rows: 40})
	if err != nil {
		t.Fatalf("NewSpec() error = %v", err)
	}
	if spec.CommandText != commandText {
		t.Fatalf("CommandText = %q, want %q", spec.CommandText, commandText)
	}
	if spec.InitialSize != (TerminalSize{Cols: 100, Rows: 40}) {
		t.Fatalf("InitialSize = %#v", spec.InitialSize)
	}
}

func TestNewSpecRequiresCommand(t *testing.T) {
	_, err := NewSpec(`D:\project`, "   ", TerminalSize{})
	if err == nil {
		t.Fatal("NewSpec() error = nil, want error")
	}
}

func TestNewSpecSanitizesLaunchEnv(t *testing.T) {
	t.Setenv("SHELLOPTS", "braceexpand:errexit:hashall")
	t.Setenv("BASHOPTS", "checkwinsize:cmdhist")
	t.Setenv("TERMBRIDGE_ENV_SANITIZE_TEST", "1")

	spec, err := NewSpec(`D:\project`, "bash", TerminalSize{Cols: 80, Rows: 25})
	if err != nil {
		t.Fatalf("NewSpec() error = %v", err)
	}
	foundMarker := false
	for _, entry := range spec.Env {
		key, _, _ := strings.Cut(entry, "=")
		switch strings.ToUpper(key) {
		case "SHELLOPTS", "BASHOPTS":
			t.Fatalf("NewSpec() Env still contains %q", entry)
		}
		if entry == "TERMBRIDGE_ENV_SANITIZE_TEST=1" {
			foundMarker = true
		}
	}
	if !foundMarker {
		t.Fatal("NewSpec() Env missing TERMBRIDGE_ENV_SANITIZE_TEST=1")
	}
}

func TestValidateRequiresCwd(t *testing.T) {
	spec := ProcessSpec{CommandText: "ccs"}
	if err := spec.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
}

func TestTerminalSizeDefaults(t *testing.T) {
	if got := (TerminalSize{}).OrDefault(); got != DefaultTerminalSize() {
		t.Fatalf("OrDefault() = %#v", got)
	}
}
