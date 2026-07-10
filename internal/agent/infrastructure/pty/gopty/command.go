package gopty

import apperrors "gitee.com/leoninew/TermBridge-go/internal/shared/common/errors"

type parsedCommand struct {
	executable string
	args       []string
}

func parseCommandText(commandText string) (parsedCommand, error) {
	if hasUnquotedShellControl(commandText) {
		return parsedCommand{}, apperrors.Usage("shell control operators are not supported")
	}
	args, err := parseCommandArguments(commandText)
	if err != nil {
		return parsedCommand{}, apperrors.Usage("invalid command text")
	}
	if len(args) == 0 {
		return parsedCommand{}, apperrors.Usage("missing command")
	}
	return parsedCommand{executable: args[0], args: args[1:]}, nil
}
