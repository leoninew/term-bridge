package git

import (
	"errors"
	"strings"
	"unicode/utf8"
)

const (
	MaxCommitMessageBytes = 16 * 1024
	MaxBranchNameBytes    = 240
	MaxHistoryEntries     = 50
)

func ValidateCommitMessage(value string) error {
	if strings.TrimSpace(value) == "" || !utf8.ValidString(value) || len(value) > MaxCommitMessageBytes || strings.ContainsRune(value, 0) {
		return errors.New("commit message is invalid")
	}
	return nil
}

func ValidateBranchName(value string) error {
	if value == "" || len(value) > MaxBranchNameBytes || !utf8.ValidString(value) || strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") || strings.HasPrefix(value, ".") || strings.HasSuffix(value, ".") || strings.Contains(value, "..") || strings.Contains(value, "@{") || strings.ContainsAny(value, " ~^:?*[]\\") {
		return errors.New("branch name is invalid")
	}
	for segment := range strings.SplitSeq(value, "/") {
		if segment == "" || strings.HasSuffix(segment, ".lock") {
			return errors.New("branch name is invalid")
		}
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return errors.New("branch name is invalid")
		}
	}
	return nil
}
