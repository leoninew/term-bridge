package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

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

func (s Store) SessionDir(workspaceKey string, sessionID string) string {
	return filepath.Join(s.WorkspaceDir(workspaceKey), sessionID)
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

func (s Store) SaveSession(value session.Session) error {
	path := filepath.Join(s.SessionDir(value.WorkspaceKey, value.ID), "session.json")
	return writeJSON(path, value)
}

func (s Store) LoadSession(workspaceKey string, sessionID string) (session.Session, error) {
	var value session.Session
	err := readJSON(filepath.Join(s.SessionDir(workspaceKey, sessionID), "session.json"), &value)
	return value, err
}

func (s Store) SaveState(workspaceKey string, sessionID string, value session.StateRecord) error {
	path := filepath.Join(s.SessionDir(workspaceKey, sessionID), "state.json")
	return writeJSON(path, value)
}

func (s Store) LoadState(workspaceKey string, sessionID string) (session.StateRecord, error) {
	var value session.StateRecord
	err := readJSON(filepath.Join(s.SessionDir(workspaceKey, sessionID), "state.json"), &value)
	return value, err
}

func (s Store) SaveProcess(workspaceKey string, sessionID string, value process.Record) error {
	path := filepath.Join(s.SessionDir(workspaceKey, sessionID), "process.json")
	return writeJSON(path, value)
}

func (s Store) LoadProcess(workspaceKey string, sessionID string) (process.Record, error) {
	var value process.Record
	err := readJSON(filepath.Join(s.SessionDir(workspaceKey, sessionID), "process.json"), &value)
	return value, err
}

func (s Store) SaveExit(workspaceKey string, sessionID string, value process.ExitRecord) error {
	path := filepath.Join(s.SessionDir(workspaceKey, sessionID), "exit.json")
	return writeJSON(path, value)
}

func (s Store) LoadExit(workspaceKey string, sessionID string) (process.ExitRecord, error) {
	var value process.ExitRecord
	err := readJSON(filepath.Join(s.SessionDir(workspaceKey, sessionID), "exit.json"), &value)
	return value, err
}

func (s Store) HistoryPath(workspaceKey string, sessionID string) string {
	return filepath.Join(s.SessionDir(workspaceKey, sessionID), "history.log")
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
	return values, warnings, nil
}

func (s Store) ListSessions() ([]session.View, []Warning, error) {
	workspaces, warnings, err := s.ListWorkspaces()
	if err != nil {
		return nil, warnings, err
	}
	var views []session.View
	for _, ws := range workspaces {
		entries, err := os.ReadDir(s.WorkspaceDir(ws.Key))
		if err != nil {
			warnings = append(warnings, Warning{Path: s.WorkspaceDir(ws.Key), Err: err})
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			sess, err := s.LoadSession(ws.Key, entry.Name())
			if err != nil {
				warnings = append(warnings, Warning{Path: filepath.Join(s.SessionDir(ws.Key, entry.Name()), "session.json"), Err: err})
				continue
			}
			stateRecord, err := s.LoadState(ws.Key, entry.Name())
			if err != nil {
				warnings = append(warnings, Warning{Path: filepath.Join(s.SessionDir(ws.Key, entry.Name()), "state.json"), Err: err})
			}
			view := session.View{Session: sess, State: stateRecord, CommandText: formatCommand(sess.Command)}
			if exit, err := s.LoadExit(ws.Key, entry.Name()); err == nil {
				code := exit.ExitCode
				view.ExitCode = &code
				view.ExitReason = exit.Reason
			} else if !errors.Is(err, os.ErrNotExist) {
				warnings = append(warnings, Warning{Path: filepath.Join(s.SessionDir(ws.Key, entry.Name()), "exit.json"), Err: err})
			}
			views = append(views, view)
		}
	}
	return views, warnings, nil
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
