package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"termbridge-go/internal/process"
	"termbridge-go/internal/session"
	"termbridge-go/internal/workspace"
)

const StateDirName = ".termbridge"

type Store struct {
	Root string
}

type Warning struct {
	Path string
	Err  error
}

func NewStore(root string) Store {
	return Store{Root: root}
}

func RootForCwd(cwd string, stateDir string) string {
	if stateDir == "" {
		stateDir = StateDirName
	}
	if filepath.IsAbs(stateDir) {
		return filepath.Clean(stateDir)
	}
	return filepath.Join(cwd, stateDir)
}

func (s Store) WorkspaceDir(key string) string {
	return filepath.Join(s.Root, key)
}

func (s Store) SessionDir(workspaceKey string, sessionId string) string {
	return filepath.Join(s.WorkspaceDir(workspaceKey), sessionId)
}

func (s Store) SaveWorkspace(value workspace.Workspace) error {
	path := filepath.Join(s.WorkspaceDir(value.Key), "workspace.json")
	return writeJSON(path, value)
}

func (s Store) LoadWorkspace(key string) (workspace.Workspace, error) {
	var value workspace.Workspace
	err := readJSON(filepath.Join(s.WorkspaceDir(key), "workspace.json"), &value)
	return value, err
}

func (s Store) FindWorkspaceById(workspaceId string) (workspace.Workspace, error) {
	workspaces, _, err := s.ListWorkspaces()
	if err != nil {
		return workspace.Workspace{}, err
	}
	for _, value := range workspaces {
		if value.ID == workspaceId {
			return value, nil
		}
	}
	return workspace.Workspace{}, os.ErrNotExist
}

func (s Store) SaveSession(value session.Session) error {
	ws, err := s.LoadWorkspace(value.WorkspaceKey)
	if err != nil {
		return err
	}
	ws.UpdatedAt = value.UpdatedAt
	upsertSessionNode(&ws, sessionNodeFromSession(value))
	if err := s.SaveWorkspace(ws); err != nil {
		return err
	}
	path := filepath.Join(s.SessionDir(value.WorkspaceKey, value.ID), "session.json")
	return writeJSON(path, value)
}

func (s Store) LoadSession(workspaceKey string, sessionId string) (session.Session, error) {
	ws, err := s.LoadWorkspace(workspaceKey)
	if err != nil {
		return session.Session{}, err
	}
	for _, child := range ws.Children {
		if child.ID == sessionId {
			return sessionFromWorkspaceNode(ws, child), nil
		}
	}
	return session.Session{}, os.ErrNotExist
}

func (s Store) UpdateSession(workspaceKey string, sessionId string, update func(*session.Session) error) (session.Session, error) {
	value, err := s.LoadSession(workspaceKey, sessionId)
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

func (s Store) DeleteSession(workspaceKey string, sessionId string) error {
	ws, err := s.LoadWorkspace(workspaceKey)
	if err != nil {
		return err
	}
	ws.UpdatedAt = time.Now().UTC()
	removeSessionNode(&ws, sessionId)
	if err := s.SaveWorkspace(ws); err != nil {
		return err
	}
	return os.RemoveAll(s.SessionDir(workspaceKey, sessionId))
}

func (s Store) DeleteWorkspace(workspaceKey string) error {
	return os.RemoveAll(s.WorkspaceDir(workspaceKey))
}

func (s Store) SaveState(workspaceKey string, sessionId string, value session.StateRecord) error {
	path := filepath.Join(s.SessionDir(workspaceKey, sessionId), "state.json")
	return writeJSON(path, value)
}

func (s Store) LoadState(workspaceKey string, sessionId string) (session.StateRecord, error) {
	var value session.StateRecord
	err := readJSON(filepath.Join(s.SessionDir(workspaceKey, sessionId), "state.json"), &value)
	return value, err
}

func (s Store) SaveProcess(workspaceKey string, sessionId string, value process.Record) error {
	path := filepath.Join(s.SessionDir(workspaceKey, sessionId), "process.json")
	return writeJSON(path, value)
}

func (s Store) LoadProcess(workspaceKey string, sessionId string) (process.Record, error) {
	var value process.Record
	err := readJSON(filepath.Join(s.SessionDir(workspaceKey, sessionId), "process.json"), &value)
	return value, err
}

func (s Store) SaveExit(workspaceKey string, sessionId string, value process.ExitRecord) error {
	path := filepath.Join(s.SessionDir(workspaceKey, sessionId), "exit.json")
	return writeJSON(path, value)
}

func (s Store) LoadExit(workspaceKey string, sessionId string) (process.ExitRecord, error) {
	var value process.ExitRecord
	err := readJSON(filepath.Join(s.SessionDir(workspaceKey, sessionId), "exit.json"), &value)
	return value, err
}

func (s Store) HistoryPath(workspaceKey string, sessionId string) string {
	return filepath.Join(s.SessionDir(workspaceKey, sessionId), "history.log")
}

func (s Store) ListWorkspaces() ([]workspace.Workspace, []Warning, error) {
	entries, err := os.ReadDir(s.Root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var values []workspace.Workspace
	var warnings []Warning
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(s.Root, entry.Name(), "workspace.json")
		var value workspace.Workspace
		if err := readJSON(path, &value); err != nil {
			warnings = append(warnings, Warning{Path: path, Err: err})
			continue
		}
		values = append(values, value)
	}
	sortWorkspaces(values)
	return values, warnings, nil
}

func (s Store) ListSessionsByWorkspaceId(workspaceId string) ([]session.View, []Warning, error) {
	ws, err := s.FindWorkspaceById(workspaceId)
	if err != nil {
		return nil, nil, err
	}
	return s.listSessionsInWorkspace(ws)
}

func (s Store) ListSessions() ([]session.View, []Warning, error) {
	workspaces, warnings, err := s.ListWorkspaces()
	if err != nil {
		return nil, warnings, err
	}
	var views []session.View
	for _, ws := range workspaces {
		workspaceViews, workspaceWarnings, err := s.listSessionsInWorkspace(ws)
		warnings = append(warnings, workspaceWarnings...)
		if err != nil {
			warnings = append(warnings, Warning{Path: s.WorkspaceDir(ws.Key), Err: err})
			continue
		}
		views = append(views, workspaceViews...)
	}
	return views, warnings, nil
}

func (s Store) UpdateWorkspaceOrder(workspaceIds []string, now time.Time) ([]workspace.Workspace, error) {
	workspaces, _, err := s.ListWorkspaces()
	if err != nil {
		return nil, err
	}
	byID := make(map[string]workspace.Workspace, len(workspaces))
	for _, ws := range workspaces {
		byID[ws.ID] = ws
	}
	seen := map[string]bool{}
	for _, id := range workspaceIds {
		if seen[id] {
			return nil, fmt.Errorf("duplicate workspace_id %q", id)
		}
		seen[id] = true
		ws, ok := byID[id]
		if !ok {
			return nil, os.ErrNotExist
		}
		ws.SortOrder = len(seen)
		ws.UpdatedAt = now.UTC()
		if err := s.SaveWorkspace(ws); err != nil {
			return nil, err
		}
		byID[id] = ws
	}
	updated, _, err := s.ListWorkspaces()
	return updated, err
}

func (s Store) listSessionsInWorkspace(ws workspace.Workspace) ([]session.View, []Warning, error) {
	var views []session.View
	var warnings []Warning
	for _, child := range ws.Children {
		sess := sessionFromWorkspaceNode(ws, child)
		stateRecord, err := s.LoadState(ws.Key, child.ID)
		if err != nil {
			warnings = append(warnings, Warning{Path: filepath.Join(s.SessionDir(ws.Key, child.ID), "state.json"), Err: err})
		}
		view := session.View{Session: sess, State: stateRecord, CommandText: formatCommand(sess.Command)}
		if exit, err := s.LoadExit(ws.Key, child.ID); err == nil {
			code := exit.ExitCode
			view.ExitCode = &code
			view.ExitReason = exit.Reason
		} else if !errors.Is(err, os.ErrNotExist) {
			warnings = append(warnings, Warning{Path: filepath.Join(s.SessionDir(ws.Key, child.ID), "exit.json"), Err: err})
		}
		views = append(views, view)
	}
	return views, warnings, nil
}

func sessionNodeFromSession(value session.Session) workspace.SessionNode {
	return workspace.SessionNode{
		ID:        value.ID,
		Name:      value.Name,
		LaunchCwd: value.LaunchCwd,
		Command: workspace.CommandRecord{
			Executable:  value.Command.Executable,
			Command:     value.Command.Command,
			Args:        append([]string(nil), value.Command.Args...),
			EnvStrategy: value.Command.EnvStrategy,
			EnvCount:    value.Command.EnvCount,
		},
		LogPath:   value.LogPath,
		CreatedAt: value.CreatedAt,
		UpdatedAt: value.UpdatedAt,
	}
}

func sessionFromWorkspaceNode(ws workspace.Workspace, child workspace.SessionNode) session.Session {
	return session.Session{
		SchemaVersion: session.SchemaVersion,
		ID:            child.ID,
		Name:          child.Name,
		WorkspaceId:   ws.ID,
		WorkspaceKey:  ws.Key,
		LaunchCwd:     child.LaunchCwd,
		Command: session.CommandRecord{
			Executable:  child.Command.Executable,
			Command:     child.Command.Command,
			Args:        append([]string(nil), child.Command.Args...),
			EnvStrategy: child.Command.EnvStrategy,
			EnvCount:    child.Command.EnvCount,
		},
		LogPath:   child.LogPath,
		CreatedAt: child.CreatedAt,
		UpdatedAt: child.UpdatedAt,
	}
}

func upsertSessionNode(ws *workspace.Workspace, child workspace.SessionNode) {
	for i := range ws.Children {
		if ws.Children[i].ID == child.ID {
			ws.Children[i] = child
			return
		}
	}
	ws.Children = append(ws.Children, child)
}

func removeSessionNode(ws *workspace.Workspace, sessionId string) {
	for i := range ws.Children {
		if ws.Children[i].ID == sessionId {
			ws.Children = append(ws.Children[:i], ws.Children[i+1:]...)
			return
		}
	}
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

func sortWorkspaces(values []workspace.Workspace) {
	sort.SliceStable(values, func(i, j int) bool {
		left := values[i]
		right := values[j]
		if left.SortOrder != right.SortOrder {
			if left.SortOrder == 0 {
				return false
			}
			if right.SortOrder == 0 {
				return true
			}
			return left.SortOrder < right.SortOrder
		}
		if !left.CreatedAt.Equal(right.CreatedAt) {
			return left.CreatedAt.Before(right.CreatedAt)
		}
		if left.Name != right.Name {
			return left.Name < right.Name
		}
		return left.ID < right.ID
	})
}

func readJSON(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, value); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func formatCommand(command session.CommandRecord) string {
	if len(command.Args) == 0 {
		return command.Command
	}
	return command.Command + " " + joinArgs(command.Args)
}

func joinArgs(args []string) string {
	out := ""
	for i, arg := range args {
		if i > 0 {
			out += " "
		}
		out += arg
	}
	return out
}
