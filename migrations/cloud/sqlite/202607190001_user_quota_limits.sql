-- +goose Up
CREATE TABLE IF NOT EXISTS user_quota_limits (
  user_id TEXT NOT NULL,
  quota_key TEXT NOT NULL,
  limit_value INTEGER NOT NULL,
  updated_at TEXT NOT NULL,
  updated_by TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (user_id, quota_key),
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE IF EXISTS user_quota_limits;
