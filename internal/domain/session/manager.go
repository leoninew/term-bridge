package session

import (
	"time"

	"termbridge-go/internal/domain/identity"
	"termbridge-go/internal/domain/workspace"
)

type Store interface {
	SaveSession(Session) error
	SaveState(workspaceKey string, sessionId string, value StateRecord) error
}

type Manager struct {
	Store Store
	Ids   identity.Generator
	Now   func() time.Time
}

type CreateOptions struct {
	Workspace workspace.Workspace
	Name      string
	LaunchCwd string
	Command   CommandRecord
	History   HistoryRecord
	LogPath   string
}

func (m Manager) Create(options CreateOptions) (Session, error) {
	id, err := m.Ids.NewId()
	if err != nil {
		return Session{}, err
	}
	now := m.now()
	session := Session{
		SchemaVersion: SchemaVersion,
		Id:            id,
		Name:          options.Name,
		WorkspaceId:   options.Workspace.Id,
		WorkspaceKey:  options.Workspace.Key,
		LaunchCwd:     options.LaunchCwd,
		Command:       options.Command,
		History:       options.History,
		LogPath:       options.LogPath,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := m.Store.SaveSession(session); err != nil {
		return Session{}, err
	}
	if err := m.Store.SaveState(session.WorkspaceKey, session.Id, StateRecord{SchemaVersion: SchemaVersion, State: StateStarting, Reason: "session_created", UpdatedAt: now}); err != nil {
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
