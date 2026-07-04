package session

import "testing"

func TestCanTransitionAllowsExpectedTransitions(t *testing.T) {
	allowed := [][2]State{
		{StateRunning, StateStopped},
		{StateRunning, StateFailed},
		{StateStopped, StateRunning},
		{StateStopped, StateFailed},
		{StateFailed, StateRunning},
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
		{StateRunning, StateRunning},
		{StateStopped, StateStopped},
		{StateFailed, StateFailed},
		{State("starting"), StateRunning},
		{StateRunning, State("stopping")},
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
	for _, state := range []State{StateRunning, StateStopped, StateFailed} {
		if !state.Valid() {
			t.Fatalf("%q.Valid() = false", state)
		}
	}
	for _, state := range []State{State("starting"), State("stopping"), State("wat")} {
		if state.Valid() {
			t.Fatalf("%q.Valid() = true, want false", state)
		}
	}
}
