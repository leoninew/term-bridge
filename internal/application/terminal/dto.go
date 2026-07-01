package terminal

import (
	"encoding/json"
	"fmt"
	"time"

	"termbridge-go/internal/domain/session"
)

type CreateSessionReq struct {
	WorkspaceId string   `json:"workspace_id,omitempty"`
	Name        string   `json:"name"`
	Cwd         string   `json:"cwd"`
	Command     []string `json:"command"`
	Cols        int      `json:"cols"`
	Rows        int      `json:"rows"`
}

type RerunSessionReq struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

type UpdateSessionReq struct {
	Name string `json:"name"`
}

func (r *UpdateSessionReq) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	for key := range raw {
		if key != "name" {
			return fmt.Errorf("unsupported session edit field %q", key)
		}
	}
	if value, ok := raw["name"]; ok {
		if err := json.Unmarshal(value, &r.Name); err != nil {
			return err
		}
	}
	return nil
}

type UpdateWorkspaceOrderReq struct {
	WorkspaceIds []string `json:"workspace_ids"`
}

type UpdateSessionOrderReq struct {
	SessionIds []string `json:"session_ids"`
}

type DeleteWorkspaceReq struct {
	WorkspaceId string `json:"workspace_id"`
}

type WorkspaceSessionsReq struct {
	WorkspaceId string `json:"workspace_id"`
}

type WorkspaceSessionOrderReq struct {
	WorkspaceId string   `json:"workspace_id"`
	SessionIds  []string `json:"session_ids"`
}

type WorkspaceSessionReq struct {
	WorkspaceId string `json:"workspace_id"`
	SessionId   string `json:"session_id"`
}

type UpdateWorkspaceSessionReq struct {
	WorkspaceId string           `json:"workspace_id"`
	SessionId   string           `json:"session_id"`
	Request     UpdateSessionReq `json:"request"`
}

type RerunWorkspaceSessionReq struct {
	WorkspaceId string          `json:"workspace_id"`
	SessionId   string          `json:"session_id"`
	Request     RerunSessionReq `json:"request"`
}

type ListWorkspacesResp struct {
	Items []WorkspaceSummary `json:"items"`
}

type WorkspaceTreeResp struct {
	Items []WorkspaceTreeNode `json:"items"`
}

type WorkspaceSessionsResp struct {
	Items []WorkspaceSessionSummary `json:"items"`
}

type UpdateWorkspaceOrderResp struct {
	Items []WorkspaceSummary `json:"items"`
}

type UpdateSessionOrderResp struct {
	Items []WorkspaceSessionSummary `json:"items"`
}

type WorkspaceTreeNode struct {
	Id        string                    `json:"id"`
	Name      string                    `json:"name"`
	Path      string                    `json:"path"`
	UpdatedAt time.Time                 `json:"updated_at"`
	Children  []WorkspaceSessionSummary `json:"children"`
}

type CreateSessionResp struct {
	SessionId   string `json:"session_id"`
	WorkspaceId string `json:"workspace_id"`
	State       string `json:"state"`
}

type WorkspaceSummary struct {
	Id        string    `json:"id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	UpdatedAt time.Time `json:"updated_at"`
}

type SessionSummary struct {
	Id              string          `json:"id"`
	Name            string          `json:"name"`
	WorkspaceId     string          `json:"workspace_id"`
	Command         string          `json:"command"`
	Cwd             string          `json:"cwd"`
	LifecycleState  session.State   `json:"lifecycle_state"`
	AttachmentState AttachmentState `json:"attachment_state,omitempty"`
	ExitCode        *int            `json:"exit_code,omitempty"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type WorkspaceSessionSummary struct {
	Id              string          `json:"id"`
	Name            string          `json:"name"`
	Command         string          `json:"command"`
	Cwd             string          `json:"cwd"`
	LifecycleState  session.State   `json:"lifecycle_state"`
	AttachmentState AttachmentState `json:"attachment_state,omitempty"`
	ExitCode        *int            `json:"exit_code,omitempty"`
	UpdatedAt       time.Time       `json:"updated_at"`
}
