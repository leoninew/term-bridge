package session

import "time"

type CommandRecord struct {
	Executable  string   `json:"executable"`
	Command     string   `json:"command"`
	Args        []string `json:"args"`
	EnvStrategy string   `json:"env_strategy"`
	EnvCount    int      `json:"env_count"`
}

type HistoryRecord struct {
	Path         string `json:"path"`
	MaxLines     int    `json:"max_lines"`
	MaxBytes     int64  `json:"max_bytes"`
	MaxLineBytes int    `json:"max_line_bytes"`
	Truncated    bool   `json:"truncated"`
}

type Session struct {
	SchemaVersion int           `json:"schema_version"`
	Id            string        `json:"session_id"`
	Name          string        `json:"name"`
	WorkspaceId   string        `json:"workspace_id"`
	LaunchCwd     string        `json:"launch_cwd"`
	Command       CommandRecord `json:"command"`
	History       HistoryRecord `json:"history"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

type View struct {
	Session     Session
	State       StateRecord
	ExitCode    *int
	ExitReason  string
	CommandText string
}
