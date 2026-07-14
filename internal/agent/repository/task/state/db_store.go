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

	"gitee.com/leoninew/TermBridge-go/internal/agent/model/task/process"
	"gitee.com/leoninew/TermBridge-go/internal/agent/model/task/session"
	"gitee.com/leoninew/TermBridge-go/internal/agent/model/task/shortcut"
	"gitee.com/leoninew/TermBridge-go/internal/agent/model/task/workspace"
	"gitee.com/leoninew/TermBridge-go/internal/shared/common/utils/idgen"
)

type DbStore struct {
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

func NewDbStore(db *sql.DB, driver string, root string, deviceId string) DbStore {
	return DbStore{db: db, driver: strings.ToLower(strings.TrimSpace(driver)), root: root, deviceId: strings.TrimSpace(deviceId)}
}

func (s DbStore) WorkspaceRoot() string {
	return filepath.Join(s.root, "workspaces")
}

func (s DbStore) WorkspaceDir(workspaceId string) string {
	return filepath.Join(s.WorkspaceRoot(), workspaceId)
}

func (s DbStore) SessionDir(workspaceId string, sessionId string) string {
	return filepath.Join(s.WorkspaceDir(workspaceId), "sessions", sessionId)
}

func (s DbStore) SaveWorkspace(value workspace.Workspace) error {
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

func (s DbStore) LoadWorkspace(workspaceId string) (workspace.Workspace, error) {
	return s.FindWorkspaceById(workspaceId)
}

func (s DbStore) FindWorkspaceByPath(path string) (workspace.Workspace, error) {
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

func (s DbStore) FindWorkspaceById(workspaceId string) (workspace.Workspace, error) {
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

func (s DbStore) SaveSession(value session.Session) error {
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

func (s DbStore) LoadSession(workspaceId string, sessionId string) (session.Session, error) {
	row, err := s.loadSessionRow(strings.TrimSpace(workspaceId), strings.TrimSpace(sessionId))
	if err != nil {
		return session.Session{}, err
	}
	return row.session, nil
}

func (s DbStore) UpdateSession(workspaceId string, sessionId string, update func(*session.Session) error) (session.Session, error) {
	return s.updateSession(workspaceId, sessionId, false, update)
}

func (s DbStore) UpdateTerminalSession(workspaceId string, sessionId string, update func(*session.Session) error) (session.Session, error) {
	return s.updateSession(workspaceId, sessionId, true, update)
}

func (s DbStore) updateSession(workspaceId string, sessionId string, terminalOnly bool, update func(*session.Session) error) (session.Session, error) {
	if err := s.validate(); err != nil {
		return session.Session{}, err
	}
	workspaceId = strings.TrimSpace(workspaceId)
	sessionId = strings.TrimSpace(sessionId)
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return session.Session{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if exists, err := s.activeWorkspaceExistsTx(ctx, tx, workspaceId); err != nil {
		return session.Session{}, err
	} else if !exists {
		return session.Session{}, os.ErrNotExist
	}
	var value session.Session
	var commandJSON, historyJSON string
	var createdAt, updatedAt any
	row := tx.QueryRowContext(ctx, `SELECT id,workspace_id,name,launch_cwd,command_json,history_json,created_at,updated_at FROM sessions WHERE id=? AND workspace_id=? AND device_id=? AND deleted_at IS NULL`, sessionId, workspaceId, s.deviceId)
	if err := row.Scan(&value.Id, &value.WorkspaceId, &value.Name, &value.LaunchCwd, &commandJSON, &historyJSON, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return session.Session{}, os.ErrNotExist
		}
		return session.Session{}, err
	}
	value.SchemaVersion = session.SchemaVersion
	if err := json.Unmarshal([]byte(commandJSON), &value.Command); err != nil {
		return session.Session{}, fmt.Errorf("decode session command: %w", err)
	}
	if err := json.Unmarshal([]byte(historyJSON), &value.History); err != nil {
		return session.Session{}, fmt.Errorf("decode session history: %w", err)
	}
	value.CreatedAt, err = dbScanTime(createdAt)
	if err != nil {
		return session.Session{}, err
	}
	originalUpdatedAt, err := dbScanTime(updatedAt)
	if err != nil {
		return session.Session{}, err
	}
	value.UpdatedAt = originalUpdatedAt
	if err := update(&value); err != nil {
		return session.Session{}, err
	}
	value.Id = strings.TrimSpace(value.Id)
	value.WorkspaceId = strings.TrimSpace(value.WorkspaceId)
	value.Name = strings.TrimSpace(value.Name)
	value.LaunchCwd = strings.TrimSpace(value.LaunchCwd)
	if value.Id != sessionId || value.WorkspaceId != workspaceId {
		return session.Session{}, errors.New("session update cannot change id or workspace id")
	}
	if value.Name == "" {
		return session.Session{}, errors.New("session name is required")
	}
	commandJSON, err = marshalJSON(value.Command)
	if err != nil {
		return session.Session{}, fmt.Errorf("marshal session command: %w", err)
	}
	historyJSON, err = marshalJSON(value.History)
	if err != nil {
		return session.Session{}, fmt.Errorf("marshal session history: %w", err)
	}
	query := `UPDATE sessions SET name=?, launch_cwd=?, command_json=?, history_json=?, updated_at=? WHERE id=? AND workspace_id=? AND device_id=? AND deleted_at IS NULL AND updated_at=?`
	args := []any{value.Name, value.LaunchCwd, commandJSON, historyJSON, s.storeTime(value.UpdatedAt), value.Id, value.WorkspaceId, s.deviceId, s.storeTime(originalUpdatedAt)}
	if terminalOnly {
		query += ` AND current_state IN (?,?)`
		args = append(args, string(session.StateStopped), string(session.StateFailed))
	}
	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return session.Session{}, err
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		if terminalOnly {
			return session.Session{}, errors.New("session is no longer terminal")
		}
		return session.Session{}, errors.New("session was updated concurrently")
	}
	return value, tx.Commit()
}

func (s DbStore) CreateShortcut(value shortcut.Shortcut) (shortcut.Shortcut, error) {
	if err := s.validate(); err != nil {
		return shortcut.Shortcut{}, err
	}
	if err := value.Normalize(); err != nil {
		return shortcut.Shortcut{}, err
	}
	if value.Id == "" {
		id, err := idgen.New()
		if err != nil {
			return shortcut.Shortcut{}, err
		}
		value.Id = id
	}
	now := time.Now().UTC()
	if value.CreatedAt.IsZero() {
		value.CreatedAt = now
	}
	if value.UpdatedAt.IsZero() {
		value.UpdatedAt = value.CreatedAt
	}
	ctx := context.Background()
	var txOptions *sql.TxOptions
	if s.driver == "mysql" {
		txOptions = &sql.TxOptions{Isolation: sql.LevelSerializable}
	}
	tx, err := s.db.BeginTx(ctx, txOptions)
	if err != nil {
		return shortcut.Shortcut{}, err
	}
	defer func() { _ = tx.Rollback() }()
	sortOrder, err := s.nextShortcutOrderTx(ctx, tx)
	if err != nil {
		return shortcut.Shortcut{}, err
	}
	tagsJSON, err := marshalJSON(value.Tags)
	if err != nil {
		return shortcut.Shortcut{}, fmt.Errorf("marshal shortcut tags: %w", err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO shortcuts (id,device_id,name,command,description,icon,enabled,tags_json,last_used_at,sort_order,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, value.Id, s.deviceId, value.Name, value.Command, value.Description, value.Icon, shortcutEnabled(value.Enabled), tagsJSON, s.storeNullableTime(value.LastUsedAt), sortOrder, s.storeTime(value.CreatedAt), s.storeTime(value.UpdatedAt))
	if err != nil {
		return shortcut.Shortcut{}, err
	}
	if err := tx.Commit(); err != nil {
		return shortcut.Shortcut{}, err
	}
	value.SchemaVersion = shortcut.SchemaVersion
	return value, nil
}

func (s DbStore) ListShortcuts() ([]shortcut.Shortcut, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(context.Background(), `SELECT id,name,command,description,icon,enabled,tags_json,last_used_at,created_at,updated_at FROM shortcuts WHERE device_id=? ORDER BY sort_order ASC, id ASC`, s.deviceId)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	values := make([]shortcut.Shortcut, 0)
	for rows.Next() {
		value, err := scanShortcut(rows)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func (s DbStore) UpdateShortcutOrder(shortcutIds []string) ([]shortcut.Shortcut, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	values, err := s.listShortcutsTx(ctx, tx)
	if err != nil {
		return nil, err
	}
	ordered, err := completeShortcutIds(shortcutIds, values)
	if err != nil {
		return nil, err
	}
	for index, id := range ordered {
		result, err := tx.ExecContext(ctx, `UPDATE shortcuts SET sort_order=? WHERE id=? AND device_id=?`, index, id, s.deviceId)
		if err != nil {
			return nil, err
		}
		if affected, err := result.RowsAffected(); err == nil && affected != 1 {
			return nil, os.ErrNotExist
		}
	}
	updated, err := s.listShortcutsTx(ctx, tx)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return updated, nil
}

func (s DbStore) listShortcutsTx(ctx context.Context, tx *sql.Tx) ([]shortcut.Shortcut, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id,name,command,description,icon,enabled,tags_json,last_used_at,created_at,updated_at FROM shortcuts WHERE device_id=? ORDER BY sort_order ASC, id ASC`, s.deviceId)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	values := make([]shortcut.Shortcut, 0)
	for rows.Next() {
		value, err := scanShortcut(rows)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func completeShortcutIds(ids []string, values []shortcut.Shortcut) ([]string, error) {
	if len(ids) != len(values) {
		return nil, fmt.Errorf("shortcut_ids must include every shortcut: %w", os.ErrInvalid)
	}
	byId := make(map[string]bool, len(values))
	for _, value := range values {
		byId[value.Id] = true
	}
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if seen[id] {
			return nil, fmt.Errorf("duplicate shortcut_id %q: %w", id, os.ErrInvalid)
		}
		if !byId[id] {
			return nil, fmt.Errorf("shortcut_id %q: %w", id, os.ErrNotExist)
		}
		seen[id] = true
	}
	return append([]string(nil), ids...), nil
}

func (s DbStore) UpdateShortcut(shortcutId string, update func(*shortcut.Shortcut) error) (shortcut.Shortcut, error) {
	if err := s.validate(); err != nil {
		return shortcut.Shortcut{}, err
	}
	shortcutId = strings.TrimSpace(shortcutId)
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return shortcut.Shortcut{}, err
	}
	defer func() { _ = tx.Rollback() }()
	value, err := scanShortcut(tx.QueryRowContext(ctx, `SELECT id,name,command,description,icon,enabled,tags_json,last_used_at,created_at,updated_at FROM shortcuts WHERE id=? AND device_id=?`, shortcutId, s.deviceId))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return shortcut.Shortcut{}, os.ErrNotExist
		}
		return shortcut.Shortcut{}, err
	}
	originalUpdatedAt := value.UpdatedAt
	if err := update(&value); err != nil {
		return shortcut.Shortcut{}, err
	}
	if value.Id != shortcutId {
		return shortcut.Shortcut{}, errors.New("shortcut update cannot change id")
	}
	if err := value.Normalize(); err != nil {
		return shortcut.Shortcut{}, err
	}
	if value.UpdatedAt.IsZero() || value.UpdatedAt.Equal(originalUpdatedAt) {
		value.UpdatedAt = time.Now().UTC()
	}
	tagsJSON, err := marshalJSON(value.Tags)
	if err != nil {
		return shortcut.Shortcut{}, fmt.Errorf("marshal shortcut tags: %w", err)
	}
	result, err := tx.ExecContext(ctx, `UPDATE shortcuts SET name=?, command=?, description=?, icon=?, enabled=?, tags_json=?, updated_at=? WHERE id=? AND device_id=? AND updated_at=?`, value.Name, value.Command, value.Description, value.Icon, shortcutEnabled(value.Enabled), tagsJSON, s.storeTime(value.UpdatedAt), value.Id, s.deviceId, s.storeTime(originalUpdatedAt))
	if err != nil {
		return shortcut.Shortcut{}, err
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return shortcut.Shortcut{}, errors.New("shortcut was updated concurrently")
	}
	if err := tx.Commit(); err != nil {
		return shortcut.Shortcut{}, err
	}
	return value, nil
}

func (s DbStore) DeleteShortcut(shortcutId string) error {
	if err := s.validate(); err != nil {
		return err
	}
	result, err := s.db.ExecContext(context.Background(), `DELETE FROM shortcuts WHERE id=? AND device_id=?`, strings.TrimSpace(shortcutId), s.deviceId)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return os.ErrNotExist
	}
	return nil
}

func shortcutEnabled(value *bool) bool {
	return value == nil || *value
}

func scanShortcut(scanner interface{ Scan(dest ...any) error }) (shortcut.Shortcut, error) {
	var value shortcut.Shortcut
	var description, icon sql.NullString
	var enabled bool
	var tagsJSON string
	var lastUsedAt, createdAt, updatedAt any
	if err := scanner.Scan(&value.Id, &value.Name, &value.Command, &description, &icon, &enabled, &tagsJSON, &lastUsedAt, &createdAt, &updatedAt); err != nil {
		return shortcut.Shortcut{}, err
	}
	if description.Valid {
		value.Description = &description.String
	}
	if icon.Valid {
		value.Icon = &icon.String
	}
	value.Enabled = &enabled
	if err := json.Unmarshal([]byte(tagsJSON), &value.Tags); err != nil {
		return shortcut.Shortcut{}, fmt.Errorf("decode shortcut tags: %w", err)
	}
	if err := value.Normalize(); err != nil {
		return shortcut.Shortcut{}, err
	}
	var err error
	value.LastUsedAt, err = dbScanTime(lastUsedAt)
	if err != nil {
		return shortcut.Shortcut{}, err
	}
	value.CreatedAt, err = dbScanTime(createdAt)
	if err != nil {
		return shortcut.Shortcut{}, err
	}
	value.UpdatedAt, err = dbScanTime(updatedAt)
	if err != nil {
		return shortcut.Shortcut{}, err
	}
	value.SchemaVersion = shortcut.SchemaVersion
	return value, nil
}

func (s DbStore) DeleteSession(workspaceId string, sessionId string) error {
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

func (s DbStore) DeleteWorkspace(workspaceId string) error {
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

func (s DbStore) SaveState(workspaceId string, sessionId string, value session.StateRecord) error {
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
	_, err = tx.ExecContext(ctx, `UPDATE sessions SET current_state=?, current_state_reason=?, updated_at=? WHERE id=? AND workspace_id=? AND device_id=? AND deleted_at IS NULL`, string(value.State), value.Reason, s.storeTime(value.UpdatedAt), strings.TrimSpace(sessionId), strings.TrimSpace(workspaceId), s.deviceId)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s DbStore) LoadState(workspaceId string, sessionId string) (session.StateRecord, error) {
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

func (s DbStore) SaveProcess(workspaceId string, sessionId string, value process.Record) error {
	return s.SaveProcessState(workspaceId, sessionId, value, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateRunning, Reason: "process_started", UpdatedAt: value.StartedAt})
}

func (s DbStore) SaveProcessState(workspaceId string, sessionId string, value process.Record, stateRecord session.StateRecord) error {
	if !stateRecord.State.Valid() {
		return fmt.Errorf("invalid lifecycle state %q", stateRecord.State)
	}
	updatedAt := stateRecord.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = value.StartedAt
	}
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	stateRecord.UpdatedAt = updatedAt
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
	workspaceId = strings.TrimSpace(workspaceId)
	sessionId = strings.TrimSpace(sessionId)
	runId, err := s.ensureCurrentRunTx(ctx, tx, workspaceId, sessionId, stateRecord.State == session.StateRunning, nil, updatedAt)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE session_runs SET process_json=?, state=?, state_reason=?, started_at=?, updated_at=? WHERE id=? AND deleted_at IS NULL`, processJSON, string(stateRecord.State), stateRecord.Reason, s.storeNullableTime(value.StartedAt), s.storeTime(updatedAt), runId)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE sessions SET current_state=?, current_state_reason=?, updated_at=? WHERE id=? AND workspace_id=? AND device_id=? AND deleted_at IS NULL`, string(stateRecord.State), stateRecord.Reason, s.storeTime(updatedAt), sessionId, workspaceId, s.deviceId)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s DbStore) LoadProcess(workspaceId string, sessionId string) (process.Record, error) {
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

func (s DbStore) SaveExit(workspaceId string, sessionId string, value process.ExitRecord) error {
	return s.SaveExitState(workspaceId, sessionId, value, session.StateRecord{SchemaVersion: session.SchemaVersion, State: session.StateStopped, Reason: value.Reason, UpdatedAt: value.EndedAt})
}

func (s DbStore) SaveExitState(workspaceId string, sessionId string, value process.ExitRecord, stateRecord session.StateRecord) error {
	return s.saveSessionExitState(workspaceId, sessionId, nil, value, stateRecord)
}

func (s DbStore) SaveSessionExitState(value session.Session, exit process.ExitRecord, stateRecord session.StateRecord) error {
	return s.saveSessionExitState(value.WorkspaceId, value.Id, &value, exit, stateRecord)
}

func (s DbStore) saveSessionExitState(workspaceId string, sessionId string, sessionValue *session.Session, exit process.ExitRecord, stateRecord session.StateRecord) error {
	if !stateRecord.State.Valid() {
		return fmt.Errorf("invalid lifecycle state %q", stateRecord.State)
	}
	updatedAt := stateRecord.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = exit.EndedAt
	}
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	stateRecord.UpdatedAt = updatedAt
	exitJSON, err := marshalJSON(exit)
	if err != nil {
		return fmt.Errorf("marshal exit record: %w", err)
	}
	var commandJSON string
	var historyJSON string
	if sessionValue != nil {
		sessionValue.Id = strings.TrimSpace(sessionValue.Id)
		sessionValue.WorkspaceId = strings.TrimSpace(sessionValue.WorkspaceId)
		sessionValue.Name = strings.TrimSpace(sessionValue.Name)
		sessionValue.LaunchCwd = strings.TrimSpace(sessionValue.LaunchCwd)
		if sessionValue.Id == "" || sessionValue.WorkspaceId == "" || sessionValue.Name == "" {
			return errors.New("session id, workspace id and name are required")
		}
		commandJSON, err = marshalJSON(sessionValue.Command)
		if err != nil {
			return fmt.Errorf("marshal session command: %w", err)
		}
		historyJSON, err = marshalJSON(sessionValue.History)
		if err != nil {
			return fmt.Errorf("marshal session history: %w", err)
		}
	}
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if sessionValue != nil {
		result, err := tx.ExecContext(ctx, `UPDATE sessions SET name=?, launch_cwd=?, command_json=?, history_json=?, updated_at=? WHERE id=? AND workspace_id=? AND device_id=? AND deleted_at IS NULL`, sessionValue.Name, sessionValue.LaunchCwd, commandJSON, historyJSON, s.storeTime(updatedAt), sessionValue.Id, sessionValue.WorkspaceId, s.deviceId)
		if err != nil {
			return err
		}
		if affected, err := result.RowsAffected(); err == nil && affected == 0 {
			return os.ErrNotExist
		}
	}
	workspaceId = strings.TrimSpace(workspaceId)
	sessionId = strings.TrimSpace(sessionId)
	runId, err := s.ensureCurrentRunTx(ctx, tx, workspaceId, sessionId, false, nil, updatedAt)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE session_runs SET exit_json=?, state=?, state_reason=?, ended_at=?, updated_at=? WHERE id=? AND deleted_at IS NULL`, exitJSON, string(stateRecord.State), stateRecord.Reason, s.storeNullableTime(exit.EndedAt), s.storeTime(updatedAt), runId)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE sessions SET current_state=?, current_state_reason=?, updated_at=? WHERE id=? AND workspace_id=? AND device_id=? AND deleted_at IS NULL`, string(stateRecord.State), stateRecord.Reason, s.storeTime(updatedAt), strings.TrimSpace(sessionId), strings.TrimSpace(workspaceId), s.deviceId)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s DbStore) LoadExit(workspaceId string, sessionId string) (process.ExitRecord, error) {
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

func (s DbStore) BeginSessionRun(workspaceId string, sessionId string, size process.TerminalSize) error {
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

func (s DbStore) HistoryPath(workspaceId string, sessionId string) string {
	return filepath.Join(s.root, "history", sessionId+".log")
}

func (s DbStore) ListWorkspaces() ([]workspace.Workspace, []Warning, error) {
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

func (s DbStore) ListSessionsByWorkspaceId(workspaceId string) ([]session.View, []Warning, error) {
	if _, err := s.FindWorkspaceById(workspaceId); err != nil {
		return nil, nil, err
	}
	views, err := s.listSessionViews(`WHERE sess.workspace_id=? AND sess.device_id=? AND sess.deleted_at IS NULL`, strings.TrimSpace(workspaceId), s.deviceId)
	return views, nil, err
}

func (s DbStore) ListSessions() ([]session.View, []Warning, error) {
	views, err := s.listSessionViews(`WHERE sess.device_id=? AND sess.deleted_at IS NULL`, s.deviceId)
	return views, nil, err
}

func (s DbStore) UpdateWorkspaceOrder(workspaceIds []string, now time.Time) ([]workspace.Workspace, error) {
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

func (s DbStore) UpdateSessionOrder(workspaceId string, sessionIds []string, now time.Time) ([]session.View, []Warning, error) {
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

func (s DbStore) validate() error {
	if s.db == nil {
		return errors.New("database is required")
	}
	if strings.TrimSpace(s.deviceId) == "" {
		return errors.New("device id is required")
	}
	return nil
}

func (s DbStore) populateWorkspace(ws workspace.Workspace) (workspace.Workspace, error) {
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

func (s DbStore) sessionNodeFromView(view session.View) (workspace.SessionNode, error) {
	state := workspaceStateFromSession(view.State)
	node := workspace.SessionNode{
		Id:        view.Session.Id,
		Name:      view.Session.Name,
		LaunchCwd: view.Session.LaunchCwd,
		Command: workspace.CommandRecord{
			Command:              view.Session.Command.Command,
			EnvStrategy:          view.Session.Command.EnvStrategy,
			EnvCount:             view.Session.Command.EnvCount,
			Source:               string(view.Session.Command.Source),
			ShortcutIdSnapshot:   view.Session.Command.ShortcutIdSnapshot,
			ShortcutNameSnapshot: view.Session.Command.ShortcutNameSnapshot,
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

func (s DbStore) scanWorkspace(scanner interface{ Scan(dest ...any) error }) (workspace.Workspace, error) {
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

func (s DbStore) loadSessionRow(workspaceId string, sessionId string) (dbSessionRow, error) {
	rows, err := s.listSessionRows(`WHERE sess.id=? AND sess.workspace_id=? AND sess.device_id=? AND sess.deleted_at IS NULL`, strings.TrimSpace(sessionId), strings.TrimSpace(workspaceId), s.deviceId)
	if err != nil {
		return dbSessionRow{}, err
	}
	if len(rows) == 0 {
		return dbSessionRow{}, os.ErrNotExist
	}
	return rows[0], nil
}

func (s DbStore) listSessionViews(where string, args ...any) ([]session.View, error) {
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

func (s DbStore) listSessionRows(where string, args ...any) ([]dbSessionRow, error) {
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

func (s DbStore) activeWorkspaceExistsTx(ctx context.Context, tx *sql.Tx, workspaceId string) (bool, error) {
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

func (s DbStore) activeSessionExistsTx(ctx context.Context, tx *sql.Tx, workspaceId string, sessionId string) (bool, error) {
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

func (s DbStore) ensureCurrentRunTx(ctx context.Context, tx *sql.Tx, workspaceId string, sessionId string, appendIfTerminal bool, size *process.TerminalSize, now time.Time) (string, error) {
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

func (s DbStore) nextShortcutOrderTx(ctx context.Context, tx *sql.Tx) (int, error) {
	query := `SELECT sort_order FROM shortcuts WHERE device_id=? ORDER BY sort_order DESC LIMIT 1`
	if s.driver == "mysql" {
		query += ` FOR UPDATE`
	}
	var order int
	err := tx.QueryRowContext(ctx, query, s.deviceId).Scan(&order)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return order + 1, nil
}

func (s DbStore) nextWorkspaceOrderTx(ctx context.Context, tx *sql.Tx) (int, error) {
	var order sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT MAX(sort_order)+1 FROM workspaces WHERE device_id=?`, s.deviceId).Scan(&order); err != nil {
		return 0, err
	}
	if !order.Valid {
		return 0, nil
	}
	return int(order.Int64), nil
}

func (s DbStore) nextSessionOrderTx(ctx context.Context, tx *sql.Tx, workspaceId string) (int, error) {
	var order sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT MAX(sort_order)+1 FROM sessions WHERE workspace_id=? AND device_id=?`, workspaceId, s.deviceId).Scan(&order); err != nil {
		return 0, err
	}
	if !order.Valid {
		return 0, nil
	}
	return int(order.Int64), nil
}

func (s DbStore) nextRunSequenceTx(ctx context.Context, tx *sql.Tx, sessionId string) (int, error) {
	var sequence sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT MAX(sequence)+1 FROM session_runs WHERE session_id=? AND device_id=?`, sessionId, s.deviceId).Scan(&sequence); err != nil {
		return 0, err
	}
	if !sequence.Valid {
		return 1, nil
	}
	return int(sequence.Int64), nil
}

func (s DbStore) storeTime(value time.Time) any {
	value = value.UTC()
	if s.driver == "sqlite" {
		return value.Format(time.RFC3339Nano)
	}
	return value
}

func (s DbStore) storeNullableTime(value time.Time) any {
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
