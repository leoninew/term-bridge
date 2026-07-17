package git

import (
	"context"

	workspace "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/workspace"
)

type WorkspaceLocator interface {
	FindWorkspaceById(workspaceId string) (workspace.Workspace, error)
}

type Repository interface {
	Identity(ctx context.Context, root string) (RepositoryIdentity, error)
	Status(ctx context.Context, root string) (StatusResult, error)
	Diff(ctx context.Context, root string, request DiffRequest) (DiffResult, error)
	Summary(ctx context.Context, root string) (RepositorySummary, error)
	MutatePath(ctx context.Context, root string, request PathMutationRequest) (OperationResult, error)
	Commit(ctx context.Context, root string, request CommitRequest) (OperationResult, error)
	CreateBranch(ctx context.Context, root string, request BranchRequest) (OperationResult, error)
	SwitchBranch(ctx context.Context, root string, request BranchRequest) (OperationResult, error)
}
