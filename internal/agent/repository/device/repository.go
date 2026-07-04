package device

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

type Repository struct {
	db     *sql.DB
	driver string
}

type Device struct {
	ID        string
	Name      string
	PublicKey string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewRepository(db *sql.DB, driver string) *Repository {
	return &Repository{db: db, driver: strings.ToLower(strings.TrimSpace(driver))}
}

func (r *Repository) UpsertLocalDevice(ctx context.Context, device Device) (Device, error) {
	device.ID = strings.TrimSpace(device.ID)
	device.Name = strings.TrimSpace(device.Name)
	device.PublicKey = strings.TrimSpace(device.PublicKey)
	if device.ID == "" || device.Name == "" {
		return Device{}, errors.New("device id and device name are required")
	}
	now := time.Now().UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Device{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := r.upsertDevice(ctx, tx, device, now); err != nil {
		return Device{}, err
	}
	if err := tx.Commit(); err != nil {
		return Device{}, err
	}
	return r.Device(ctx, device.ID)
}

func (r *Repository) Device(ctx context.Context, deviceID string) (Device, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id,name,COALESCE(public_key,''),created_at,updated_at FROM devices WHERE id=?`, strings.TrimSpace(deviceID))
	var device Device
	var createdAt, updatedAt any
	if err := row.Scan(&device.ID, &device.Name, &device.PublicKey, &createdAt, &updatedAt); err != nil {
		return Device{}, err
	}
	var err error
	device.CreatedAt, err = scanTime(createdAt)
	if err != nil {
		return Device{}, err
	}
	device.UpdatedAt, err = scanTime(updatedAt)
	if err != nil {
		return Device{}, err
	}
	return device, nil
}

func (r *Repository) upsertDevice(ctx context.Context, tx *sql.Tx, device Device, now time.Time) error {
	var existing string
	err := tx.QueryRowContext(ctx, `SELECT id FROM devices WHERE id=?`, device.ID).Scan(&existing)
	if err == nil {
		_, err = tx.ExecContext(ctx, `UPDATE devices SET name=?, public_key=CASE WHEN ?='' THEN public_key ELSE ? END, updated_at=? WHERE id=?`, device.Name, device.PublicKey, device.PublicKey, r.storeTime(now), device.ID)
		return err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO devices (id,name,public_key,created_at,updated_at) VALUES (?,?,?,?,?)`, device.ID, device.Name, device.PublicKey, r.storeTime(now), r.storeTime(now))
	return err
}

func (r *Repository) storeTime(value time.Time) any {
	value = value.UTC()
	if r.driver == "sqlite" {
		return value.Format(time.RFC3339Nano)
	}
	return value
}

func scanTime(value any) (time.Time, error) {
	switch typed := value.(type) {
	case time.Time:
		return typed, nil
	case string:
		return parseStoredTime(typed)
	case []byte:
		return parseStoredTime(string(typed))
	default:
		return time.Time{}, errors.New("unsupported time value")
	}
}

func parseStoredTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	return time.Parse(time.RFC3339Nano, value)
}
