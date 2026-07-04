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

	"termbridge-go/internal/agent/model/task/process"
	"termbridge-go/internal/agent/model/task/session"
	"termbridge-go/internal/agent/model/task/workspace"
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

func (s Store) workspaceIndexPath() string {
	return filepath.Join(s.WorkspaceRoot(), "index.json")
}

func (s Store) WorkspaceDir(workspaceId string) string {
	return filepath.Join(s.WorkspaceRoot(), workspaceId)
}

func (s Store) SessionDir(workspaceId string, sessionId string) string {
	return filepath.Join(s.WorkspaceDir(workspaceId), "sessions", sessionId)
}

func (s Store) SaveWorkspace(value workspace.Workspace) error {
	path := filepath.Join(s.WorkspaceDir(value.Id), "workspace.json")
	_, statErr := os.Stat(path)
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}
	if err := writeJSON(path, value); err != nil {
		return err
	}
	if errors.Is(statErr, os.ErrNotExist) {
		return s.appendWorkspaceId(value.Id, value.UpdatedAt)
	}
	return nil
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
	removeString(&ws.SessionIds, sessionId)
	if err := s.SaveWorkspace(ws); err != nil {
		return err
	}
	return os.RemoveAll(s.SessionDir(workspaceId, sessionId))
}

func (s Store) DeleteWorkspace(workspaceId string) error {
	if err := os.RemoveAll(s.WorkspaceDir(workspaceId)); err != nil {
		return err
	}
	return s.removeWorkspaceId(workspaceId, time.Now().UTC())
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
	index, err := s.loadWorkspaceIndex()
	if err != nil {
		warnings = append(warnings, Warning{Path: s.workspaceIndexPath(), Err: err})
	}
	sortWorkspaces(values, index.WorkspaceIds)
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
	ordered, err := completeOrderedIds(workspaceIds, workspaces, func(ws workspace.Workspace) string { return ws.Id }, "workspace_id")
	if err != nil {
		return nil, err
	}
	if err := s.saveWorkspaceIndex(workspace.WorkspaceIndex{SchemaVersion: workspace.SchemaVersion, WorkspaceIds: ordered, UpdatedAt: now.UTC()}); err != nil {
		return nil, err
	}
	updated, _, err := s.ListWorkspaces()
	return updated, err
}

func (s Store) UpdateSessionOrder(workspaceId string, sessionIds []string, now time.Time) ([]session.View, []Warning, error) {
	ws, err := s.LoadWorkspace(workspaceId)
	if err != nil {
		return nil, nil, err
	}
	ordered, err := completeOrderedIds(sessionIds, ws.Children, func(child workspace.SessionNode) string { return child.Id }, "session_id")
	if err != nil {
		return nil, nil, err
	}
	ws.SessionIds = ordered
	ws.UpdatedAt = now.UTC()
	if err := s.SaveWorkspace(ws); err != nil {
		return nil, nil, err
	}
	return s.listSessionsInWorkspace(ws)
}

func (s Store) listSessionsInWorkspace(ws workspace.Workspace) ([]session.View, []Warning, error) {
	var views []session.View
	var warnings []Warning
	children := orderSessionNodes(ws.Children, ws.SessionIds)
	for _, child := range children {
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

func upsertSessionNode(ws *workspace.Workspace, child workspace.SessionNode) {
	for i := range ws.Children {
		if ws.Children[i].Id == child.Id {
			child.State = ws.Children[i].State
			child.CurrentRun = ws.Children[i].CurrentRun
			ws.Children[i] = child
			return
		}
	}
	if len(ws.SessionIds) == 0 {
		for _, existing := range ws.Children {
			ws.SessionIds = append(ws.SessionIds, existing.Id)
		}
	}
	ws.Children = append(ws.Children, child)
	ws.SessionIds = append(ws.SessionIds, child.Id)
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

func (s Store) loadWorkspaceIndex() (workspace.WorkspaceIndex, error) {
	var index workspace.WorkspaceIndex
	err := readJSON(s.workspaceIndexPath(), &index)
	if errors.Is(err, os.ErrNotExist) {
		return workspace.WorkspaceIndex{}, nil
	}
	return index, err
}

func (s Store) saveWorkspaceIndex(index workspace.WorkspaceIndex) error {
	return writeJSON(s.workspaceIndexPath(), index)
}

func (s Store) appendWorkspaceId(workspaceId string, now time.Time) error {
	index, err := s.loadWorkspaceIndex()
	if err != nil {
		return err
	}
	if containsString(index.WorkspaceIds, workspaceId) {
		return nil
	}
	if len(index.WorkspaceIds) == 0 {
		workspaces, _, err := s.ListWorkspaces()
		if err != nil {
			return err
		}
		for _, ws := range workspaces {
			index.WorkspaceIds = append(index.WorkspaceIds, ws.Id)
		}
	} else {
		index.WorkspaceIds = append(index.WorkspaceIds, workspaceId)
	}
	index.SchemaVersion = workspace.SchemaVersion
	index.UpdatedAt = now.UTC()
	return s.saveWorkspaceIndex(index)
}

func (s Store) removeWorkspaceId(workspaceId string, now time.Time) error {
	index, err := s.loadWorkspaceIndex()
	if err != nil {
		return err
	}
	if !removeString(&index.WorkspaceIds, workspaceId) {
		return nil
	}
	index.SchemaVersion = workspace.SchemaVersion
	index.UpdatedAt = now.UTC()
	return s.saveWorkspaceIndex(index)
}

func sortWorkspaces(values []workspace.Workspace, workspaceIds []string) {
	order := orderMap(workspaceIds)
	sort.SliceStable(values, func(i, j int) bool {
		left := values[i]
		right := values[j]
		leftOrder, leftOrdered := order[left.Id]
		rightOrder, rightOrdered := order[right.Id]
		if leftOrdered != rightOrdered {
			return leftOrdered
		}
		if leftOrdered && leftOrder != rightOrder {
			return leftOrder < rightOrder
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

func orderSessionNodes(children []workspace.SessionNode, sessionIds []string) []workspace.SessionNode {
	ordered := append([]workspace.SessionNode(nil), children...)
	order := orderMap(sessionIds)
	sort.SliceStable(ordered, func(i, j int) bool {
		left := ordered[i]
		right := ordered[j]
		leftOrder, leftOrdered := order[left.Id]
		rightOrder, rightOrdered := order[right.Id]
		if leftOrdered != rightOrdered {
			return leftOrdered
		}
		if leftOrdered && leftOrder != rightOrder {
			return leftOrder < rightOrder
		}
		if !left.CreatedAt.Equal(right.CreatedAt) {
			return left.CreatedAt.Before(right.CreatedAt)
		}
		if left.Name != right.Name {
			return left.Name < right.Name
		}
		return left.Id < right.Id
	})
	return ordered
}

func completeOrderedIds[T any](ids []string, values []T, idFor func(T) string, label string) ([]string, error) {
	byId := make(map[string]bool, len(values))
	for _, value := range values {
		byId[idFor(value)] = true
	}
	seen := map[string]bool{}
	ordered := make([]string, 0, len(values))
	for _, id := range ids {
		if seen[id] {
			return nil, fmt.Errorf("duplicate %s %q", label, id)
		}
		if !byId[id] {
			return nil, fmt.Errorf("%s %q: %w", label, id, os.ErrNotExist)
		}
		seen[id] = true
		ordered = append(ordered, id)
	}
	for _, value := range values {
		id := idFor(value)
		if !seen[id] {
			ordered = append(ordered, id)
		}
	}
	return ordered, nil
}

func orderMap(ids []string) map[string]int {
	order := make(map[string]int, len(ids))
	for index, id := range ids {
		if _, exists := order[id]; !exists {
			order[id] = index
		}
	}
	return order
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func removeString(values *[]string, target string) bool {
	for index, value := range *values {
		if value == target {
			*values = append((*values)[:index], (*values)[index+1:]...)
			return true
		}
	}
	return false
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
