package process

import (
	"fmt"
	"os/exec"
)

// ResolveExecutable resolves a command name to an executable path.
func ResolveExecutable(command string) (string, error) {
	path, err := exec.LookPath(command)
	if err != nil {
		return "", fmt.Errorf("resolve executable %q: %w", command, err)
	}
	return path, nil
}
