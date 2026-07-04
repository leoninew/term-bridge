-- +goose Up
CREATE TABLE IF NOT EXISTS devices (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  public_key TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS workspaces (
  id TEXT PRIMARY KEY,
  device_id TEXT NOT NULL,
  name TEXT NOT NULL,
  path TEXT NOT NULL,
  sort_order INTEGER NOT NULL DEFAULT 0,
  metadata_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  deleted_at TEXT NULL,
  FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_workspaces_device_deleted ON workspaces(device_id, deleted_at);
CREATE INDEX IF NOT EXISTS idx_workspaces_device_path ON workspaces(device_id, path);
CREATE INDEX IF NOT EXISTS idx_workspaces_device_order ON workspaces(device_id, sort_order);
CREATE INDEX IF NOT EXISTS idx_workspaces_updated ON workspaces(updated_at);

CREATE TABLE IF NOT EXISTS sessions (
  id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL,
  device_id TEXT NOT NULL,
  name TEXT NOT NULL,
  launch_cwd TEXT NOT NULL,
  command_json TEXT NOT NULL,
  history_json TEXT NOT NULL,
  current_state TEXT NOT NULL,
  current_state_reason TEXT NOT NULL DEFAULT '',
  current_run_id TEXT NULL,
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  deleted_at TEXT NULL,
  FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE,
  FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_sessions_workspace_deleted ON sessions(workspace_id, deleted_at);
CREATE INDEX IF NOT EXISTS idx_sessions_workspace_order ON sessions(workspace_id, sort_order);
CREATE INDEX IF NOT EXISTS idx_sessions_device_deleted ON sessions(device_id, deleted_at);
CREATE INDEX IF NOT EXISTS idx_sessions_current_run ON sessions(current_run_id);

CREATE TABLE IF NOT EXISTS session_runs (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  workspace_id TEXT NOT NULL,
  device_id TEXT NOT NULL,
  sequence INTEGER NOT NULL,
  command_json TEXT NOT NULL,
  terminal_size_json TEXT NOT NULL DEFAULT '{}',
  process_json TEXT NOT NULL DEFAULT '{}',
  exit_json TEXT NOT NULL DEFAULT '{}',
  state TEXT NOT NULL,
  state_reason TEXT NOT NULL DEFAULT '',
  started_at TEXT NULL,
  ended_at TEXT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  deleted_at TEXT NULL,
  FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE,
  FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_session_runs_session_sequence ON session_runs(session_id, sequence);
CREATE INDEX IF NOT EXISTS idx_session_runs_session_deleted ON session_runs(session_id, deleted_at);
CREATE INDEX IF NOT EXISTS idx_session_runs_workspace_deleted ON session_runs(workspace_id, deleted_at);
CREATE INDEX IF NOT EXISTS idx_session_runs_device_deleted ON session_runs(device_id, deleted_at);
CREATE INDEX IF NOT EXISTS idx_session_runs_updated ON session_runs(updated_at);

-- +goose Down
DROP INDEX IF EXISTS idx_session_runs_updated;
DROP INDEX IF EXISTS idx_session_runs_device_deleted;
DROP INDEX IF EXISTS idx_session_runs_workspace_deleted;
DROP INDEX IF EXISTS idx_session_runs_session_deleted;
DROP INDEX IF EXISTS idx_session_runs_session_sequence;
DROP TABLE IF EXISTS session_runs;
DROP INDEX IF EXISTS idx_sessions_current_run;
DROP INDEX IF EXISTS idx_sessions_device_deleted;
DROP INDEX IF EXISTS idx_sessions_workspace_order;
DROP INDEX IF EXISTS idx_sessions_workspace_deleted;
DROP TABLE IF EXISTS sessions;
DROP INDEX IF EXISTS idx_workspaces_updated;
DROP INDEX IF EXISTS idx_workspaces_device_order;
DROP INDEX IF EXISTS idx_workspaces_device_path;
DROP INDEX IF EXISTS idx_workspaces_device_deleted;
DROP TABLE IF EXISTS workspaces;
DROP TABLE IF EXISTS devices;
