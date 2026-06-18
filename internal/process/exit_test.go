package process

import (
	"errors"
	"testing"
)

func TestInterpretExitPassesNormalExitCode(t *testing.T) {
	got := InterpretExit(nil, 7, StopNone)
	if got.Code != 7 || got.Stopped {
		t.Fatalf("InterpretExit() = %#v", got)
	}
}

func TestInterpretExitUsesForcedCodeAfterClose(t *testing.T) {
	got := InterpretExit(errors.New("exit status 0xc000013a"), -1073741510, StopClose)
	if got.Code != ForcedExitCode || !got.Forced || !got.Closed {
		t.Fatalf("InterpretExit() = %#v", got)
	}
}

func TestInterpretExitClassifiesClosedWaitError(t *testing.T) {
	got := InterpretExit(errors.New("exit status 0xc000013a"), 0, StopNone)
	if got.Code != ForcedExitCode || !got.Closed {
		t.Fatalf("InterpretExit() = %#v", got)
	}
}
