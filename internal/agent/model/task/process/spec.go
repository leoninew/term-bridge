package process

import (
	"fmt"
	"os"
	"strings"
)

// TerminalSize describes a PTY size in character cells.
type TerminalSize struct {
	Cols int
	Rows int
}

func DefaultTerminalSize() TerminalSize {
	return TerminalSize{Cols: 80, Rows: 25}
}

func (s TerminalSize) IsValid() bool {
	return s.Cols > 0 && s.Rows > 0
}

func (s TerminalSize) OrDefault() TerminalSize {
	if s.IsValid() {
		return s
	}
	return DefaultTerminalSize()
}

// ProcessSpec is the stable boundary between application code and PTY launch.
// CommandText is the exact command text supplied by the caller. The PTY
// infrastructure is the only layer permitted to parse it for process launch.
type ProcessSpec struct {
	CommandText string
	Cwd         string
	Env         []string
	InitialSize TerminalSize
}

func NewSpec(cwd string, commandText string, size TerminalSize) (ProcessSpec, error) {
	if strings.TrimSpace(commandText) == "" {
		return ProcessSpec{}, fmt.Errorf("missing command")
	}
	return ProcessSpec{
		CommandText: commandText,
		Cwd:         cwd,
		Env:         append([]string(nil), os.Environ()...),
		InitialSize: size.OrDefault(),
	}, nil
}

func (s ProcessSpec) Validate() error {
	if strings.TrimSpace(s.CommandText) == "" {
		return fmt.Errorf("missing command")
	}
	if strings.TrimSpace(s.Cwd) == "" {
		return fmt.Errorf("missing cwd")
	}
	return nil
}
