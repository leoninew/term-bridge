package process

import "testing"

func TestNewSpecSplitsCommandAndArgs(t *testing.T) {
	spec, err := NewSpec(`D:\\project`, []string{"pwsh", "-NoLogo"}, TerminalSize{Cols: 100, Rows: 40})
	if err != nil {
		t.Fatalf("NewSpec() error = %v", err)
	}
	if spec.Command != "pwsh" {
		t.Fatalf("Command = %q", spec.Command)
	}
	if len(spec.Args) != 1 || spec.Args[0] != "-NoLogo" {
		t.Fatalf("Args = %#v", spec.Args)
	}
	if spec.InitialSize != (TerminalSize{Cols: 100, Rows: 40}) {
		t.Fatalf("InitialSize = %#v", spec.InitialSize)
	}
}

func TestNewSpecRequiresCommand(t *testing.T) {
	_, err := NewSpec(`D:\\project`, nil, TerminalSize{})
	if err == nil {
		t.Fatal("NewSpec() error = nil, want error")
	}
}

func TestTerminalSizeDefaults(t *testing.T) {
	if got := (TerminalSize{}).OrDefault(); got != DefaultTerminalSize() {
		t.Fatalf("OrDefault() = %#v", got)
	}
}

func TestValidateRequiresCwd(t *testing.T) {
	spec := ProcessSpec{Command: "pwsh"}
	if err := spec.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
}
