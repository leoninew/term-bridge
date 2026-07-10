//go:build !windows

package gopty

import "github.com/mattn/go-shellwords"

func parseCommandArguments(commandText string) ([]string, error) {
	parser := shellwords.NewParser()
	parser.ParseEnv = false
	parser.ParseBacktick = false
	return parser.Parse(commandText)
}
