package workspace

import "time"

const SchemaVersion = 1

type Workspace struct {
	SchemaVersion        int       `json:"schema_version"`
	ID                   string    `json:"workspace_id"`
	Key                  string    `json:"workspace_key"`
	Name                 string    `json:"name"`
	Path                 string    `json:"path"`
	PathHashInputVersion int       `json:"path_hash_input_version"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}
