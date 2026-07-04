package device

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

const BindingCodeTTL = 10 * time.Minute

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

func (r *Repository) UpsertDeviceBinding(ctx context.Context, userID string, device Device) error {
	return r.UpsertUserDevice(ctx, userID, device)
}

func (r *Repository) UpsertUserDevice(ctx context.Context, userID string, device Device) error {
	userID = strings.TrimSpace(userID)
	device.ID = strings.TrimSpace(device.ID)
	device.Name = strings.TrimSpace(device.Name)
	device.PublicKey = strings.TrimSpace(device.PublicKey)
	if userID == "" || device.ID == "" || device.Name == "" || device.PublicKey == "" {
		return errors.New("user id, device id, device name and public key are required")
	}
	now := time.Now().UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := r.upsertDevice(ctx, tx, device, now); err != nil {
		return err
	}
	if err := r.upsertUserDevice(ctx, tx, userID, device.ID, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) ListDevicesForUser(ctx context.Context, userID string) ([]Device, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT d.id,d.name,COALESCE(d.public_key,''),d.created_at,d.updated_at FROM devices d JOIN user_devices ud ON ud.device_id=d.id WHERE ud.user_id=? ORDER BY d.updated_at DESC`, strings.TrimSpace(userID))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	devices := []Device{}
	for rows.Next() {
		var device Device
		var createdAt, updatedAt any
		if err := rows.Scan(&device.ID, &device.Name, &device.PublicKey, &createdAt, &updatedAt); err != nil {
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

func (r *Repository) UserOwnsDevice(ctx context.Context, userID string, deviceID string) (bool, error) {
	row := r.db.QueryRowContext(ctx, `SELECT 1 FROM user_devices WHERE user_id=? AND device_id=?`, strings.TrimSpace(userID), strings.TrimSpace(deviceID))
	var value int
	if err := row.Scan(&value); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *Repository) DeleteUserDevice(ctx context.Context, userID string, deviceID string) error {
	userID = strings.TrimSpace(userID)
	deviceID = strings.TrimSpace(deviceID)
	if userID == "" || deviceID == "" {
		return errors.New("user id and device id are required")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_devices WHERE user_id=? AND device_id=?`, userID, deviceID); err != nil {
		return err
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_devices WHERE device_id=?`, deviceID).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		if _, err := tx.ExecContext(ctx, `DELETE FROM devices WHERE id=?`, deviceID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *Repository) PublicKey(ctx context.Context, deviceID string) (string, error) {
	row := r.db.QueryRowContext(ctx, `SELECT public_key FROM devices WHERE id=?`, strings.TrimSpace(deviceID))
	var publicKey string
	if err := row.Scan(&publicKey); err != nil {
		return "", err
	}
	return publicKey, nil
}

func (r *Repository) CreateBindingCode(ctx context.Context, userID string, expiresAt time.Time) (string, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "", errors.New("user id is required")
	}
	code, err := randomToken()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	_, err = r.db.ExecContext(ctx, `INSERT INTO device_binding_codes (code_hash,user_id,expires_at,created_at) VALUES (?,?,?,?)`, hashToken(code), userID, r.storeTime(expiresAt), r.storeTime(now))
	return code, err
}

func (r *Repository) UseBindingCode(ctx context.Context, code string) (string, bool, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", false, nil
	}
	now := time.Now().UTC()
	storedNow := r.storeTime(now)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", false, err
	}
	defer func() { _ = tx.Rollback() }()
	codeHash := hashToken(code)
	row := tx.QueryRowContext(ctx, `SELECT user_id FROM device_binding_codes WHERE code_hash=? AND used_at IS NULL AND expires_at>?`, codeHash, storedNow)
	var userID string
	if err := row.Scan(&userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE device_binding_codes SET used_at=? WHERE code_hash=? AND used_at IS NULL`, storedNow, codeHash); err != nil {
		return "", false, err
	}
	return userID, true, tx.Commit()
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

func (r *Repository) upsertUserDevice(ctx context.Context, tx *sql.Tx, userID string, deviceID string, now time.Time) error {
	var existing string
	err := tx.QueryRowContext(ctx, `SELECT device_id FROM user_devices WHERE user_id=? AND device_id=?`, userID, deviceID).Scan(&existing)
	if err == nil {
		_, err = tx.ExecContext(ctx, `UPDATE user_devices SET role=?, updated_at=? WHERE user_id=? AND device_id=?`, "owner", r.storeTime(now), userID, deviceID)
		return err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO user_devices (user_id,device_id,role,created_at,updated_at) VALUES (?,?,?,?,?)`, userID, deviceID, "owner", r.storeTime(now), r.storeTime(now))
	return err
}

func (r *Repository) storeTime(value time.Time) any {
	value = value.UTC()
	if r.driver == "sqlite" {
		return value.Format(time.RFC3339Nano)
	}
	return value
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func hashToken(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:])
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
