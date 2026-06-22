package process

import "time"

type Record struct {
	SchemaVersion int       `json:"schema_version"`
	PID           int       `json:"pid"`
	OwnerPID      int       `json:"owner_pid"`
	Executable    string    `json:"executable"`
	CommandLine   string    `json:"command_line"`
	Cwd           string    `json:"cwd"`
	StartedAt     time.Time `json:"started_at"`
}

type ExitRecord struct {
	SchemaVersion int       `json:"schema_version"`
	ExitCode      int       `json:"exit_code"`
	Reason        string    `json:"reason"`
	Forced        bool      `json:"forced"`
	Closed        bool      `json:"closed"`
	StartedAt     time.Time `json:"started_at"`
	EndedAt       time.Time `json:"ended_at"`
	WaitError     string    `json:"wait_error"`
}
