package session

import (
	"time"

	"termbridge-go/internal/shared/idgen"
	"termbridge-go/internal/agent/domain/workspace"
)

type Store interface {
	SaveSession(Session) error
}

type Manager struct {
	Store Store
	Now   func() time.Time
}

type CreateOptions struct {
	Workspace workspace.Workspace
	Name      string
	LaunchCwd string
	Command   CommandRecord
	History   HistoryRecord
}

func (m Manager) Create(options CreateOptions) (Session, error) {
	now := m.now()
	id, err := idgen.New()
	if err != nil {
		return Session{}, err
	}
	session := Session{
		SchemaVersion: SchemaVersion,
		Id:            id,
		Name:          options.Name,
		WorkspaceId:   options.Workspace.Id,
		LaunchCwd:     options.LaunchCwd,
		Command:       options.Command,
		History:       options.History,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := m.Store.SaveSession(session); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (m Manager) now() time.Time {
	if m.Now != nil {
		return m.Now().UTC()
	}
	return time.Now().UTC()
}
