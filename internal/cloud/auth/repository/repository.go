package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"termbridge-go/internal/shared/idgen"
)

const (
	ProviderEmail  = "email"
	ProviderGoogle = "google"
	StatusPending  = "pending_verification"
	StatusEnabled  = "enabled"
)

type Repository struct {
	db     *sql.DB
	driver string
}

type User struct {
	ID              string
	EmailNormalized string
	DisplayName     string
	Status          string
	EmailVerifiedAt sql.NullTime
	LastLoginAt     sql.NullTime
}

type Identity struct {
	ID              string
	UserId          string
	Provider        string
	ProviderSubject string
	PasswordHash    sql.NullString
	OAuthEmail      sql.NullString
}

type AuthCode struct {
	ID              string
	UserId          sql.NullString
	EmailNormalized string
	Purpose         string
	CodeHash        string
	ExpiresAt       time.Time
	Attempts        int
	UsedAt          sql.NullTime
	InvalidatedAt   sql.NullTime
	CreatedAt       time.Time
}

type EmailLog struct {
	EmailNormalized   string
	Purpose           string
	ProviderMessageID string
	Status            string
	ResponseBody      string
}

func New(db *sql.DB, driver string) *Repository {
	return &Repository{db: db, driver: strings.ToLower(strings.TrimSpace(driver))}
}

func (r *Repository) storeTime(value time.Time) any {
	value = value.UTC()
	if r.driver == "sqlite" {
		return value.Format(time.RFC3339Nano)
	}
	return value
}

func (r *Repository) storeNullTime(value sql.NullTime) any {
	if !value.Valid {
		return nil
	}
	return r.storeTime(value.Time)
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (r *Repository) CreateEmailUser(ctx context.Context, email, passwordHash string) (User, error) {
	email = NormalizeEmail(email)
	now := time.Now().UTC()
	userID, err := idgen.New()
	if err != nil {
		return User{}, err
	}
	identityID, err := idgen.New()
	if err != nil {
		return User{}, err
	}
	user := User{ID: userID, EmailNormalized: email, DisplayName: email, Status: StatusPending}
	identity := Identity{ID: identityID, UserId: user.ID, Provider: ProviderEmail, ProviderSubject: email, PasswordHash: sql.NullString{String: passwordHash, Valid: true}}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()
	storedNow := r.storeTime(now)
	if _, err := tx.ExecContext(ctx, `INSERT INTO users (id,email_normalized,display_name,status,created_at,updated_at) VALUES (?,?,?,?,?,?)`, user.ID, user.EmailNormalized, user.DisplayName, user.Status, storedNow, storedNow); err != nil {
		return User{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO user_identities (id,user_id,provider,provider_subject,password_hash,oauth_email,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?)`, identity.ID, identity.UserId, identity.Provider, identity.ProviderSubject, identity.PasswordHash, sql.NullString{}, storedNow, storedNow); err != nil {
		return User{}, err
	}
	return user, tx.Commit()
}

func (r *Repository) CreateGoogleUser(ctx context.Context, email, sub string) (User, error) {
	email = NormalizeEmail(email)
	now := time.Now().UTC()
	userID, err := idgen.New()
	if err != nil {
		return User{}, err
	}
	identityID, err := idgen.New()
	if err != nil {
		return User{}, err
	}
	user := User{ID: userID, EmailNormalized: email, DisplayName: email, Status: StatusEnabled, EmailVerifiedAt: sql.NullTime{Time: now, Valid: true}}
	identity := Identity{ID: identityID, UserId: user.ID, Provider: ProviderGoogle, ProviderSubject: sub, OAuthEmail: sql.NullString{String: email, Valid: true}}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()
	storedNow := r.storeTime(now)
	if _, err := tx.ExecContext(ctx, `INSERT INTO users (id,email_normalized,display_name,status,email_verified_at,created_at,updated_at) VALUES (?,?,?,?,?,?,?)`, user.ID, user.EmailNormalized, user.DisplayName, user.Status, r.storeNullTime(user.EmailVerifiedAt), storedNow, storedNow); err != nil {
		return User{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO user_identities (id,user_id,provider,provider_subject,password_hash,oauth_email,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?)`, identity.ID, identity.UserId, identity.Provider, identity.ProviderSubject, sql.NullString{}, identity.OAuthEmail, storedNow, storedNow); err != nil {
		return User{}, err
	}
	return user, tx.Commit()
}

func (r *Repository) FindUserByEmail(ctx context.Context, email string) (User, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id,email_normalized,display_name,status,email_verified_at,last_login_at FROM users WHERE email_normalized=?`, NormalizeEmail(email))
	return scanUser(row)
}

func (r *Repository) FindUserByID(ctx context.Context, id string) (User, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id,email_normalized,display_name,status,email_verified_at,last_login_at FROM users WHERE id=?`, id)
	return scanUser(row)
}

func (r *Repository) FindIdentity(ctx context.Context, provider, subject string) (Identity, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id,user_id,provider,provider_subject,password_hash,oauth_email FROM user_identities WHERE provider=? AND provider_subject=?`, provider, subject)
	var ident Identity
	err := row.Scan(&ident.ID, &ident.UserId, &ident.Provider, &ident.ProviderSubject, &ident.PasswordHash, &ident.OAuthEmail)
	return ident, err
}

func (r *Repository) FindIdentityForUser(ctx context.Context, userId, provider string) (Identity, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id,user_id,provider,provider_subject,password_hash,oauth_email FROM user_identities WHERE user_id=? AND provider=?`, userId, provider)
	var ident Identity
	err := row.Scan(&ident.ID, &ident.UserId, &ident.Provider, &ident.ProviderSubject, &ident.PasswordHash, &ident.OAuthEmail)
	return ident, err
}

func (r *Repository) MarkEmailVerified(ctx context.Context, userId string) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `UPDATE users SET status=?, email_verified_at=?, updated_at=? WHERE id=?`, StatusEnabled, r.storeTime(now), r.storeTime(now), userId)
	return err
}

func (r *Repository) UpdatePassword(ctx context.Context, userId, hash string) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `UPDATE user_identities SET password_hash=?, updated_at=? WHERE user_id=? AND provider=?`, hash, r.storeTime(now), userId, ProviderEmail)
	return err
}

func (r *Repository) UpdateLastLogin(ctx context.Context, userId string) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `UPDATE users SET last_login_at=?, updated_at=? WHERE id=?`, r.storeTime(now), r.storeTime(now), userId)
	return err
}

func (r *Repository) CreateCode(ctx context.Context, userId *string, email, purpose, codeHash string, expiresAt time.Time) (string, error) {
	email = NormalizeEmail(email)
	now := time.Now().UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	storedNow := r.storeTime(now)
	if _, err := tx.ExecContext(ctx, `UPDATE auth_codes SET invalidated_at=? WHERE email_normalized=? AND purpose=? AND used_at IS NULL AND invalidated_at IS NULL`, storedNow, email, purpose); err != nil {
		return "", err
	}
	var user sql.NullString
	if userId != nil {
		user = sql.NullString{String: *userId, Valid: true}
	}
	id, err := idgen.New()
	if err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO auth_codes (id,user_id,email_normalized,purpose,code_hash,expires_at,attempts,created_at) VALUES (?,?,?,?,?,?,0,?)`, id, user, email, purpose, codeHash, r.storeTime(expiresAt), storedNow); err != nil {
		return "", err
	}
	return id, tx.Commit()
}

func (r *Repository) LatestCode(ctx context.Context, email, purpose string) (AuthCode, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id,user_id,email_normalized,purpose,code_hash,expires_at,attempts,used_at,invalidated_at,created_at FROM auth_codes WHERE email_normalized=? AND purpose=? ORDER BY created_at DESC LIMIT 1`, NormalizeEmail(email), purpose)
	return scanAuthCode(row)
}

func (r *Repository) InvalidateCode(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE auth_codes SET invalidated_at=? WHERE id=? AND used_at IS NULL AND invalidated_at IS NULL`, r.storeTime(time.Now().UTC()), id)
	return err
}

func (r *Repository) IncrementCodeAttempts(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE auth_codes SET attempts=attempts+1 WHERE id=?`, id)
	return err
}

func (r *Repository) MarkCodeUsed(ctx context.Context, id string) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `UPDATE auth_codes SET used_at=? WHERE id=?`, r.storeTime(now), id)
	return err
}

func (r *Repository) SaveEmailLog(ctx context.Context, log EmailLog) error {
	body := log.ResponseBody
	if len(body) > 2048 {
		body = body[:2048]
	}
	id, err := idgen.New()
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO email_delivery_logs (id,email_normalized,purpose,provider_message_id,status,response_body,created_at) VALUES (?,?,?,?,?,?,?)`, id, NormalizeEmail(log.EmailNormalized), log.Purpose, log.ProviderMessageID, log.Status, body, r.storeTime(time.Now().UTC()))
	return err
}

func (r *Repository) CreateOAuthState(ctx context.Context, state string, expiresAt time.Time) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `INSERT INTO oauth_states (state,expires_at,created_at) VALUES (?,?,?)`, state, r.storeTime(expiresAt), r.storeTime(now))
	return err
}

func (r *Repository) UseOAuthState(ctx context.Context, state string) (bool, error) {
	now := time.Now().UTC()
	storedNow := r.storeTime(now)
	res, err := r.db.ExecContext(ctx, `UPDATE oauth_states SET used_at=? WHERE state=? AND used_at IS NULL AND expires_at>?`, storedNow, state, storedNow)
	if err != nil {
		return false, err
	}
	rows, err := res.RowsAffected()
	return rows == 1, err
}

type scanner interface{ Scan(dest ...any) error }

func scanAuthCode(row scanner) (AuthCode, error) {
	var code AuthCode
	var expiresAt any
	var usedAt any
	var invalidatedAt any
	var createdAt any
	if err := row.Scan(&code.ID, &code.UserId, &code.EmailNormalized, &code.Purpose, &code.CodeHash, &expiresAt, &code.Attempts, &usedAt, &invalidatedAt, &createdAt); err != nil {
		return AuthCode{}, err
	}
	var err error
	code.ExpiresAt, err = scanTime(expiresAt)
	if err != nil {
		return AuthCode{}, err
	}
	code.UsedAt, err = scanNullTime(usedAt)
	if err != nil {
		return AuthCode{}, err
	}
	code.InvalidatedAt, err = scanNullTime(invalidatedAt)
	if err != nil {
		return AuthCode{}, err
	}
	code.CreatedAt, err = scanTime(createdAt)
	if err != nil {
		return AuthCode{}, err
	}
	return code, nil
}

func scanUser(row scanner) (User, error) {
	var user User
	var emailVerifiedAt any
	var lastLoginAt any
	if err := row.Scan(&user.ID, &user.EmailNormalized, &user.DisplayName, &user.Status, &emailVerifiedAt, &lastLoginAt); err != nil {
		return User{}, err
	}
	var err error
	user.EmailVerifiedAt, err = scanNullTime(emailVerifiedAt)
	if err != nil {
		return User{}, err
	}
	user.LastLoginAt, err = scanNullTime(lastLoginAt)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func scanNullTime(value any) (sql.NullTime, error) {
	if value == nil {
		return sql.NullTime{}, nil
	}
	parsed, err := scanTime(value)
	if err != nil {
		return sql.NullTime{}, err
	}
	return sql.NullTime{Time: parsed, Valid: true}, nil
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

func IsNotFound(err error) bool { return errors.Is(err, sql.ErrNoRows) }
