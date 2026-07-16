package git

import (
	"context"
	"errors"
	"os"

	filemodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/file"
	gitmodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/git"
)

type Service struct {
	config     Config
	workspaces gitmodel.WorkspaceLocator
	repository gitmodel.Repository
}

func NewService(config Config, workspaces gitmodel.WorkspaceLocator, repository gitmodel.Repository) (*Service, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if workspaces == nil || repository == nil {
		return nil, errors.New("workspace locator and Git repository are required")
	}
	return &Service{config: config, workspaces: workspaces, repository: repository}, nil
}

func (s *Service) Status(ctx context.Context, workspaceId string) (gitmodel.StatusResult, error) {
	operation, err := s.root(ctx, workspaceId)
	if err != nil {
		return gitmodel.StatusResult{}, err
	}
	defer operation.Cancel()
	return s.repository.Status(operation.Context, operation.Root)
}

func (s *Service) Diff(ctx context.Context, workspaceId string, request gitmodel.DiffRequest) (gitmodel.DiffResult, error) {
	if request.Layer != gitmodel.LayerStaged && request.Layer != gitmodel.LayerUnstaged && request.Layer != gitmodel.LayerUntracked {
		return gitmodel.DiffResult{}, filemodel.NewError("invalid_git_layer", "The Git diff layer is invalid.")
	}
	operation, err := s.root(ctx, workspaceId)
	if err != nil {
		return gitmodel.DiffResult{}, err
	}
	defer operation.Cancel()
	return s.repository.Diff(operation.Context, operation.Root, request)
}

type rootOperation struct {
	Root    string
	Context context.Context
	Cancel  context.CancelFunc
}

func (s *Service) root(ctx context.Context, workspaceId string) (rootOperation, error) {
	if err := ctx.Err(); err != nil {
		return rootOperation{}, err
	}
	workspace, err := s.workspaces.FindWorkspaceById(workspaceId)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return rootOperation{}, filemodel.NewError("workspace_not_found", "Workspace was not found.")
		}
		return rootOperation{}, err
	}
	operationCtx, cancel := context.WithTimeout(ctx, s.config.CommandTimeout)
	return rootOperation{Root: workspace.Path, Context: operationCtx, Cancel: cancel}, nil
}
