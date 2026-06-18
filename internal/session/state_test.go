package session

import "testing"

func TestCanTransitionAllowsExpectedTransitions(t *testing.T) {
	allowed := [][2]State{
		{StateStarting, StateRunning},
		{StateStarting, StateFailed},
		{StateRunning, StateStopping},
		{StateRunning, StateStopped},
		{StateRunning, StateFailed},
		{StateStopping, StateStopped},
		{StateStopping, StateFailed},
		{StateFailed, StateStopped},
	}
	for _, pair := range allowed {
		if !CanTransition(pair[0], pair[1]) {
			t.Fatalf("CanTransition(%q, %q) = false, want true", pair[0], pair[1])
		}
	}
}

func TestCanTransitionRejectsInvalidTransitions(t *testing.T) {
	invalid := [][2]State{
		{StateStopped, StateRunning},
		{StateFailed, StateRunning},
		{StateStarting, StateStopped},
		{State("bogus"), StateRunning},
		{StateRunning, State("bogus")},
	}
	for _, pair := range invalid {
		if CanTransition(pair[0], pair[1]) {
			t.Fatalf("CanTransition(%q, %q) = true, want false", pair[0], pair[1])
		}
	}
}

func TestStateValid(t *testing.T) {
	if !StateRunning.Valid() {
		t.Fatal("StateRunning.Valid() = false")
	}
	if State("wat").Valid() {
		t.Fatal("unknown state is valid")
	}
}
