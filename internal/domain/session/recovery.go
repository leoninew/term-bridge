package session

import (
	"errors"
	"os"
	"time"

	"termbridge-go/internal/domain/process"
)

type AliveStatus int

const (
	AliveMissing AliveStatus = iota
	AliveMatched
	AliveUnverified
)

type AliveChecker interface {
	Check(process.Record) AliveStatus
}

type RecoveryStore interface {
	LoadProcess(workspaceKey string, sessionId string) (process.Record, error)
	LoadExit(workspaceKey string, sessionId string) (process.ExitRecord, error)
	SaveState(workspaceKey string, sessionId string, value StateRecord) error
}

type Recoverer struct {
	Store RecoveryStore
	Alive AliveChecker
	Now   func() time.Time
}

func (r Recoverer) Refresh(view View) (View, error) {
	if Terminal(view.State.State) {
		return view, nil
	}
	record, err := r.Store.LoadProcess(view.Session.WorkspaceKey, view.Session.Id)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return r.markFailed(view, "stale_process_missing")
		}
		return view, err
	}
	checker := r.Alive
	if checker == nil {
		checker = OSAliveChecker{}
	}
	switch checker.Check(record) {
	case AliveMatched:
		return view, nil
	case AliveMissing:
		if _, err := r.Store.LoadExit(view.Session.WorkspaceKey, view.Session.Id); err == nil {
			return r.markState(view, StateStopped, "recovered_exit_record")
		}
		return r.markFailed(view, "stale_process_missing")
	default:
		return r.markFailed(view, "stale_process_unverified")
	}
}

func (r Recoverer) markFailed(view View, reason string) (View, error) {
	return r.markState(view, StateFailed, reason)
}

func (r Recoverer) markState(view View, state State, reason string) (View, error) {
	view.State = StateRecord{SchemaVersion: SchemaVersion, State: state, Reason: reason, UpdatedAt: r.now()}
	if err := r.Store.SaveState(view.Session.WorkspaceKey, view.Session.Id, view.State); err != nil {
		return view, err
	}
	return view, nil
}

func (r Recoverer) now() time.Time {
	if r.Now != nil {
		return r.Now().UTC()
	}
	return time.Now().UTC()
}
