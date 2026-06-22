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

// ProcessSpec is the stable boundary between CLI/app parsing and runtime launch.
type ProcessSpec struct {
	Command         string
	Args            []string
	Cwd             string
	Env             []string
	InitialSize     TerminalSize
	ResolvedCommand string
}

func NewSpec(cwd string, command []string, size TerminalSize) (ProcessSpec, error) {
	if len(command) == 0 {
		return ProcessSpec{}, fmt.Errorf("missing command")
	}
	return ProcessSpec{
		Command:     command[0],
		Args:        append([]string(nil), command[1:]...),
		Cwd:         cwd,
		Env:         append([]string(nil), os.Environ()...),
		InitialSize: size.OrDefault(),
	}, nil
}

func (s ProcessSpec) Validate() error {
	if strings.TrimSpace(s.Command) == "" {
		return fmt.Errorf("missing command")
	}
	if strings.TrimSpace(s.Cwd) == "" {
		return fmt.Errorf("missing cwd")
	}
	return nil
}

func (s ProcessSpec) WithResolvedCommand(path string) ProcessSpec {
	s.ResolvedCommand = path
	return s
}

func (s ProcessSpec) EffectiveCommand() string {
	if s.ResolvedCommand != "" {
		return s.ResolvedCommand
	}
	return s.Command
}
