package git

import (
	"context"

	workspace "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/workspace"
)

type WorkspaceLocator interface {
	FindWorkspaceById(workspaceId string) (workspace.Workspace, error)
}

type Repository interface {
	Status(ctx context.Context, root string) (StatusResult, error)
	Diff(ctx context.Context, root string, request DiffRequest) (DiffResult, error)
}
