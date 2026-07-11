-- +goose Up
CREATE TABLE IF NOT EXISTS shortcuts (
  id TEXT PRIMARY KEY,
  device_id TEXT NOT NULL,
  name TEXT NOT NULL,
  command TEXT NOT NULL,
  description TEXT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_shortcuts_device_updated ON shortcuts(device_id, updated_at);

-- +goose Down
DROP INDEX IF EXISTS idx_shortcuts_device_updated;
DROP TABLE IF EXISTS shortcuts;
