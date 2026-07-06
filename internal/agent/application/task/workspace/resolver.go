package workspace

import (
	"errors"
	"os"
	"time"

	workspacemodel "termbridge/internal/agent/model/task/workspace"
	"termbridge/internal/shared/common/utils/idgen"
)

type Store interface {
	FindWorkspaceByPath(path string) (workspacemodel.Workspace, error)
	SaveWorkspace(workspacemodel.Workspace) error
}

type Resolver struct {
	Store Store
	Now   func() time.Time
}

func (r Resolver) Resolve(path string) (workspacemodel.Workspace, error) {
	normalized, err := workspacemodel.NormalizePath(path)
	if err != nil {
		return workspacemodel.Workspace{}, err
	}
	if existing, err := r.Store.FindWorkspaceByPath(normalized); err == nil {
		existing.UpdatedAt = r.now()
		if err := r.Store.SaveWorkspace(existing); err != nil {
			return workspacemodel.Workspace{}, err
		}
		return existing, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return workspacemodel.Workspace{}, err
	}

	id, err := idgen.New()
	if err != nil {
		return workspacemodel.Workspace{}, err
	}
	now := r.now()
	workspace := workspacemodel.Workspace{
		SchemaVersion: workspacemodel.SchemaVersion,
		Id:            id,
		Name:          workspacemodel.NameForPath(normalized),
		Path:          normalized,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := r.Store.SaveWorkspace(workspace); err != nil {
		return workspacemodel.Workspace{}, err
	}
	return workspace, nil
}

func (r Resolver) now() time.Time {
	if r.Now != nil {
		return r.Now().UTC()
	}
	return time.Now().UTC()
}
