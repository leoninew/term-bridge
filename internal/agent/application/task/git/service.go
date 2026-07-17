package git

import (
	"context"
	"errors"
	"os"
	"sync"

	filemodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/file"
	gitmodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/git"
)

type Service struct {
	config     Config
	workspaces gitmodel.WorkspaceLocator
	repository gitmodel.Repository
	locks      sync.Map
}

type repositoryLock chan struct{}

func newRepositoryLock() repositoryLock {
	lock := make(repositoryLock, 1)
	lock <- struct{}{}
	return lock
}

func (l repositoryLock) Lock(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case <-l:
		return true
	}
}

func (l repositoryLock) Unlock() {
	l <- struct{}{}
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
	lock, state, err := s.repositoryLock(operation.Context, operation.Root)
	if err != nil {
		return gitmodel.StatusResult{}, err
	}
	if state != gitmodel.StateAvailable {
		return gitmodel.StatusResult{State: state}, nil
	}
	if !lock.Lock(operation.Context) {
		return gitmodel.StatusResult{State: gitmodel.StateUnavailable}, nil
	}
	defer lock.Unlock()
	return s.repository.Status(operation.Context, operation.Root)
}

func (s *Service) Diff(ctx context.Context, workspaceId string, request gitmodel.DiffRequest) (gitmodel.DiffResult, error) {
	if !validLayer(request.Layer) {
		return gitmodel.DiffResult{}, filemodel.NewError("invalid_git_layer", "The Git diff layer is invalid.")
	}
	operation, err := s.root(ctx, workspaceId)
	if err != nil {
		return gitmodel.DiffResult{}, err
	}
	defer operation.Cancel()
	lock, state, err := s.repositoryLock(operation.Context, operation.Root)
	if err != nil {
		return gitmodel.DiffResult{}, err
	}
	if state != gitmodel.StateAvailable {
		return gitmodel.DiffResult{State: state}, nil
	}
	if !lock.Lock(operation.Context) {
		return gitmodel.DiffResult{State: gitmodel.StateUnavailable}, nil
	}
	defer lock.Unlock()
	return s.repository.Diff(operation.Context, operation.Root, request)
}

func (s *Service) Summary(ctx context.Context, workspaceId string) (gitmodel.RepositorySummary, error) {
	operation, err := s.root(ctx, workspaceId)
	if err != nil {
		return gitmodel.RepositorySummary{}, err
	}
	defer operation.Cancel()
	lock, state, err := s.repositoryLock(operation.Context, operation.Root)
	if err != nil {
		return gitmodel.RepositorySummary{}, err
	}
	if state != gitmodel.StateAvailable {
		return gitmodel.RepositorySummary{State: state}, nil
	}
	if !lock.Lock(operation.Context) {
		return gitmodel.RepositorySummary{State: gitmodel.StateUnavailable}, nil
	}
	defer lock.Unlock()
	return s.repository.Summary(operation.Context, operation.Root)
}

func (s *Service) MutatePath(ctx context.Context, workspaceId string, request gitmodel.PathMutationRequest) (gitmodel.OperationResult, error) {
	if err := validatePathMutationRequest(request); err != nil {
		return gitmodel.OperationResult{State: gitmodel.OperationStateConflict, Message: "Git status changed. Refresh, then retry the operation."}, nil
	}
	operation, err := s.root(ctx, workspaceId)
	if err != nil {
		return gitmodel.OperationResult{}, err
	}
	defer operation.Cancel()
	lock, state, err := s.repositoryLock(operation.Context, operation.Root)
	if err != nil {
		return gitmodel.OperationResult{}, err
	}
	if state != gitmodel.StateAvailable {
		return gitmodel.OperationResult{State: gitmodel.OperationStateUnavailable, Status: gitmodel.StatusResult{State: state}}, nil
	}
	if !lock.Lock(operation.Context) {
		return gitmodel.OperationResult{State: gitmodel.OperationStateUnavailable, Status: gitmodel.StatusResult{State: gitmodel.StateUnavailable}}, nil
	}
	defer lock.Unlock()
	return s.repository.MutatePath(operation.Context, operation.Root, request)
}

func (s *Service) Commit(ctx context.Context, workspaceId string, request gitmodel.CommitRequest) (gitmodel.OperationResult, error) {
	if err := validateCommitMessage(request.Message); err != nil {
		return gitmodel.OperationResult{State: gitmodel.OperationStateInvalidCommitMessage, Message: "The commit message is invalid."}, nil
	}
	operation, err := s.root(ctx, workspaceId)
	if err != nil {
		return gitmodel.OperationResult{}, err
	}
	defer operation.Cancel()
	lock, state, err := s.repositoryLock(operation.Context, operation.Root)
	if err != nil {
		return gitmodel.OperationResult{}, err
	}
	if state != gitmodel.StateAvailable {
		return gitmodel.OperationResult{State: gitmodel.OperationStateUnavailable, Status: gitmodel.StatusResult{State: state}}, nil
	}
	if !lock.Lock(operation.Context) {
		return gitmodel.OperationResult{State: gitmodel.OperationStateUnavailable, Status: gitmodel.StatusResult{State: gitmodel.StateUnavailable}}, nil
	}
	defer lock.Unlock()
	return s.repository.Commit(operation.Context, operation.Root, request)
}

func (s *Service) CreateBranch(ctx context.Context, workspaceId string, request gitmodel.BranchRequest) (gitmodel.OperationResult, error) {
	if err := validateBranchName(request.Name); err != nil {
		return gitmodel.OperationResult{State: gitmodel.OperationStateInvalidBranchName, Message: "The branch name is invalid."}, nil
	}
	operation, err := s.root(ctx, workspaceId)
	if err != nil {
		return gitmodel.OperationResult{}, err
	}
	defer operation.Cancel()
	lock, state, err := s.repositoryLock(operation.Context, operation.Root)
	if err != nil {
		return gitmodel.OperationResult{}, err
	}
	if state != gitmodel.StateAvailable {
		return gitmodel.OperationResult{State: gitmodel.OperationStateUnavailable, Status: gitmodel.StatusResult{State: state}}, nil
	}
	if !lock.Lock(operation.Context) {
		return gitmodel.OperationResult{State: gitmodel.OperationStateUnavailable, Status: gitmodel.StatusResult{State: gitmodel.StateUnavailable}}, nil
	}
	defer lock.Unlock()
	return s.repository.CreateBranch(operation.Context, operation.Root, request)
}

func (s *Service) SwitchBranch(ctx context.Context, workspaceId string, request gitmodel.BranchRequest) (gitmodel.OperationResult, error) {
	if err := validateBranchName(request.Name); err != nil {
		return gitmodel.OperationResult{State: gitmodel.OperationStateInvalidBranchName, Message: "The branch name is invalid."}, nil
	}
	operation, err := s.root(ctx, workspaceId)
	if err != nil {
		return gitmodel.OperationResult{}, err
	}
	defer operation.Cancel()
	lock, state, err := s.repositoryLock(operation.Context, operation.Root)
	if err != nil {
		return gitmodel.OperationResult{}, err
	}
	if state != gitmodel.StateAvailable {
		return gitmodel.OperationResult{State: gitmodel.OperationStateUnavailable, Status: gitmodel.StatusResult{State: state}}, nil
	}
	if !lock.Lock(operation.Context) {
		return gitmodel.OperationResult{State: gitmodel.OperationStateUnavailable, Status: gitmodel.StatusResult{State: gitmodel.StateUnavailable}}, nil
	}
	defer lock.Unlock()
	return s.repository.SwitchBranch(operation.Context, operation.Root, request)
}

func validLayer(value gitmodel.Layer) bool {
	return value == gitmodel.LayerStaged || value == gitmodel.LayerUnstaged || value == gitmodel.LayerUntracked
}

func validatePathMutationRequest(request gitmodel.PathMutationRequest) error {
	if _, err := filemodel.ParseRelativePath(request.Path, false); err != nil || !validLayer(request.Layer) {
		return errors.New("invalid Git path mutation")
	}
	switch request.Mutation {
	case gitmodel.MutationStage:
		if request.Layer == gitmodel.LayerUnstaged || request.Layer == gitmodel.LayerUntracked {
			return nil
		}
	case gitmodel.MutationUnstage:
		if request.Layer == gitmodel.LayerStaged {
			return nil
		}
	case gitmodel.MutationRestoreUnstaged:
		if request.Layer == gitmodel.LayerUnstaged {
			return nil
		}
	case gitmodel.MutationDeleteUntracked:
		if request.Layer == gitmodel.LayerUntracked {
			return nil
		}
	}
	return errors.New("invalid Git path mutation")
}

func (s *Service) repositoryLock(ctx context.Context, root string) (repositoryLock, gitmodel.State, error) {
	identity, err := s.repository.Identity(ctx, root)
	if err != nil {
		return nil, gitmodel.StateUnavailable, err
	}
	if identity.State != gitmodel.StateAvailable || identity.Key == "" {
		if identity.State == gitmodel.StateAvailable {
			return nil, gitmodel.StateUnavailable, nil
		}
		return nil, identity.State, nil
	}
	value, _ := s.locks.LoadOrStore(identity.Key, newRepositoryLock())
	return value.(repositoryLock), gitmodel.StateAvailable, nil
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
