package device

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"gitee.com/leoninew/TermBridge-go/internal/shared/common/security"
)

var (
	ErrInvalidDeviceIdentity   = errors.New("invalid device identity")
	ErrDevicePublicKeyConflict = errors.New("device public key conflict")
)

type Repository struct {
	db     *sql.DB
	driver string
}

type Device struct {
	Id        string
	Name      string
	PublicKey string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewRepository(db *sql.DB, driver string) *Repository {
	return &Repository{db: db, driver: strings.ToLower(strings.TrimSpace(driver))}
}

func (r *Repository) UpsertDeviceBinding(ctx context.Context, userId string, device Device) error {
	return r.UpsertUserDevice(ctx, userId, device)
}

func (r *Repository) UpsertUserDevice(ctx context.Context, userId string, device Device) error {
	userId = strings.TrimSpace(userId)
	device.Id = strings.TrimSpace(device.Id)
	device.Name = strings.TrimSpace(device.Name)
	if userId == "" || device.Name == "" || !security.IsCanonicalDeviceId(device.Id) {
		return ErrInvalidDeviceIdentity
	}
	publicKey, canonicalPublicKey, err := security.ParseCanonicalEd25519PublicKey(device.PublicKey)
	if err != nil {
		return ErrInvalidDeviceIdentity
	}
	expectedId, err := security.DeviceIdForEd25519PublicKey(publicKey)
	if err != nil {
		return ErrInvalidDeviceIdentity
	}
	if device.Id != expectedId {
		var storedPublicKey string
		err := r.db.QueryRowContext(ctx, `SELECT public_key FROM devices WHERE id=?`, device.Id).Scan(&storedPublicKey)
		switch {
		case err == nil:
			return ErrDevicePublicKeyConflict
		case !errors.Is(err, sql.ErrNoRows):
			return err
		default:
			return ErrInvalidDeviceIdentity
		}
	}
	device.PublicKey = canonicalPublicKey
	now := time.Now().UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := r.upsertDevice(ctx, tx, device, now); err != nil {
		return err
	}
	if err := r.upsertUserDevice(ctx, tx, userId, device.Id, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) ListDevicesForUser(ctx context.Context, userId string) ([]Device, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT d.id,d.name,COALESCE(d.public_key,''),d.created_at,d.updated_at FROM devices d JOIN user_devices ud ON ud.device_id=d.id WHERE ud.user_id=? ORDER BY d.updated_at DESC`, strings.TrimSpace(userId))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	devices := []Device{}
	for rows.Next() {
		var device Device
		var createdAt, updatedAt any
		if err := rows.Scan(&device.Id, &device.Name, &device.PublicKey, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		var err error
		device.CreatedAt, err = scanTime(createdAt)
		if err != nil {
			return nil, err
		}
		device.UpdatedAt, err = scanTime(updatedAt)
		if err != nil {
			return nil, err
		}
		devices = append(devices, device)
	}
	return devices, rows.Err()
}

func (r *Repository) UserOwnsDevice(ctx context.Context, userId string, deviceId string) (bool, error) {
	row := r.db.QueryRowContext(ctx, `SELECT 1 FROM user_devices WHERE user_id=? AND device_id=?`, strings.TrimSpace(userId), strings.TrimSpace(deviceId))
	var value int
	if err := row.Scan(&value); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *Repository) DeleteUserDevice(ctx context.Context, userId string, deviceId string) (bool, error) {
	userId = strings.TrimSpace(userId)
	deviceId = strings.TrimSpace(deviceId)
	if userId == "" || deviceId == "" {
		return false, errors.New("user id and device id are required")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_devices WHERE user_id=? AND device_id=?`, userId, deviceId); err != nil {
		return false, err
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_devices WHERE device_id=?`, deviceId).Scan(&count); err != nil {
		return false, err
	}
	deleteDevice := count == 0
	if deleteDevice {
		if _, err := tx.ExecContext(ctx, `DELETE FROM devices WHERE id=?`, deviceId); err != nil {
			return false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return deleteDevice, nil
}

func (r *Repository) PublicKey(ctx context.Context, deviceId string) (string, error) {
	row := r.db.QueryRowContext(ctx, `SELECT public_key FROM devices WHERE id=?`, strings.TrimSpace(deviceId))
	var publicKey string
	if err := row.Scan(&publicKey); err != nil {
		return "", err
	}
	return publicKey, nil
}

func (r *Repository) upsertDevice(ctx context.Context, tx *sql.Tx, device Device, now time.Time) error {
	if _, err := tx.ExecContext(ctx, r.insertDeviceIfAbsentSQL(), device.Id, device.Name, device.PublicKey, r.storeTime(now), r.storeTime(now)); err != nil {
		return err
	}
	var storedPublicKey string
	if err := tx.QueryRowContext(ctx, `SELECT public_key FROM devices WHERE id=?`, device.Id).Scan(&storedPublicKey); err != nil {
		return err
	}
	if storedPublicKey != device.PublicKey {
		return ErrDevicePublicKeyConflict
	}
	_, err := tx.ExecContext(ctx, `UPDATE devices SET name=?, updated_at=? WHERE id=?`, device.Name, r.storeTime(now), device.Id)
	return err
}

func (r *Repository) insertDeviceIfAbsentSQL() string {
	if r.driver == "mysql" {
		return `INSERT INTO devices (id,name,public_key,created_at,updated_at) VALUES (?,?,?,?,?) ON DUPLICATE KEY UPDATE id=id`
	}
	return `INSERT INTO devices (id,name,public_key,created_at,updated_at) VALUES (?,?,?,?,?) ON CONFLICT(id) DO NOTHING`
}

func (r *Repository) upsertUserDevice(ctx context.Context, tx *sql.Tx, userId string, deviceId string, now time.Time) error {
	var existing string
	err := tx.QueryRowContext(ctx, `SELECT device_id FROM user_devices WHERE user_id=? AND device_id=?`, userId, deviceId).Scan(&existing)
	if err == nil {
		_, err = tx.ExecContext(ctx, `UPDATE user_devices SET role=?, updated_at=? WHERE user_id=? AND device_id=?`, "owner", r.storeTime(now), userId, deviceId)
		return err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO user_devices (user_id,device_id,role,created_at,updated_at) VALUES (?,?,?,?,?)`, userId, deviceId, "owner", r.storeTime(now), r.storeTime(now))
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
		return time.Time{}, fmt.Errorf("unsupported time value type %T", value)
	}
}

func parseStoredTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid RFC3339Nano time value %q", value)
	}
	return parsed, nil
}
