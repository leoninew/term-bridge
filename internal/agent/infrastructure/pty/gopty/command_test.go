package gopty

import (
	"testing"

	apperrors "gitee.com/leoninew/TermBridge-go/internal/shared/common/errors"
)

func TestParseCommandTextPreservesQuotedArgument(t *testing.T) {
	parsed, err := parseCommandText(`ccs list --filter "my project"`)
	if err != nil {
		t.Fatalf("parseCommandText() error = %v", err)
	}
	if parsed.executable != "ccs" {
		t.Fatalf("executable = %q, want ccs", parsed.executable)
	}
	wantArgs := []string{"list", "--filter", "my project"}
	if len(parsed.args) != len(wantArgs) {
		t.Fatalf("args = %#v, want %#v", parsed.args, wantArgs)
	}
	for i, want := range wantArgs {
		if parsed.args[i] != want {
			t.Fatalf("args[%d] = %q, want %q", i, parsed.args[i], want)
		}
	}
}

func TestParseCommandTextRejectsShellControlOperators(t *testing.T) {
	for _, commandText := range []string{
		"ccs list | findstr active",
		"ccs list && echo active",
		"ccs list; echo active",
		"ccs list > output.txt",
		"ccs list < input.txt",
	} {
		t.Run(commandText, func(t *testing.T) {
			_, err := parseCommandText(commandText)
			if !apperrors.IsUsage(err) {
				t.Fatalf("parseCommandText() error kind = %s, want usage; err=%v", apperrors.KindOf(err), err)
			}
		})
	}
}

func TestParseCommandTextRejectsControlOperatorAfterSingleQuotedBackslash(t *testing.T) {
	_, err := parseCommandText(`printf 'x\'; echo blocked`)
	if !apperrors.IsUsage(err) {
		t.Fatalf("parseCommandText() error kind = %s, want usage; err=%v", apperrors.KindOf(err), err)
	}
}

func TestParseCommandTextKeepsQuotedShellControlOperator(t *testing.T) {
	parsed, err := parseCommandText(`ccs --label "a|b"`)
	if err != nil {
		t.Fatalf("parseCommandText() error = %v", err)
	}
	if len(parsed.args) != 2 || parsed.args[1] != "a|b" {
		t.Fatalf("args = %#v, want quoted control operator", parsed.args)
	}
}

func TestParseCommandTextHandlesQuotedWindowsPath(t *testing.T) {
	parsed, err := parseCommandText(`"C:\Program Files\ccs.exe" --filter "a|b"`)
	if err != nil {
		t.Fatalf("parseCommandText() error = %v", err)
	}
	if parsed.executable != `C:\Program Files\ccs.exe` {
		t.Fatalf("executable = %q, want quoted Windows path", parsed.executable)
	}
	if len(parsed.args) != 2 || parsed.args[1] != "a|b" {
		t.Fatalf("args = %#v, want quoted control argument", parsed.args)
	}
}

func TestCommandTextFromArgumentsPreservesArgumentBoundaries(t *testing.T) {
	command := []string{`C:\Program Files\ccs.exe`, "list", "--filter", "my project", "a|b", ""}
	commandText, err := CommandTextFromArguments(command)
	if err != nil {
		t.Fatalf("CommandTextFromArguments() error = %v", err)
	}
	parsed, err := parseCommandText(commandText)
	if err != nil {
		t.Fatalf("parseCommandText() error = %v", err)
	}
	got := append([]string{parsed.executable}, parsed.args...)
	if len(got) != len(command) {
		t.Fatalf("parsed command = %#v, want %#v", got, command)
	}
	for index, want := range command {
		if got[index] != want {
			t.Fatalf("parsed command[%d] = %q, want %q; command text = %q", index, got[index], want, commandText)
		}
	}
}

func TestCommandTextFromArgumentsRequiresExecutable(t *testing.T) {
	_, err := CommandTextFromArguments([]string{"", "list"})
	if err == nil {
		t.Fatal("CommandTextFromArguments() error = nil, want error")
	}
}

func TestParseCommandTextDoesNotExpandEnvironment(t *testing.T) {
	parsed, err := parseCommandText("ccs --filter $TERMBRIDGE_GOPTY_TEST")
	if err != nil {
		t.Fatalf("parseCommandText() error = %v", err)
	}
	if len(parsed.args) != 2 || parsed.args[1] != "$TERMBRIDGE_GOPTY_TEST" {
		t.Fatalf("args = %#v, want unexpanded environment reference", parsed.args)
	}
}
