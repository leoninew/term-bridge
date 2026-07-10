//go:build windows

package gopty

import "strings"

func quoteCommandArgument(value string) string {
	if value != "" && !strings.ContainsAny(value, " \t\n\r\v\";&|<>") {
		return value
	}

	var builder strings.Builder
	builder.WriteByte('"')
	backslashes := 0
	for _, character := range value {
		switch character {
		case '\\':
			backslashes++
		case '"':
			builder.WriteString(strings.Repeat("\\", backslashes*2+1))
			builder.WriteRune(character)
			backslashes = 0
		default:
			builder.WriteString(strings.Repeat("\\", backslashes))
			builder.WriteRune(character)
			backslashes = 0
		}
	}
	builder.WriteString(strings.Repeat("\\", backslashes*2))
	builder.WriteByte('"')
	return builder.String()
}
