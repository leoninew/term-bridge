package process

import "strings"

// SanitizeLaunchEnv removes parent-shell option variables that would otherwise
// change interactive shell semantics for a newly launched user process.
//
// Bash imports SHELLOPTS/BASHOPTS at startup. When the TermBridge agent is
// started from a parent shell that enabled set -e / set -u, those options are
// exported and would force interactive session shells to exit on the first
// failing command.
func SanitizeLaunchEnv(env []string) []string {
	if env == nil {
		return nil
	}
	out := make([]string, 0, len(env))
	for _, entry := range env {
		key, _, ok := strings.Cut(entry, "=")
		if ok && isBlockedShellOptionKey(key) {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func isBlockedShellOptionKey(key string) bool {
	switch strings.ToUpper(key) {
	case "SHELLOPTS", "BASHOPTS":
		return true
	default:
		return false
	}
}
