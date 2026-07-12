-- +goose Up
ALTER TABLE shortcuts ADD COLUMN sort_order INTEGER NOT NULL DEFAULT 0;

WITH ranked AS (
  SELECT id, ROW_NUMBER() OVER (PARTITION BY device_id ORDER BY updated_at DESC, id ASC) - 1 AS position
  FROM shortcuts
)
UPDATE shortcuts
SET sort_order = (SELECT position FROM ranked WHERE ranked.id = shortcuts.id);

CREATE INDEX IF NOT EXISTS idx_shortcuts_device_order ON shortcuts(device_id, sort_order);

-- +goose Down
DROP INDEX IF EXISTS idx_shortcuts_device_order;
ALTER TABLE shortcuts DROP COLUMN sort_order;
