package session

import (
	"time"

	sessionmodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/session"
	"gitee.com/leoninew/TermBridge-go/internal/agent/model/task/workspace"
	"gitee.com/leoninew/TermBridge-go/internal/shared/common/utils/idgen"
)

type Store interface {
	SaveSession(sessionmodel.Session) error
}

type Manager struct {
	Store Store
	Now   func() time.Time
}

type CreateOptions struct {
	Workspace workspace.Workspace
	Name      string
	LaunchCwd string
	Command   sessionmodel.CommandRecord
	History   sessionmodel.HistoryRecord
}

func (m Manager) Create(options CreateOptions) (sessionmodel.Session, error) {
	now := m.now()
	id, err := idgen.New()
	if err != nil {
		return sessionmodel.Session{}, err
	}
	session := sessionmodel.Session{
		SchemaVersion: sessionmodel.SchemaVersion,
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
		return sessionmodel.Session{}, err
	}
	return session, nil
}

func (m Manager) now() time.Time {
	if m.Now != nil {
		return m.Now().UTC()
	}
	return time.Now().UTC()
}
