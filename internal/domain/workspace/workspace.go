package workspace

import "time"

const SchemaVersion = 1

type CommandRecord struct {
	Executable  string   `json:"executable"`
	Command     string   `json:"command"`
	Args        []string `json:"args"`
	EnvStrategy string   `json:"env_strategy"`
	EnvCount    int      `json:"env_count"`
}

type SessionNode struct {
	Id        string        `json:"session_id"`
	Name      string        `json:"name"`
	LaunchCwd string        `json:"launch_cwd"`
	Command   CommandRecord `json:"command"`
	LogPath   string        `json:"log_path"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

type Workspace struct {
	SchemaVersion        int           `json:"schema_version"`
	Id                   string        `json:"workspace_id"`
	Key                  string        `json:"workspace_key"`
	Name                 string        `json:"name"`
	Path                 string        `json:"path"`
	SortOrder            int           `json:"sort_order"`
	Children             []SessionNode `json:"children"`
	PathHashInputVersion int           `json:"path_hash_input_version"`
	CreatedAt            time.Time     `json:"created_at"`
	UpdatedAt            time.Time     `json:"updated_at"`
}
