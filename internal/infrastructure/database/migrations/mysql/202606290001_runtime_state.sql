-- +goose Up
CREATE TABLE workspaces (
  id VARCHAR(32) PRIMARY KEY,
  device_id VARCHAR(128) NOT NULL,
  name VARCHAR(255) NOT NULL,
  path VARCHAR(1024) NOT NULL,
  sort_order BIGINT NOT NULL DEFAULT 0,
  metadata_json JSON NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  deleted_at DATETIME(6) NULL,
  CONSTRAINT fk_workspaces_device FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE,
  INDEX idx_workspaces_device_deleted (device_id, deleted_at),
  INDEX idx_workspaces_device_path (device_id, path),
  INDEX idx_workspaces_device_order (device_id, sort_order),
  INDEX idx_workspaces_updated (updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE sessions (
  id VARCHAR(32) PRIMARY KEY,
  workspace_id VARCHAR(32) NOT NULL,
  device_id VARCHAR(128) NOT NULL,
  name VARCHAR(255) NOT NULL,
  launch_cwd VARCHAR(1024) NOT NULL,
  command_json JSON NOT NULL,
  history_json JSON NOT NULL,
  current_state VARCHAR(32) NOT NULL,
  current_state_reason VARCHAR(255) NOT NULL DEFAULT '',
  current_run_id VARCHAR(32) NULL,
  sort_order BIGINT NOT NULL DEFAULT 0,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  deleted_at DATETIME(6) NULL,
  CONSTRAINT fk_sessions_workspace FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE,
  CONSTRAINT fk_sessions_device FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE,
  INDEX idx_sessions_workspace_deleted (workspace_id, deleted_at),
  INDEX idx_sessions_workspace_order (workspace_id, sort_order),
  INDEX idx_sessions_device_deleted (device_id, deleted_at),
  INDEX idx_sessions_current_run (current_run_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE session_runs (
  id VARCHAR(32) PRIMARY KEY,
  session_id VARCHAR(32) NOT NULL,
  workspace_id VARCHAR(32) NOT NULL,
  device_id VARCHAR(128) NOT NULL,
  sequence BIGINT NOT NULL,
  command_json JSON NOT NULL,
  terminal_size_json JSON NOT NULL,
  process_json JSON NOT NULL,
  exit_json JSON NOT NULL,
  state VARCHAR(32) NOT NULL,
  state_reason VARCHAR(255) NOT NULL DEFAULT '',
  started_at DATETIME(6) NULL,
  ended_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  deleted_at DATETIME(6) NULL,
  CONSTRAINT fk_session_runs_session FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE,
  CONSTRAINT fk_session_runs_workspace FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE,
  CONSTRAINT fk_session_runs_device FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE,
  INDEX idx_session_runs_session_sequence (session_id, sequence),
  INDEX idx_session_runs_session_deleted (session_id, deleted_at),
  INDEX idx_session_runs_workspace_deleted (workspace_id, deleted_at),
  INDEX idx_session_runs_device_deleted (device_id, deleted_at),
  INDEX idx_session_runs_updated (updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE IF EXISTS session_runs;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS workspaces;
