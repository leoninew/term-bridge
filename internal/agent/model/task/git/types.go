package git

import "time"

type State string

const (
	StateAvailable             State = "available"
	StateNotRepository         State = "not_repository"
	StateExecutableUnavailable State = "executable_unavailable"
	StateBinary                State = "binary"
	StateTooLarge              State = "too_large"
	StateUnmerged              State = "unmerged"
	StateSubmodule             State = "submodule"
	StateLayerUnavailable      State = "layer_unavailable"
	StateUnavailable           State = "unavailable"
)

type Layer string

const (
	LayerStaged    Layer = "staged"
	LayerUnstaged  Layer = "unstaged"
	LayerUntracked Layer = "untracked"
)

type MutationKind string

const (
	MutationStage           MutationKind = "stage"
	MutationUnstage         MutationKind = "unstage"
	MutationRestoreUnstaged MutationKind = "restore_unstaged"
	MutationDeleteUntracked MutationKind = "delete_untracked"
)

type OperationState string

const (
	OperationStateAvailable             OperationState = "available"
	OperationStateCleanWorktreeRequired OperationState = "clean_worktree_required"
	OperationStateNoStagedChanges       OperationState = "no_staged_changes"
	OperationStateBranchExists          OperationState = "branch_exists"
	OperationStateBranchNotFound        OperationState = "branch_not_found"
	OperationStateInvalidBranchName     OperationState = "invalid_branch_name"
	OperationStateInvalidCommitMessage  OperationState = "invalid_commit_message"
	OperationStateConflict              OperationState = "conflict"
	OperationStateCommitFailed          OperationState = "commit_failed"
	OperationStateUnavailable           OperationState = "unavailable"
)

type Change struct {
	Path            string
	OriginalPath    string
	IndexStatus     string
	WorktreeStatus  string
	Untracked       bool
	Unmerged        bool
	AvailableLayers []Layer
}

type StatusResult struct {
	State   State
	Changes []Change
	Message string
}

type DiffRequest struct {
	Path  string
	Layer Layer
}

type DiffResult struct {
	State        State
	OriginalPath string
	ModifiedPath string
	OriginalText string
	ModifiedText string
	Message      string
}

type PathMutationRequest struct {
	Path     string
	Layer    Layer
	Mutation MutationKind
}

type CommitRequest struct {
	Message string
}

type BranchRequest struct {
	Name string
}

type HistoryEntry struct {
	Id         string
	ShortId    string
	Subject    string
	AuthorName string
	AuthoredAt time.Time
}

type RepositorySummary struct {
	State         State
	CurrentBranch string
	LocalBranches []string
	History       []HistoryEntry
	Message       string
}

type RepositoryIdentity struct {
	State State
	Key   string
}

type OperationResult struct {
	State   OperationState
	Status  StatusResult
	Summary RepositorySummary
	Message string
}
