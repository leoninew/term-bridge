//go:build !windows

package gopty

func hasUnquotedShellControl(commandText string) bool {
	var quote rune
	escaped := false
	for _, r := range commandText {
		if quote == '\'' {
			if r == quote {
				quote = 0
			}
			continue
		}
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if quote == '"' {
			if r == quote {
				quote = 0
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		switch r {
		case ';', '&', '|', '<', '>':
			return true
		}
	}
	return false
}
