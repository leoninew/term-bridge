package file

import "context"

type WorkspaceChangeKind string

const (
	WorkspaceChangeAdded          WorkspaceChangeKind = "added"
	WorkspaceChangeUpdated        WorkspaceChangeKind = "updated"
	WorkspaceChangeDeleted        WorkspaceChangeKind = "deleted"
	WorkspaceChangeRenamed        WorkspaceChangeKind = "renamed"
	WorkspaceChangeRescanRequired WorkspaceChangeKind = "rescan_required"
)

type WorkspaceChange struct {
	Sequence uint64
	Kind     WorkspaceChangeKind
	Path     RelativePath
	OldPath  RelativePath
}

type WorkspaceChangeSubscription interface {
	Events() <-chan WorkspaceChange
	Close() error
}

type WorkspaceChangeSource interface {
	Subscribe(ctx context.Context, workspaceRoot string) (WorkspaceChangeSubscription, error)
	Close() error
}
