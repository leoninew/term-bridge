package state

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"termbridge-go/internal/agent/model/task/process"
	"termbridge-go/internal/agent/model/task/session"
	"termbridge-go/internal/agent/model/task/workspace"
	"termbridge-go/internal/shared/common/utils/idgen"
)

type DBStore struct {
	db       *sql.DB
	driver   string
	root     string
	deviceId string
}

type dbSessionRow struct {
	session      session.Session
	state        session.StateRecord
	currentRunId string
	processJSON  string
	exitJSON     string
	runUpdatedAt time.Time
}

func NewDBStore(db *sql.DB, driver string, root string, deviceId string) DBStore {
	return DBStore{db: db, driver: strings.ToLower(strings.TrimSpace(driver)), root: root, deviceId: strings.TrimSpace(deviceId)}
}

func (s DBStore) WorkspaceRoot() string {
	return filepath.Join(s.root, "workspaces")
}

func (s DBStore) WorkspaceDir(workspaceId string) string {
	return filepath.Join(s.WorkspaceRoot(), workspaceId)
}

func (s DBStore) SessionDir(workspaceId string, sessionId string) string {
	return filepath.Join(s.WorkspaceDir(workspaceId), "sessions", sessionId)
}

func (s DBStore) SaveWorkspace(value workspace.Workspace) error {
	if err := s.validate(); err != nil {
		return err
	}
	value.Id = strings.TrimSpace(value.Id)
	value.Name = strings.TrimSpace(value.Name)
	value.Path = strings.TrimSpace(value.Path)
	if value.Id == "" || value.Path == "" {
		return errors.New("workspace id and path are required")
	}
	if value.Name == "" {
		value.Name = workspace.NameForPath(value.Path)
	}
	now := time.Now().UTC()
	if value.CreatedAt.IsZero() {
		value.CreatedAt = now
	}
	if value.UpdatedAt.IsZero() {
		value.UpdatedAt = value.CreatedAt
	}
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if exists, err := s.activeWorkspaceExistsTx(ctx, tx, value.Id); err != nil {
		return err
	} else if exists {
		_, err := tx.ExecContext(ctx, `UPDATE workspaces SET name=?, path=?, updated_at=? WHERE id=? AND device_id=? AND deleted_at IS NULL`, value.Name, value.Path, s.storeTime(value.UpdatedAt), value.Id, s.deviceId)
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	var existingId string
	err = tx.QueryRowContext(ctx, `SELECT id FROM workspaces WHERE device_id=? AND path=? AND deleted_at IS NULL ORDER BY updated_at DESC LIMIT 1`, s.deviceId, value.Path).Scan(&existingId)
	if err == nil {
		_, err := tx.ExecContext(ctx, `UPDATE workspaces SET name=?, updated_at=? WHERE id=?`, value.Name, s.storeTime(value.UpdatedAt), existingId)
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	sortOrder, err := s.nextWorkspaceOrderTx(ctx, tx)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO workspaces (id,device_id,name,path,sort_order,metadata_json,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?)`, value.Id, s.deviceId, value.Name, value.Path, sortOrder, `{}`, s.storeTime(value.CreatedAt), s.storeTime(value.UpdatedAt))
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s DBStore) LoadWorkspace(workspaceId string) (workspace.Workspace, error) {
	return s.FindWorkspaceById(workspaceId)
}

func (s DBStore) FindWorkspaceByPath(path string) (workspace.Workspace, error) {
	if err := s.validate(); err != nil {
		return workspace.Workspace{}, err
	}
	path = strings.TrimSpace(path)
	row := s.db.QueryRowContext(context.Background(), `SELECT id,name,path,created_at,updated_at FROM workspaces WHERE device_id=? AND path=? AND deleted_at IS NULL ORDER BY updated_at DESC LIMIT 1`, s.deviceId, path)
	ws, err := s.scanWorkspace(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return workspace.Workspace{}, os.ErrNotExist
		}
		return workspace.Workspace{}, err
	}
	return s.populateWorkspace(ws)
}

func (s DBStore) FindWorkspaceById(workspaceId string) (workspace.Workspace, error) {
	if err := s.validate(); err != nil {
		return workspace.Workspace{}, err
	}
	row := s.db.QueryRowContext(context.Background(), `SELECT id,name,path,created_at,updated_at FROM workspaces WHERE id=? AND device_id=? AND deleted_at IS NULL`, strings.TrimSpace(workspaceId), s.deviceId)
	ws, err := s.scanWorkspace(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return workspace.Workspace{}, os.ErrNotExist
		}
		return workspace.Workspace{}, err
	}
	return s.populateWorkspace(ws)
}

func (s DBStore) SaveSession(value session.Session) error {
	if err := s.validate(); err != nil {
		return err
	}
	value.Id = strings.TrimSpace(value.Id)
	value.WorkspaceId = strings.TrimSpace(value.WorkspaceId)
	value.Name = strings.TrimSpace(value.Name)
	value.LaunchCwd = strings.TrimSpace(value.LaunchCwd)
	if value.Id == "" || value.WorkspaceId == "" || value.Name == "" {
		return errors.New("session id, workspace id and name are required")
	}
	now := time.Now().UTC()
	if value.CreatedAt.IsZero() {
		value.CreatedAt = now
	}
	if value.UpdatedAt.IsZero() {
		value.UpdatedAt = value.CreatedAt
	}
	commandJSON, err := marshalJSON(value.Command)
	if err != nil {
		return fmt.Errorf("marshal session command: %w", err)
	}
	historyJSON, err := marshalJSON(value.History)
	if err != nil {
		return fmt.Errorf("marshal session history: %w", err)
	}
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if exists, err := s.activeWorkspaceExistsTx(ctx, tx, value.WorkspaceId); err != nil {
		return err
	} else if !exists {
		return os.ErrNotExist
	}
	if exists, err := s.activeSessionExistsTx(ctx, tx, value.WorkspaceId, value.Id); err != nil {
		return err
	} else if exists {
		_, err := tx.ExecContext(ctx, `UPDATE sessions SET name=?, launch_cwd=?, command_json=?, history_json=?, updated_at=? WHERE id=? AND workspace_id=? AND device_id=? AND deleted_at IS NULL`, value.Name, value.LaunchCwd, commandJSON, historyJSON, s.storeTime(value.UpdatedAt), value.Id, value.WorkspaceId, s.deviceId)
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	sortOrder, err := s.nextSessionOrderTx(ctx, tx, value.WorkspaceId)
	if err != nil {
		return err
	}
	runId, err := idgen.New()
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO sessions (id,workspace_id,device_id,name,launch_cwd,command_json,history_json,current_state,current_state_reason,current_run_id,sort_order,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`, value.Id, value.WorkspaceId, s.deviceId, value.Name, value.LaunchCwd, commandJSON, historyJSON, string(session.StateRunning), "created", runId, sortOrder, s.storeTime(value.CreatedAt), s.storeTime(value.UpdatedAt))
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO session_runs (id,session_id,workspace_id,device_id,sequence,command_json,terminal_size_json,process_json,exit_json,state,state_reason,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`, runId, value.Id, value.WorkspaceId, s.deviceId, 1, commandJSON, `{}`, `{}`, `{}`, string(session.StateRunning), "created", s.storeTime(value.CreatedAt), s.storeTime(value.UpdatedAt))
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s DBStore) LoadSession(workspaceId string, sessionId string) (session.Session, error) {
	row, err := s.loadSessionRow(strings.TrimSpace(workspaceId), strings.TrimSpace(sessionId))
	if err != nil {
		return session.Session{}, err
	}
	return row.session, nil
}

func (s DBStore) UpdateSession(workspaceId string, sessionId string, update func(*session.Session) error) (session.Session, error) {
	value, err := s.LoadSession(workspaceId, sessionId)
	if err != nil {
		return session.Session{}, err
	}
	if err := update(&value); err != nil {
		return session.Session{}, err
	}
	if err := s.SaveSession(value); err != nil {
		return session.Session{}, err
	}
	return value, nil
}

func (s DBStore) DeleteSession(workspaceId string, sessionId string) error {
	if err := s.validate(); err != nil {
		return err
	}
	now := time.Now().UTC()
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE sessions SET deleted_at=?, updated_at=? WHERE id=? AND workspace_id=? AND device_id=? AND deleted_at IS NULL`, s.storeTime(now), s.storeTime(now), strings.TrimSpace(sessionId), strings.TrimSpace(workspaceId), s.deviceId)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return os.ErrNotExist
	}
	if _, err := tx.ExecContext(ctx, `UPDATE session_runs SET deleted_at=?, updated_at=? WHERE session_id=? AND workspace_id=? AND device_id=? AND deleted_at IS NULL`, s.storeTime(now), s.storeTime(now), strings.TrimSpace(sessionId), strings.TrimSpace(workspaceId), s.deviceId); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE workspaces SET updated_at=? WHERE id=? AND device_id=? AND deleted_at IS NULL`, s.storeTime(now), strings.TrimSpace(workspaceId), s.deviceId); err != nil {
		return err
	}
	return tx.Commit()
}

func (s DBStore) DeleteWorkspace(workspaceId string) error {
	if err := s.validate(); err != nil {
		return err
	}
	now := time.Now().UTC()
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	workspaceId = strings.TrimSpace(workspaceId)
	result, err := tx.ExecContext(ctx, `UPDATE workspaces SET deleted_at=?, updated_at=? WHERE id=? AND device_id=? AND deleted_at IS NULL`, s.storeTime(now), s.storeTime(now), workspaceId, s.deviceId)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return os.ErrNotExist
	}
	if _, err := tx.ExecContext(ctx, `UPDATE sessions SET deleted_at=?, updated_at=? WHERE workspace_id=? AND device_id=? AND deleted_at IS NULL`, s.storeTime(now), s.storeTime(now), workspaceId, s.deviceId); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE session_runs SET deleted_at=?, updated_at=? WHERE workspace_id=? AND device_id=? AND deleted_at IS NULL`, s.storeTime(now), s.storeTime(now), workspaceId, s.deviceId); err != nil {
		return err
	}
	return tx.Commit()
}

func (s DBStore) SaveState(workspaceId string, sessionId string, value session.StateRecord) error {
	if !value.State.Valid() {
		return fmt.Errorf("invalid lifecycle state %q", value.State)
	}
	if value.UpdatedAt.IsZero() {
		value.UpdatedAt = time.Now().UTC()
	}
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	runId, err := s.ensureCurrentRunTx(ctx, tx, workspaceId, sessionId, value.State == session.StateRunning, nil, value.UpdatedAt)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE session_runs SET state=?, state_reason=?, updated_at=? WHERE id=? AND deleted_at IS NULL`, string(value.State), value.Reason, s.storeTime(value.UpdatedAt), runId); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE sessions SET current_state=?, current_state_reason=?, updated_at=? WHERE id=? AND workspace_id=? AND device_id=? AND deleted_at IS NULL`, string(value.State), value.Reason, s.storeTime(value.UpdatedAt), strings.TrimSpace(sessionId), strings.TrimSpace(workspaceId), s.deviceId); err != nil {
		return err
	}
	return tx.Commit()
}

func (s DBStore) LoadState(workspaceId string, sessionId string) (session.StateRecord, error) {
	row := s.db.QueryRowContext(context.Background(), `SELECT current_state,current_state_reason,updated_at FROM sessions WHERE id=? AND workspace_id=? AND device_id=? AND deleted_at IS NULL`, strings.TrimSpace(sessionId), strings.TrimSpace(workspaceId), s.deviceId)
	var stateText, reason string
	var updatedAt any
	if err := row.Scan(&stateText, &reason, &updatedAt); err != nil {
		return session.StateRecord{}, err
	}
	parsedUpdatedAt, err := dbScanTime(updatedAt)
	if err != nil {
		return session.StateRecord{}, err
	}
	stateRecord := session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.State(stateText), Reason: reason, UpdatedAt: parsedUpdatedAt}
	if !stateRecord.State.Valid() {
		return session.StateRecord{}, fmt.Errorf("invalid lifecycle state %q", stateText)
	}
	return stateRecord, nil
}

func (s DBStore) SaveProcess(workspaceId string, sessionId string, value process.Record) error {
	updatedAt := value.StartedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	processJSON, err := marshalJSON(value)
	if err != nil {
		return fmt.Errorf("marshal process record: %w", err)
	}
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	runId, err := s.ensureCurrentRunTx(ctx, tx, workspaceId, sessionId, true, nil, updatedAt)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE session_runs SET process_json=?, state=?, state_reason=?, started_at=?, updated_at=? WHERE id=? AND deleted_at IS NULL`, processJSON, string(session.StateRunning), "process_started", s.storeNullableTime(value.StartedAt), s.storeTime(updatedAt), runId)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE sessions SET current_state=?, current_state_reason=?, updated_at=? WHERE id=? AND workspace_id=? AND device_id=? AND deleted_at IS NULL`, string(session.StateRunning), "process_started", s.storeTime(updatedAt), strings.TrimSpace(sessionId), strings.TrimSpace(workspaceId), s.deviceId)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s DBStore) LoadProcess(workspaceId string, sessionId string) (process.Record, error) {
	var payload string
	err := s.db.QueryRowContext(context.Background(), `SELECT COALESCE(sr.process_json,'{}') FROM sessions sess JOIN session_runs sr ON sr.id=sess.current_run_id WHERE sess.id=? AND sess.workspace_id=? AND sess.device_id=? AND sess.deleted_at IS NULL AND sr.deleted_at IS NULL`, strings.TrimSpace(sessionId), strings.TrimSpace(workspaceId), s.deviceId).Scan(&payload)
	if err != nil {
		return process.Record{}, err
	}
	if emptyJSON(payload) {
		return process.Record{}, os.ErrNotExist
	}
	var record process.Record
	if err := json.Unmarshal([]byte(payload), &record); err != nil {
		return process.Record{}, fmt.Errorf("decode process record: %w", err)
	}
	if record.SchemaVersion == 0 && record.Pid == 0 && record.StartedAt.IsZero() && record.CommandLine == "" && record.Executable == "" {
		return process.Record{}, os.ErrNotExist
	}
	return record, nil
}

func (s DBStore) SaveExit(workspaceId string, sessionId string, value process.ExitRecord) error {
	updatedAt := value.EndedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	exitJSON, err := marshalJSON(value)
	if err != nil {
		return fmt.Errorf("marshal exit record: %w", err)
	}
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	runId, err := s.ensureCurrentRunTx(ctx, tx, workspaceId, sessionId, false, nil, updatedAt)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE session_runs SET exit_json=?, state=?, state_reason=?, ended_at=?, updated_at=? WHERE id=? AND deleted_at IS NULL`, exitJSON, string(session.StateStopped), value.Reason, s.storeNullableTime(value.EndedAt), s.storeTime(updatedAt), runId)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE sessions SET current_state=?, current_state_reason=?, updated_at=? WHERE id=? AND workspace_id=? AND device_id=? AND deleted_at IS NULL`, string(session.StateStopped), value.Reason, s.storeTime(updatedAt), strings.TrimSpace(sessionId), strings.TrimSpace(workspaceId), s.deviceId)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s DBStore) LoadExit(workspaceId string, sessionId string) (process.ExitRecord, error) {
	var payload string
	err := s.db.QueryRowContext(context.Background(), `SELECT COALESCE(sr.exit_json,'{}') FROM sessions sess JOIN session_runs sr ON sr.id=sess.current_run_id WHERE sess.id=? AND sess.workspace_id=? AND sess.device_id=? AND sess.deleted_at IS NULL AND sr.deleted_at IS NULL`, strings.TrimSpace(sessionId), strings.TrimSpace(workspaceId), s.deviceId).Scan(&payload)
	if err != nil {
		return process.ExitRecord{}, err
	}
	if emptyJSON(payload) {
		return process.ExitRecord{}, os.ErrNotExist
	}
	var record process.ExitRecord
	if err := json.Unmarshal([]byte(payload), &record); err != nil {
		return process.ExitRecord{}, fmt.Errorf("decode exit record: %w", err)
	}
	if record.SchemaVersion == 0 && record.ExitCode == 0 && record.Reason == "" && record.EndedAt.IsZero() {
		return process.ExitRecord{}, os.ErrNotExist
	}
	return record, nil
}

func (s DBStore) BeginSessionRun(workspaceId string, sessionId string, size process.TerminalSize) error {
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	now := time.Now().UTC()
	_, err = s.ensureCurrentRunTx(ctx, tx, workspaceId, sessionId, true, &size, now)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s DBStore) HistoryPath(workspaceId string, sessionId string) string {
	return filepath.Join(s.SessionDir(workspaceId, sessionId), "history.log")
}

func (s DBStore) ListWorkspaces() ([]workspace.Workspace, []Warning, error) {
	if err := s.validate(); err != nil {
		return nil, nil, err
	}
	rows, err := s.db.QueryContext(context.Background(), `SELECT id,name,path,created_at,updated_at FROM workspaces WHERE device_id=? AND deleted_at IS NULL ORDER BY sort_order ASC, created_at ASC, name ASC, id ASC`, s.deviceId)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()
	var values []workspace.Workspace
	for rows.Next() {
		ws, err := s.scanWorkspace(rows)
		if err != nil {
			return nil, nil, err
		}
		values = append(values, ws)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, nil, err
	}
	for i := range values {
		populated, err := s.populateWorkspace(values[i])
		if err != nil {
			return nil, nil, err
		}
		values[i] = populated
	}
	return values, nil, nil
}

func (s DBStore) ListSessionsByWorkspaceId(workspaceId string) ([]session.View, []Warning, error) {
	if _, err := s.FindWorkspaceById(workspaceId); err != nil {
		return nil, nil, err
	}
	views, err := s.listSessionViews(`WHERE sess.workspace_id=? AND sess.device_id=? AND sess.deleted_at IS NULL`, strings.TrimSpace(workspaceId), s.deviceId)
	return views, nil, err
}

func (s DBStore) ListSessions() ([]session.View, []Warning, error) {
	views, err := s.listSessionViews(`WHERE sess.device_id=? AND sess.deleted_at IS NULL`, s.deviceId)
	return views, nil, err
}

func (s DBStore) UpdateWorkspaceOrder(workspaceIds []string, now time.Time) ([]workspace.Workspace, error) {
	workspaces, _, err := s.ListWorkspaces()
	if err != nil {
		return nil, err
	}
	ordered, err := completeOrderedIds(workspaceIds, workspaces, func(ws workspace.Workspace) string { return ws.Id }, "workspace_id")
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	for index, id := range ordered {
		if _, err := tx.ExecContext(ctx, `UPDATE workspaces SET sort_order=?, updated_at=? WHERE id=? AND device_id=? AND deleted_at IS NULL`, index, s.storeTime(now), id, s.deviceId); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	updated, _, err := s.ListWorkspaces()
	return updated, err
}

func (s DBStore) UpdateSessionOrder(workspaceId string, sessionIds []string, now time.Time) ([]session.View, []Warning, error) {
	views, _, err := s.ListSessionsByWorkspaceId(workspaceId)
	if err != nil {
		return nil, nil, err
	}
	ordered, err := completeOrderedIds(sessionIds, views, func(view session.View) string { return view.Session.Id }, "session_id")
	if err != nil {
		return nil, nil, err
	}
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback() }()
	for index, id := range ordered {
		if _, err := tx.ExecContext(ctx, `UPDATE sessions SET sort_order=?, updated_at=? WHERE id=? AND workspace_id=? AND device_id=? AND deleted_at IS NULL`, index, s.storeTime(now), id, strings.TrimSpace(workspaceId), s.deviceId); err != nil {
			return nil, nil, err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE workspaces SET updated_at=? WHERE id=? AND device_id=? AND deleted_at IS NULL`, s.storeTime(now), strings.TrimSpace(workspaceId), s.deviceId); err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}
	updated, warnings, err := s.ListSessionsByWorkspaceId(workspaceId)
	return updated, warnings, err
}

func (s DBStore) validate() error {
	if s.db == nil {
		return errors.New("database is required")
	}
	if strings.TrimSpace(s.deviceId) == "" {
		return errors.New("device id is required")
	}
	return nil
}

func (s DBStore) populateWorkspace(ws workspace.Workspace) (workspace.Workspace, error) {
	views, err := s.listSessionViews(`WHERE sess.workspace_id=? AND sess.device_id=? AND sess.deleted_at IS NULL`, ws.Id, s.deviceId)
	if err != nil {
		return workspace.Workspace{}, err
	}
	ws.Children = make([]workspace.SessionNode, 0, len(views))
	ws.SessionIds = make([]string, 0, len(views))
	for _, view := range views {
		node, err := s.sessionNodeFromView(view)
		if err != nil {
			return workspace.Workspace{}, err
		}
		ws.Children = append(ws.Children, node)
		ws.SessionIds = append(ws.SessionIds, node.Id)
	}
	return ws, nil
}

func (s DBStore) sessionNodeFromView(view session.View) (workspace.SessionNode, error) {
	state := workspaceStateFromSession(view.State)
	node := workspace.SessionNode{
		Id:        view.Session.Id,
		Name:      view.Session.Name,
		LaunchCwd: view.Session.LaunchCwd,
		Command: workspace.CommandRecord{
			Executable:  view.Session.Command.Executable,
			Command:     view.Session.Command.Command,
			Args:        append([]string(nil), view.Session.Command.Args...),
			EnvStrategy: view.Session.Command.EnvStrategy,
			EnvCount:    view.Session.Command.EnvCount,
		},
		History: workspace.HistoryRecord{
			Path:         view.Session.History.Path,
			MaxLines:     view.Session.History.MaxLines,
			MaxBytes:     view.Session.History.MaxBytes,
			MaxLineBytes: view.Session.History.MaxLineBytes,
			Truncated:    view.Session.History.Truncated,
		},
		State:     state,
		CreatedAt: view.Session.CreatedAt,
		UpdatedAt: view.Session.UpdatedAt,
	}
	if record, err := s.LoadProcess(view.Session.WorkspaceId, view.Session.Id); err == nil {
		processRecord := workspaceProcessFromProcess(record)
		node.CurrentRun.Process = &processRecord
	} else if !errors.Is(err, os.ErrNotExist) {
		return workspace.SessionNode{}, err
	}
	if record, err := s.LoadExit(view.Session.WorkspaceId, view.Session.Id); err == nil {
		exitRecord := workspaceExitFromProcess(record)
		node.CurrentRun.Exit = &exitRecord
	} else if !errors.Is(err, os.ErrNotExist) {
		return workspace.SessionNode{}, err
	}
	return node, nil
}

func (s DBStore) scanWorkspace(scanner interface{ Scan(dest ...any) error }) (workspace.Workspace, error) {
	var ws workspace.Workspace
	var createdAt, updatedAt any
	if err := scanner.Scan(&ws.Id, &ws.Name, &ws.Path, &createdAt, &updatedAt); err != nil {
		return workspace.Workspace{}, err
	}
	var err error
	ws.CreatedAt, err = dbScanTime(createdAt)
	if err != nil {
		return workspace.Workspace{}, err
	}
	ws.UpdatedAt, err = dbScanTime(updatedAt)
	if err != nil {
		return workspace.Workspace{}, err
	}
	ws.SchemaVersion = workspace.SchemaVersion
	return ws, nil
}

func (s DBStore) loadSessionRow(workspaceId string, sessionId string) (dbSessionRow, error) {
	rows, err := s.listSessionRows(`WHERE sess.id=? AND sess.workspace_id=? AND sess.device_id=? AND sess.deleted_at IS NULL`, strings.TrimSpace(sessionId), strings.TrimSpace(workspaceId), s.deviceId)
	if err != nil {
		return dbSessionRow{}, err
	}
	if len(rows) == 0 {
		return dbSessionRow{}, os.ErrNotExist
	}
	return rows[0], nil
}

func (s DBStore) listSessionViews(where string, args ...any) ([]session.View, error) {
	rows, err := s.listSessionRows(where, args...)
	if err != nil {
		return nil, err
	}
	views := make([]session.View, 0, len(rows))
	for _, row := range rows {
		view := session.View{Session: row.session, State: row.state, CommandText: formatCommand(row.session.Command)}
		if !emptyJSON(row.exitJSON) {
			var exit process.ExitRecord
			if err := json.Unmarshal([]byte(row.exitJSON), &exit); err != nil {
				return nil, fmt.Errorf("decode exit record: %w", err)
			}
			if exit.SchemaVersion != 0 || exit.Reason != "" || !exit.EndedAt.IsZero() {
				code := exit.ExitCode
				view.ExitCode = &code
				view.ExitReason = exit.Reason
			}
		}
		views = append(views, view)
	}
	return views, nil
}

func (s DBStore) listSessionRows(where string, args ...any) ([]dbSessionRow, error) {
	query := `SELECT sess.id,sess.workspace_id,sess.name,sess.launch_cwd,sess.command_json,sess.history_json,sess.current_state,sess.current_state_reason,sess.current_run_id,sess.created_at,sess.updated_at,COALESCE(sr.process_json,'{}'),COALESCE(sr.exit_json,'{}'),COALESCE(sr.updated_at,sess.updated_at) FROM sessions sess LEFT JOIN session_runs sr ON sr.id=sess.current_run_id AND sr.deleted_at IS NULL ` + where + ` ORDER BY sess.sort_order ASC, sess.created_at ASC, sess.name ASC, sess.id ASC`
	rows, err := s.db.QueryContext(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []dbSessionRow
	for rows.Next() {
		var row dbSessionRow
		var commandJSON, historyJSON string
		var stateText string
		var createdAt, updatedAt, runUpdatedAt any
		if err := rows.Scan(&row.session.Id, &row.session.WorkspaceId, &row.session.Name, &row.session.LaunchCwd, &commandJSON, &historyJSON, &stateText, &row.state.Reason, &row.currentRunId, &createdAt, &updatedAt, &row.processJSON, &row.exitJSON, &runUpdatedAt); err != nil {
			return nil, err
		}
		row.session.SchemaVersion = session.SchemaVersion
		if err := json.Unmarshal([]byte(commandJSON), &row.session.Command); err != nil {
			return nil, fmt.Errorf("decode session command: %w", err)
		}
		if err := json.Unmarshal([]byte(historyJSON), &row.session.History); err != nil {
			return nil, fmt.Errorf("decode session history: %w", err)
		}
		var err error
		row.session.CreatedAt, err = dbScanTime(createdAt)
		if err != nil {
			return nil, err
		}
		row.session.UpdatedAt, err = dbScanTime(updatedAt)
		if err != nil {
			return nil, err
		}
		row.runUpdatedAt, err = dbScanTime(runUpdatedAt)
		if err != nil {
			return nil, err
		}
		row.state = session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.State(stateText), Reason: row.state.Reason, UpdatedAt: row.runUpdatedAt}
		if !row.state.State.Valid() {
			return nil, fmt.Errorf("invalid lifecycle state %q", stateText)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s DBStore) activeWorkspaceExistsTx(ctx context.Context, tx *sql.Tx, workspaceId string) (bool, error) {
	var existing string
	err := tx.QueryRowContext(ctx, `SELECT id FROM workspaces WHERE id=? AND device_id=? AND deleted_at IS NULL`, strings.TrimSpace(workspaceId), s.deviceId).Scan(&existing)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return false, err
}

func (s DBStore) activeSessionExistsTx(ctx context.Context, tx *sql.Tx, workspaceId string, sessionId string) (bool, error) {
	var existing string
	err := tx.QueryRowContext(ctx, `SELECT id FROM sessions WHERE id=? AND workspace_id=? AND device_id=? AND deleted_at IS NULL`, strings.TrimSpace(sessionId), strings.TrimSpace(workspaceId), s.deviceId).Scan(&existing)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return false, err
}

func (s DBStore) ensureCurrentRunTx(ctx context.Context, tx *sql.Tx, workspaceId string, sessionId string, appendIfTerminal bool, size *process.TerminalSize, now time.Time) (string, error) {
	workspaceId = strings.TrimSpace(workspaceId)
	sessionId = strings.TrimSpace(sessionId)
	var currentRunId, stateText, commandJSON string
	err := tx.QueryRowContext(ctx, `SELECT current_run_id,current_state,command_json FROM sessions WHERE id=? AND workspace_id=? AND device_id=? AND deleted_at IS NULL`, sessionId, workspaceId, s.deviceId).Scan(&currentRunId, &stateText, &commandJSON)
	if err != nil {
		return "", err
	}
	state := session.State(stateText)
	if currentRunId != "" && (!appendIfTerminal || !session.Terminal(state)) {
		return currentRunId, nil
	}
	sequence, err := s.nextRunSequenceTx(ctx, tx, sessionId)
	if err != nil {
		return "", err
	}
	runId, err := idgen.New()
	if err != nil {
		return "", err
	}
	terminalSizeJSON := `{}`
	if size != nil {
		terminalSizeJSON, err = marshalJSON(size.OrDefault())
		if err != nil {
			return "", fmt.Errorf("marshal terminal size: %w", err)
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO session_runs (id,session_id,workspace_id,device_id,sequence,command_json,terminal_size_json,process_json,exit_json,state,state_reason,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`, runId, sessionId, workspaceId, s.deviceId, sequence, commandJSON, terminalSizeJSON, `{}`, `{}`, string(session.StateRunning), "created", s.storeTime(now), s.storeTime(now))
	if err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, `UPDATE sessions SET current_run_id=?, current_state=?, current_state_reason=?, updated_at=? WHERE id=? AND workspace_id=? AND device_id=? AND deleted_at IS NULL`, runId, string(session.StateRunning), "created", s.storeTime(now), sessionId, workspaceId, s.deviceId)
	if err != nil {
		return "", err
	}
	return runId, nil
}

func (s DBStore) nextWorkspaceOrderTx(ctx context.Context, tx *sql.Tx) (int, error) {
	var order sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT MAX(sort_order)+1 FROM workspaces WHERE device_id=?`, s.deviceId).Scan(&order); err != nil {
		return 0, err
	}
	if !order.Valid {
		return 0, nil
	}
	return int(order.Int64), nil
}

func (s DBStore) nextSessionOrderTx(ctx context.Context, tx *sql.Tx, workspaceId string) (int, error) {
	var order sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT MAX(sort_order)+1 FROM sessions WHERE workspace_id=? AND device_id=?`, workspaceId, s.deviceId).Scan(&order); err != nil {
		return 0, err
	}
	if !order.Valid {
		return 0, nil
	}
	return int(order.Int64), nil
}

func (s DBStore) nextRunSequenceTx(ctx context.Context, tx *sql.Tx, sessionId string) (int, error) {
	var sequence sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT MAX(sequence)+1 FROM session_runs WHERE session_id=? AND device_id=?`, sessionId, s.deviceId).Scan(&sequence); err != nil {
		return 0, err
	}
	if !sequence.Valid {
		return 1, nil
	}
	return int(sequence.Int64), nil
}

func (s DBStore) storeTime(value time.Time) any {
	value = value.UTC()
	if s.driver == "sqlite" {
		return value.Format(time.RFC3339Nano)
	}
	return value
}

func (s DBStore) storeNullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return s.storeTime(value)
}

func marshalJSON(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func emptyJSON(value string) bool {
	value = strings.TrimSpace(value)
	return value == "" || value == "{}" || value == "null"
}

func dbScanTime(value any) (time.Time, error) {
	switch typed := value.(type) {
	case time.Time:
		return typed.UTC(), nil
	case string:
		return dbParseStoredTime(typed)
	case []byte:
		return dbParseStoredTime(string(typed))
	case nil:
		return time.Time{}, nil
	default:
		return time.Time{}, fmt.Errorf("unsupported time value type %T", value)
	}
}

func dbParseStoredTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err == nil {
		return parsed.UTC(), nil
	}
	parsed, err = time.Parse("2006-01-02 15:04:05.999999", value)
	if err == nil {
		return parsed.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("invalid stored time value %q", value)
}
