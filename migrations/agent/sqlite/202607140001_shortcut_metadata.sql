-- +goose Up
ALTER TABLE shortcuts ADD COLUMN icon TEXT NULL;
ALTER TABLE shortcuts ADD COLUMN enabled INTEGER NOT NULL DEFAULT 1;
ALTER TABLE shortcuts ADD COLUMN tags_json TEXT NOT NULL DEFAULT '[]';
ALTER TABLE shortcuts ADD COLUMN last_used_at TEXT NULL;

-- +goose Down
ALTER TABLE shortcuts DROP COLUMN last_used_at;
ALTER TABLE shortcuts DROP COLUMN tags_json;
ALTER TABLE shortcuts DROP COLUMN enabled;
ALTER TABLE shortcuts DROP COLUMN icon;
