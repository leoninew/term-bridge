//go:build windows

package gopty

func hasUnquotedShellControl(commandText string) bool {
	inQuotes := false
	backslashes := 0
	for _, value := range commandText {
		if value == '\\' {
			backslashes++
			continue
		}
		if value == '"' {
			if backslashes%2 == 0 {
				inQuotes = !inQuotes
			}
			backslashes = 0
			continue
		}
		if !inQuotes {
			switch value {
			case ';', '&', '|', '<', '>':
				return true
			}
		}
		backslashes = 0
	}
	return false
}
