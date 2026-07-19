-- +goose Up
CREATE TABLE IF NOT EXISTS user_quota_limits (
  user_id VARCHAR(64) NOT NULL,
  quota_key VARCHAR(128) NOT NULL,
  limit_value INT NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  updated_by VARCHAR(64) NOT NULL DEFAULT '',
  PRIMARY KEY (user_id, quota_key),
  CONSTRAINT fk_user_quota_limits_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE IF EXISTS user_quota_limits;
