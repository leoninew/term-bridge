package workspace

import "time"

const SchemaVersion = 2

type CommandRecord struct {
	Command     string `json:"command"`
	EnvStrategy string `json:"env_strategy"`
	EnvCount    int    `json:"env_count"`
}

type HistoryRecord struct {
	Path         string `json:"path"`
	MaxLines     int    `json:"max_lines"`
	MaxBytes     int64  `json:"max_bytes"`
	MaxLineBytes int    `json:"max_line_bytes"`
	Truncated    bool   `json:"truncated"`
}

type StateRecord struct {
	SchemaVersion int       `json:"schema_version"`
	State         string    `json:"state"`
	Reason        string    `json:"reason"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ProcessRecord struct {
	SchemaVersion int       `json:"schema_version"`
	Pid           int       `json:"pid"`
	OwnerPid      int       `json:"owner_pid"`
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

type RunRecord struct {
	Process *ProcessRecord `json:"process,omitempty"`
	Exit    *ExitRecord    `json:"exit,omitempty"`
}

type SessionNode struct {
	Id         string        `json:"session_id"`
	Name       string        `json:"name"`
	LaunchCwd  string        `json:"launch_cwd"`
	Command    CommandRecord `json:"command"`
	History    HistoryRecord `json:"history"`
	State      StateRecord   `json:"state"`
	CurrentRun RunRecord     `json:"current_run"`
	CreatedAt  time.Time     `json:"created_at"`
	UpdatedAt  time.Time     `json:"updated_at"`
}

type WorkspaceIndex struct {
	SchemaVersion int       `json:"schema_version"`
	WorkspaceIds  []string  `json:"workspace_ids"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Workspace struct {
	SchemaVersion int           `json:"schema_version"`
	Id            string        `json:"workspace_id"`
	Name          string        `json:"name"`
	Path          string        `json:"path"`
	SessionIds    []string      `json:"session_ids"`
	Children      []SessionNode `json:"children"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}
