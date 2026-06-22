package session

import (
	"fmt"
	"time"
)

const SchemaVersion = 1

type State string

const (
	StateStarting State = "starting"
	StateRunning  State = "running"
	StateStopping State = "stopping"
	StateStopped  State = "stopped"
	StateFailed   State = "failed"
)

type StateRecord struct {
	SchemaVersion int       `json:"schema_version"`
	State         State     `json:"state"`
	Reason        string    `json:"reason"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (s State) Valid() bool {
	switch s {
	case StateStarting, StateRunning, StateStopping, StateStopped, StateFailed:
		return true
	default:
		return false
	}
}

func CanTransition(from State, to State) bool {
	if !from.Valid() || !to.Valid() {
		return false
	}
	switch from {
	case StateStarting:
		return to == StateRunning || to == StateFailed
	case StateRunning:
		return to == StateStopping || to == StateStopped || to == StateFailed
	case StateStopping:
		return to == StateStopped || to == StateFailed
	case StateFailed:
		return to == StateStopped
	default:
		return false
	}
}

func ValidateTransition(from State, to State) error {
	if CanTransition(from, to) {
		return nil
	}
	return fmt.Errorf("invalid session state transition %q -> %q", from, to)
}

func Terminal(s State) bool {
	return s == StateStopped || s == StateFailed
}
