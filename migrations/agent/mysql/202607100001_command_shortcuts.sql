-- +goose Up
CREATE TABLE IF NOT EXISTS shortcuts (
  id VARCHAR(32) PRIMARY KEY,
  device_id VARCHAR(128) NOT NULL,
  name VARCHAR(255) NOT NULL,
  command TEXT NOT NULL,
  description TEXT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  INDEX idx_shortcuts_device_updated (device_id, updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE IF EXISTS shortcuts;
