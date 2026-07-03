package process

import "testing"

func TestResolveExecutableRejectsMissingCommand(t *testing.T) {
	_, err := ResolveExecutable("definitely-not-a-termbridge-command")
	if err == nil {
		t.Fatal("ResolveExecutable() error = nil, want error")
	}
}
