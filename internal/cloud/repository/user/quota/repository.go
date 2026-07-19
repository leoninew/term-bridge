package quota

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

type Repository struct {
	db     *sql.DB
	driver string
}

func NewRepository(db *sql.DB, driver string) *Repository {
	return &Repository{db: db, driver: strings.TrimSpace(driver)}
}

func (r *Repository) GetLimit(ctx context.Context, userID, key string) (int, bool, error) {
	userID = strings.TrimSpace(userID)
	key = strings.TrimSpace(key)
	if userID == "" || key == "" {
		return 0, false, nil
	}
	var value int
	err := r.db.QueryRowContext(ctx, `SELECT limit_value FROM user_quota_limits WHERE user_id=? AND quota_key=?`, userID, key).Scan(&value)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return value, true, nil
}

func (r *Repository) UpsertLimit(ctx context.Context, userID, key string, value int, updatedBy string) error {
	userID = strings.TrimSpace(userID)
	key = strings.TrimSpace(key)
	updatedBy = strings.TrimSpace(updatedBy)
	now := r.storeTime(time.Now().UTC())
	if r.driver == "mysql" {
		_, err := r.db.ExecContext(ctx, `INSERT INTO user_quota_limits (user_id,quota_key,limit_value,updated_at,updated_by) VALUES (?,?,?,?,?)
ON DUPLICATE KEY UPDATE limit_value=VALUES(limit_value), updated_at=VALUES(updated_at), updated_by=VALUES(updated_by)`, userID, key, value, now, updatedBy)
		return err
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO user_quota_limits (user_id,quota_key,limit_value,updated_at,updated_by) VALUES (?,?,?,?,?)
ON CONFLICT(user_id,quota_key) DO UPDATE SET limit_value=excluded.limit_value, updated_at=excluded.updated_at, updated_by=excluded.updated_by`, userID, key, value, now, updatedBy)
	return err
}

func (r *Repository) DeleteLimit(ctx context.Context, userID, key string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM user_quota_limits WHERE user_id=? AND quota_key=?`, strings.TrimSpace(userID), strings.TrimSpace(key))
	return err
}

func (r *Repository) storeTime(value time.Time) any {
	value = value.UTC()
	if r.driver == "sqlite" {
		return value.Format(time.RFC3339Nano)
	}
	return value
}
