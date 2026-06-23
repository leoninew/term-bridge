package workspace

import (
	"errors"
	"os"
	"time"

	"termbridge-go/internal/domain/identity"
)

type Store interface {
	LoadWorkspace(key string) (Workspace, error)
	SaveWorkspace(Workspace) error
}

type Resolver struct {
	Store Store
	Ids   identity.Generator
	Now   func() time.Time
}

func (r Resolver) Resolve(path string) (Workspace, error) {
	key, normalized, err := KeyForPath(path)
	if err != nil {
		return Workspace{}, err
	}
	if existing, err := r.Store.LoadWorkspace(key); err == nil {
		existing.UpdatedAt = r.now()
		if err := r.Store.SaveWorkspace(existing); err != nil {
			return Workspace{}, err
		}
		return existing, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return Workspace{}, err
	}

	id, err := r.Ids.NewId()
	if err != nil {
		return Workspace{}, err
	}
	now := r.now()
	workspace := Workspace{
		SchemaVersion:        SchemaVersion,
		Id:                   id,
		Key:                  key,
		Name:                 NameForPath(normalized, key),
		Path:                 normalized,
		PathHashInputVersion: PathHashInputVersion,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := r.Store.SaveWorkspace(workspace); err != nil {
		return Workspace{}, err
	}
	return workspace, nil
}

func (r Resolver) now() time.Time {
	if r.Now != nil {
		return r.Now().UTC()
	}
	return time.Now().UTC()
}
