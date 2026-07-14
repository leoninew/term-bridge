-- +goose Up
ALTER TABLE shortcuts
  ADD COLUMN icon TEXT NULL,
  ADD COLUMN enabled BOOLEAN NOT NULL DEFAULT TRUE,
  ADD COLUMN tags_json TEXT NOT NULL DEFAULT '[]',
  ADD COLUMN last_used_at DATETIME(6) NULL;

-- +goose Down
ALTER TABLE shortcuts
  DROP COLUMN last_used_at,
  DROP COLUMN tags_json,
  DROP COLUMN enabled,
  DROP COLUMN icon;
