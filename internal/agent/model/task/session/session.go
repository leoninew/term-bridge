package session

import "time"

type CommandSource string

const (
	CommandSourceShortcut CommandSource = "shortcut"
	CommandSourceCommand  CommandSource = "command"
)

type CommandRecord struct {
	Command              string        `json:"command"`
	EnvStrategy          string        `json:"env_strategy"`
	EnvCount             int           `json:"env_count"`
	Source               CommandSource `json:"source,omitempty"`
	ShortcutIdSnapshot   string        `json:"shortcut_id_snapshot,omitempty"`
	ShortcutNameSnapshot string        `json:"shortcut_name_snapshot,omitempty"`
}

func (r CommandRecord) ValidSource() bool {
	switch r.Source {
	case "":
		return r.ShortcutIdSnapshot == "" && r.ShortcutNameSnapshot == ""
	case CommandSourceCommand:
		return r.ShortcutIdSnapshot == "" && r.ShortcutNameSnapshot == ""
	case CommandSourceShortcut:
		return r.ShortcutIdSnapshot != "" && r.ShortcutNameSnapshot != ""
	default:
		return false
	}
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
