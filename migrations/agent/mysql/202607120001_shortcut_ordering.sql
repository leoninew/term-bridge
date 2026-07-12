-- +goose Up
DROP PROCEDURE IF EXISTS ensure_shortcut_sort_order_column;
-- +goose StatementBegin
CREATE PROCEDURE ensure_shortcut_sort_order_column()
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'shortcuts'
      AND COLUMN_NAME = 'sort_order'
  ) THEN
    ALTER TABLE shortcuts ADD COLUMN sort_order BIGINT NOT NULL DEFAULT 0;
  END IF;
END
-- +goose StatementEnd
CALL ensure_shortcut_sort_order_column();
DROP PROCEDURE ensure_shortcut_sort_order_column;

DROP PROCEDURE IF EXISTS backfill_shortcut_sort_order;
-- +goose StatementBegin
CREATE PROCEDURE backfill_shortcut_sort_order()
BEGIN
  DECLARE done BOOLEAN DEFAULT FALSE;
  DECLARE shortcut_id VARCHAR(32);
  DECLARE shortcut_device_id VARCHAR(128);
  DECLARE previous_device_id VARCHAR(128) DEFAULT '';
  DECLARE position BIGINT DEFAULT -1;
  DECLARE shortcuts_cursor CURSOR FOR
    SELECT id, device_id
    FROM shortcuts
    ORDER BY device_id ASC, updated_at DESC, id ASC;
  DECLARE CONTINUE HANDLER FOR NOT FOUND SET done = TRUE;

  OPEN shortcuts_cursor;
  read_shortcut: LOOP
    FETCH shortcuts_cursor INTO shortcut_id, shortcut_device_id;
    IF done THEN
      LEAVE read_shortcut;
    END IF;
    IF shortcut_device_id = previous_device_id THEN
      SET position = position + 1;
    ELSE
      SET previous_device_id = shortcut_device_id;
      SET position = 0;
    END IF;
    UPDATE shortcuts
    SET sort_order = position
    WHERE id = shortcut_id;
  END LOOP;
  CLOSE shortcuts_cursor;
END
-- +goose StatementEnd
CALL backfill_shortcut_sort_order();
DROP PROCEDURE backfill_shortcut_sort_order;

DROP PROCEDURE IF EXISTS ensure_shortcut_sort_order_index;
-- +goose StatementBegin
CREATE PROCEDURE ensure_shortcut_sort_order_index()
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'shortcuts'
      AND INDEX_NAME = 'idx_shortcuts_device_order'
  ) THEN
    CREATE INDEX idx_shortcuts_device_order ON shortcuts(device_id, sort_order);
  END IF;
END
-- +goose StatementEnd
CALL ensure_shortcut_sort_order_index();
DROP PROCEDURE ensure_shortcut_sort_order_index;

-- +goose Down
ALTER TABLE shortcuts DROP INDEX idx_shortcuts_device_order;
ALTER TABLE shortcuts DROP COLUMN sort_order;
