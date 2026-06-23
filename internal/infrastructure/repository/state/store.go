package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"termbridge-go/internal/domain/process"
	"termbridge-go/internal/domain/session"
	"termbridge-go/internal/domain/workspace"
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

func NewDeviceStore(root string, deviceId string) Store {
	return Store{Root: filepath.Join(root, "devices", deviceId)}
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

func (s Store) WorkspaceRoot() string {
	return filepath.Join(s.Root, "workspaces")
}

func (s Store) WorkspaceDir(workspaceId string) string {
	return filepath.Join(s.WorkspaceRoot(), workspaceId)
}

func (s Store) SessionDir(workspaceId string, sessionId string) string {
	return filepath.Join(s.WorkspaceDir(workspaceId), "sessions", sessionId)
}

func (s Store) SaveWorkspace(value workspace.Workspace) error {
	path := filepath.Join(s.WorkspaceDir(value.Id), "workspace.json")
	return writeJSON(path, value)
}

func (s Store) LoadWorkspace(workspaceId string) (workspace.Workspace, error) {
	var value workspace.Workspace
	err := readJSON(filepath.Join(s.WorkspaceDir(workspaceId), "workspace.json"), &value)
	return value, err
}

func (s Store) FindWorkspaceByPath(path string) (workspace.Workspace, error) {
	workspaces, _, err := s.ListWorkspaces()
	if err != nil {
		return workspace.Workspace{}, err
	}
	for _, value := range workspaces {
		if value.Path == path {
			return value, nil
		}
	}
	return workspace.Workspace{}, os.ErrNotExist
}

func (s Store) FindWorkspaceById(workspaceId string) (workspace.Workspace, error) {
	return s.LoadWorkspace(workspaceId)
}

func (s Store) SaveSession(value session.Session) error {
	ws, err := s.LoadWorkspace(value.WorkspaceId)
	if err != nil {
		return err
	}
	ws.UpdatedAt = value.UpdatedAt
	upsertSessionNode(&ws, sessionNodeFromSession(value))
	return s.SaveWorkspace(ws)
}

func (s Store) LoadSession(workspaceId string, sessionId string) (session.Session, error) {
	ws, err := s.LoadWorkspace(workspaceId)
	if err != nil {
		return session.Session{}, err
	}
	for _, child := range ws.Children {
		if child.Id == sessionId {
			return sessionFromWorkspaceNode(ws, child), nil
		}
	}
	return session.Session{}, os.ErrNotExist
}

func (s Store) UpdateSession(workspaceId string, sessionId string, update func(*session.Session) error) (session.Session, error) {
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

func (s Store) DeleteSession(workspaceId string, sessionId string) error {
	ws, err := s.LoadWorkspace(workspaceId)
	if err != nil {
		return err
	}
	ws.UpdatedAt = time.Now().UTC()
	removeSessionNode(&ws, sessionId)
	if err := s.SaveWorkspace(ws); err != nil {
		return err
	}
	return os.RemoveAll(s.SessionDir(workspaceId, sessionId))
}

func (s Store) DeleteWorkspace(workspaceId string) error {
	return os.RemoveAll(s.WorkspaceDir(workspaceId))
}

func (s Store) SaveState(workspaceId string, sessionId string, value session.StateRecord) error {
	return s.updateSessionNode(workspaceId, sessionId, func(node *workspace.SessionNode) error {
		node.State = workspaceStateFromSession(value)
		node.UpdatedAt = value.UpdatedAt
		return nil
	})
}

func (s Store) LoadState(workspaceId string, sessionId string) (session.StateRecord, error) {
	node, err := s.loadSessionNode(workspaceId, sessionId)
	if err != nil {
		return session.StateRecord{}, err
	}
	return sessionStateFromWorkspace(node.State)
}

func (s Store) SaveProcess(workspaceId string, sessionId string, value process.Record) error {
	return s.updateSessionNode(workspaceId, sessionId, func(node *workspace.SessionNode) error {
		record := workspaceProcessFromProcess(value)
		node.CurrentRun.Process = &record
		if !value.StartedAt.IsZero() {
			node.UpdatedAt = value.StartedAt
		} else {
			node.UpdatedAt = time.Now().UTC()
		}
		return nil
	})
}

func (s Store) LoadProcess(workspaceId string, sessionId string) (process.Record, error) {
	node, err := s.loadSessionNode(workspaceId, sessionId)
	if err != nil {
		return process.Record{}, err
	}
	if node.CurrentRun.Process == nil {
		return process.Record{}, os.ErrNotExist
	}
	return processFromWorkspaceProcess(*node.CurrentRun.Process), nil
}

func (s Store) SaveExit(workspaceId string, sessionId string, value process.ExitRecord) error {
	return s.updateSessionNode(workspaceId, sessionId, func(node *workspace.SessionNode) error {
		record := workspaceExitFromProcess(value)
		node.CurrentRun.Exit = &record
		if !value.EndedAt.IsZero() {
			node.UpdatedAt = value.EndedAt
		} else {
			node.UpdatedAt = time.Now().UTC()
		}
		return nil
	})
}

func (s Store) LoadExit(workspaceId string, sessionId string) (process.ExitRecord, error) {
	node, err := s.loadSessionNode(workspaceId, sessionId)
	if err != nil {
		return process.ExitRecord{}, err
	}
	if node.CurrentRun.Exit == nil {
		return process.ExitRecord{}, os.ErrNotExist
	}
	return processExitFromWorkspaceExit(*node.CurrentRun.Exit), nil
}

func (s Store) ArchiveCurrentRun(workspaceId string, sessionId string, archiveId string, historyPath string, archivedAt time.Time) error {
	if strings.TrimSpace(archiveId) == "" {
		return fmt.Errorf("archive_id is required")
	}
	return s.updateSessionNode(workspaceId, sessionId, func(node *workspace.SessionNode) error {
		node.ArchivedRuns = append(node.ArchivedRuns, workspace.ArchivedRun{
			ArchiveId:   archiveId,
			HistoryPath: historyPath,
			State:       node.State,
			Process:     cloneWorkspaceProcess(node.CurrentRun.Process),
			Exit:        cloneWorkspaceExit(node.CurrentRun.Exit),
			ArchivedAt:  archivedAt,
		})
		node.CurrentRun = workspace.RunRecord{}
		node.History.Truncated = false
		node.UpdatedAt = archivedAt
		return nil
	})
}

func (s Store) HistoryPath(workspaceId string, sessionId string) string {
	return filepath.Join(s.SessionDir(workspaceId, sessionId), "history.log")
}

func (s Store) ListWorkspaces() ([]workspace.Workspace, []Warning, error) {
	entries, err := os.ReadDir(s.WorkspaceRoot())
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
		path := filepath.Join(s.WorkspaceRoot(), entry.Name(), "workspace.json")
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
			warnings = append(warnings, Warning{Path: s.WorkspaceDir(ws.Id), Err: err})
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
	byId := make(map[string]workspace.Workspace, len(workspaces))
	for _, ws := range workspaces {
		byId[ws.Id] = ws
	}
	seen := map[string]bool{}
	for _, id := range workspaceIds {
		if seen[id] {
			return nil, fmt.Errorf("duplicate workspace_id %q", id)
		}
		seen[id] = true
		ws, ok := byId[id]
		if !ok {
			return nil, os.ErrNotExist
		}
		ws.SortOrder = len(seen)
		ws.UpdatedAt = now.UTC()
		if err := s.SaveWorkspace(ws); err != nil {
			return nil, err
		}
		byId[id] = ws
	}
	updated, _, err := s.ListWorkspaces()
	return updated, err
}

func (s Store) listSessionsInWorkspace(ws workspace.Workspace) ([]session.View, []Warning, error) {
	var views []session.View
	var warnings []Warning
	for _, child := range ws.Children {
		sess := sessionFromWorkspaceNode(ws, child)
		stateRecord, err := sessionStateFromWorkspace(child.State)
		if err != nil {
			warnings = append(warnings, Warning{Path: filepath.Join(s.WorkspaceDir(ws.Id), "workspace.json"), Err: fmt.Errorf("session %s state: %w", child.Id, err)})
			continue
		}
		view := session.View{Session: sess, State: stateRecord, CommandText: formatCommand(sess.Command)}
		if child.CurrentRun.Exit != nil {
			exit := processExitFromWorkspaceExit(*child.CurrentRun.Exit)
			code := exit.ExitCode
			view.ExitCode = &code
			view.ExitReason = exit.Reason
		}
		views = append(views, view)
	}
	return views, warnings, nil
}

func (s Store) loadSessionNode(workspaceId string, sessionId string) (workspace.SessionNode, error) {
	ws, err := s.LoadWorkspace(workspaceId)
	if err != nil {
		return workspace.SessionNode{}, err
	}
	for _, child := range ws.Children {
		if child.Id == sessionId {
			return child, nil
		}
	}
	return workspace.SessionNode{}, os.ErrNotExist
}

func (s Store) updateSessionNode(workspaceId string, sessionId string, update func(*workspace.SessionNode) error) error {
	ws, err := s.LoadWorkspace(workspaceId)
	if err != nil {
		return err
	}
	for i := range ws.Children {
		if ws.Children[i].Id == sessionId {
			if err := update(&ws.Children[i]); err != nil {
				return err
			}
			ws.UpdatedAt = ws.Children[i].UpdatedAt
			return s.SaveWorkspace(ws)
		}
	}
	return os.ErrNotExist
}

func sessionNodeFromSession(value session.Session) workspace.SessionNode {
	return workspace.SessionNode{
		Id:        value.Id,
		Name:      value.Name,
		LaunchCwd: value.LaunchCwd,
		Command: workspace.CommandRecord{
			Executable:  value.Command.Executable,
			Command:     value.Command.Command,
			Args:        append([]string(nil), value.Command.Args...),
			EnvStrategy: value.Command.EnvStrategy,
			EnvCount:    value.Command.EnvCount,
		},
		History: workspace.HistoryRecord{
			Path:         value.History.Path,
			MaxLines:     value.History.MaxLines,
			MaxBytes:     value.History.MaxBytes,
			MaxLineBytes: value.History.MaxLineBytes,
			Truncated:    value.History.Truncated,
		},
		CreatedAt: value.CreatedAt,
		UpdatedAt: value.UpdatedAt,
	}
}

func sessionFromWorkspaceNode(ws workspace.Workspace, child workspace.SessionNode) session.Session {
	return session.Session{
		SchemaVersion: session.SchemaVersion,
		Id:            child.Id,
		Name:          child.Name,
		WorkspaceId:   ws.Id,
		LaunchCwd:     child.LaunchCwd,
		Command: session.CommandRecord{
			Executable:  child.Command.Executable,
			Command:     child.Command.Command,
			Args:        append([]string(nil), child.Command.Args...),
			EnvStrategy: child.Command.EnvStrategy,
			EnvCount:    child.Command.EnvCount,
		},
		History: session.HistoryRecord{
			Path:         child.History.Path,
			MaxLines:     child.History.MaxLines,
			MaxBytes:     child.History.MaxBytes,
			MaxLineBytes: child.History.MaxLineBytes,
			Truncated:    child.History.Truncated,
		},
		CreatedAt: child.CreatedAt,
		UpdatedAt: child.UpdatedAt,
	}
}

func workspaceStateFromSession(value session.StateRecord) workspace.StateRecord {
	return workspace.StateRecord{SchemaVersion: value.SchemaVersion, State: string(value.State), Reason: value.Reason, UpdatedAt: value.UpdatedAt}
}

func sessionStateFromWorkspace(value workspace.StateRecord) (session.StateRecord, error) {
	state := session.State(value.State)
	if !state.Valid() {
		return session.StateRecord{}, fmt.Errorf("invalid lifecycle state %q", value.State)
	}
	return session.StateRecord{SchemaVersion: value.SchemaVersion, State: state, Reason: value.Reason, UpdatedAt: value.UpdatedAt}, nil
}

func workspaceProcessFromProcess(value process.Record) workspace.ProcessRecord {
	return workspace.ProcessRecord{SchemaVersion: value.SchemaVersion, Pid: value.Pid, OwnerPid: value.OwnerPid, Executable: value.Executable, CommandLine: value.CommandLine, Cwd: value.Cwd, StartedAt: value.StartedAt}
}

func processFromWorkspaceProcess(value workspace.ProcessRecord) process.Record {
	return process.Record{SchemaVersion: value.SchemaVersion, Pid: value.Pid, OwnerPid: value.OwnerPid, Executable: value.Executable, CommandLine: value.CommandLine, Cwd: value.Cwd, StartedAt: value.StartedAt}
}

func workspaceExitFromProcess(value process.ExitRecord) workspace.ExitRecord {
	return workspace.ExitRecord{SchemaVersion: value.SchemaVersion, ExitCode: value.ExitCode, Reason: value.Reason, Forced: value.Forced, Closed: value.Closed, StartedAt: value.StartedAt, EndedAt: value.EndedAt, WaitError: value.WaitError}
}

func processExitFromWorkspaceExit(value workspace.ExitRecord) process.ExitRecord {
	return process.ExitRecord{SchemaVersion: value.SchemaVersion, ExitCode: value.ExitCode, Reason: value.Reason, Forced: value.Forced, Closed: value.Closed, StartedAt: value.StartedAt, EndedAt: value.EndedAt, WaitError: value.WaitError}
}

func cloneWorkspaceProcess(value *workspace.ProcessRecord) *workspace.ProcessRecord {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func cloneWorkspaceExit(value *workspace.ExitRecord) *workspace.ExitRecord {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func upsertSessionNode(ws *workspace.Workspace, child workspace.SessionNode) {
	for i := range ws.Children {
		if ws.Children[i].Id == child.Id {
			child.State = ws.Children[i].State
			child.CurrentRun = ws.Children[i].CurrentRun
			child.ArchivedRuns = append([]workspace.ArchivedRun(nil), ws.Children[i].ArchivedRuns...)
			ws.Children[i] = child
			return
		}
	}
	ws.Children = append(ws.Children, child)
}

func removeSessionNode(ws *workspace.Workspace, sessionId string) {
	for i := range ws.Children {
		if ws.Children[i].Id == sessionId {
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
	if err := replaceFile(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

func replaceFile(tmpName string, path string) error {
	if err := os.Rename(tmpName, path); err == nil {
		return nil
	}
	deadline := time.Now().Add(time.Second)
	for {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			if time.Now().After(deadline) {
				return err
			}
			time.Sleep(10 * time.Millisecond)
			continue
		}
		if err := os.Rename(tmpName, path); err != nil {
			if time.Now().After(deadline) {
				return err
			}
			time.Sleep(10 * time.Millisecond)
			continue
		}
		return nil
	}
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
		return left.Id < right.Id
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
	return command.Command + " " + strings.Join(command.Args, " ")
}
