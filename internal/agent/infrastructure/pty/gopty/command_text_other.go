//go:build !windows

package gopty

import "strings"

func quoteCommandArgument(value string) string {
	if value == "" {
		return "''"
	}
	if !strings.ContainsAny(value, " \t\n\r\\\"';&|<>") {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
