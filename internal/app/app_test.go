package app

import (
	"context"
	"testing"

	apperrors "termbridge-go/internal/errors"
)

func TestRunReturnsRuntimePlaceholder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	cwd := t.TempDir()

	result, err := Run(context.Background(), Options{Cwd: cwd, Command: []string{"pwsh"}})
	if err == nil {
		t.Fatal("Run() error = nil, want placeholder runtime error")
	}
	if apperrors.KindOf(err) != apperrors.KindRuntime {
		t.Fatalf("Run() error kind = %s, want runtime", apperrors.KindOf(err))
	}
	if len(result.Command) != 1 || result.Command[0] != "pwsh" {
		t.Fatalf("Result.Command = %#v", result.Command)
	}
}
