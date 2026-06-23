package session

import (
	"fmt"
	"time"
)

const SchemaVersion = 1

type State string

const (
	StateRunning State = "running"
	StateStopped State = "stopped"
	StateFailed  State = "failed"
)

type StateRecord struct {
	SchemaVersion int       `json:"schema_version"`
	State         State     `json:"state"`
	Reason        string    `json:"reason"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (s State) Valid() bool {
	switch s {
	case StateRunning, StateStopped, StateFailed:
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
	case StateRunning:
		return to == StateStopped || to == StateFailed
	case StateStopped:
		return to == StateRunning || to == StateFailed
	case StateFailed:
		return to == StateRunning || to == StateStopped
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
