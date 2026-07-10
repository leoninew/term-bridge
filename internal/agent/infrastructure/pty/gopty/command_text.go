package gopty

import (
	"fmt"
	"strings"
)

// CommandTextFromArguments encodes already-tokenized CLI arguments as command text
// that parseCommandText restores without applying Shell semantics.
func CommandTextFromArguments(command []string) (string, error) {
	if len(command) == 0 {
		return "", fmt.Errorf("missing command")
	}
	values := make([]string, 0, len(command))
	for index, value := range command {
		if index == 0 && strings.TrimSpace(value) == "" {
			return "", fmt.Errorf("missing command")
		}
		values = append(values, quoteCommandArgument(value))
	}
	return strings.Join(values, " "), nil
}
