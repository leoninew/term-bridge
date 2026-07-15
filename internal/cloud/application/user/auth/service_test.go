package auth

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	clouddb "gitee.com/leoninew/TermBridge-go/internal/cloud/infrastructure/database"
	authmodel "gitee.com/leoninew/TermBridge-go/internal/cloud/model/user/auth"
	repository "gitee.com/leoninew/TermBridge-go/internal/cloud/repository/user/auth"
	sharedauth "gitee.com/leoninew/TermBridge-go/internal/shared/common/auth"

	_ "modernc.org/sqlite"
)

type sentEmail struct {
	To      string
	Subject string
	HTML    string
}

type recordingSender struct {
	result EmailResult
	err    error
	sent   []sentEmail
}

func (s *recordingSender) Send(ctx context.Context, to, subject, html string) (EmailResult, error) {
	s.sent = append(s.sent, sentEmail{To: to, Subject: subject, HTML: html})
	return s.result, s.err
}

func TestRegisterRecordsActiveCodeAndDeliveryLog(t *testing.T) {
	db := openAuthServiceTestDB(t)
	sender := &recordingSender{result: EmailResult{MessageId: "msg-1", Success: true, ResponseBody: "ok"}}
	service := newAuthServiceForTest(db, sender)

	if err := service.Register(context.Background(), "User@Example.Test", "valid-password"); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if len(sender.sent) != 1 {
		t.Fatalf("sent email count = %d, want 1", len(sender.sent))
	}
	email := sender.sent[0]
	if email.Subject != "TermBridge verification code" {
		t.Fatalf("verification email subject = %q, want TermBridge verification code", email.Subject)
	}
	for _, want := range []string{"background-color:#f1f5f9", "max-width:480px", "letter-spacing:6px", "color:#0369a1", "It expires in 2 minutes. Do not share this code with anyone."} {
		if !strings.Contains(email.HTML, want) {
			t.Fatalf("verification email HTML does not contain %q: %s", want, email.HTML)
		}
	}
	if code := codeFromLatestEmail(t, sender); len(code) != 6 {
		t.Fatalf("verification email code length = %d, want 6", len(code))
	}

	assertAuthTableCount(t, db, "users", 1)
	assertAuthTableCount(t, db, "user_identities", 1)
	assertAuthTableCount(t, db, "auth_codes", 1)
	assertAuthTableCount(t, db, "email_delivery_logs", 1)

	code := latestCodeForTest(t, db, "user@example.test", PurposeEmailVerification)
	if code.InvalidatedAt.Valid {
		t.Fatal("verification code invalidated_at is valid, want active code")
	}
	if code.UsedAt.Valid {
		t.Fatal("verification code used_at is valid, want unused code")
	}
	assertLatestEmailLogStatus(t, db, "user@example.test", PurposeEmailVerification, "success")
}

func TestRegisterRecordsFailedCodeDelivery(t *testing.T) {
	db := openAuthServiceTestDB(t)
	sendErr := errors.New("mail provider unavailable")
	service := newAuthServiceForTest(db, &recordingSender{result: EmailResult{MessageId: "msg-2", Success: false, ResponseBody: "failed"}, err: sendErr})

	err := service.Register(context.Background(), "User@Example.Test", "valid-password")
	if !errors.Is(err, sendErr) {
		t.Fatalf("Register() error = %v, want %v", err, sendErr)
	}

	assertAuthTableCount(t, db, "users", 1)
	assertAuthTableCount(t, db, "user_identities", 1)
	assertAuthTableCount(t, db, "auth_codes", 1)
	assertAuthTableCount(t, db, "email_delivery_logs", 1)

	code := latestCodeForTest(t, db, "user@example.test", PurposeEmailVerification)
	if !code.InvalidatedAt.Valid {
		t.Fatal("verification code invalidated_at is invalid, want failed delivery code invalidated")
	}
	assertLatestEmailLogStatus(t, db, "user@example.test", PurposeEmailVerification, "failed")
}

func TestVerifyEmailMarksUserAndCodeInOneOperation(t *testing.T) {
	db := openAuthServiceTestDB(t)
	sender := &recordingSender{result: EmailResult{MessageId: "msg-3", Success: true}}
	service := newAuthServiceForTest(db, sender)

	if err := service.Register(context.Background(), "User@Example.Test", "valid-password"); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	code := codeFromLatestEmail(t, sender)
	if err := service.VerifyEmail(context.Background(), "user@example.test", code); err != nil {
		t.Fatalf("VerifyEmail() error = %v", err)
	}

	var status string
	var emailVerifiedAt sql.NullString
	if err := db.QueryRow(`SELECT status,email_verified_at FROM users WHERE email_normalized=?`, "user@example.test").Scan(&status, &emailVerifiedAt); err != nil {
		t.Fatalf("query user: %v", err)
	}
	if status != repository.StatusEnabled {
		t.Fatalf("user status = %q, want %q", status, repository.StatusEnabled)
	}
	if !emailVerifiedAt.Valid {
		t.Fatal("email_verified_at is invalid, want verified timestamp")
	}
	usedAt := latestCodeForTest(t, db, "user@example.test", PurposeEmailVerification).UsedAt
	if !usedAt.Valid {
		t.Fatal("verification code used_at is invalid, want used timestamp")
	}
}

func TestConfirmPasswordResetMarksCodeUsed(t *testing.T) {
	db := openAuthServiceTestDB(t)
	sender := &recordingSender{result: EmailResult{MessageId: "msg-4", Success: true}}
	service := newAuthServiceForTest(db, sender)
	ctx := context.Background()

	if err := service.Register(ctx, "User@Example.Test", "valid-password"); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := service.VerifyEmail(ctx, "user@example.test", codeFromLatestEmail(t, sender)); err != nil {
		t.Fatalf("VerifyEmail() error = %v", err)
	}
	if err := service.RequestPasswordReset(ctx, "user@example.test"); err != nil {
		t.Fatalf("RequestPasswordReset() error = %v", err)
	}
	resetCode := codeFromLatestEmail(t, sender)
	if err := service.ConfirmPasswordReset(ctx, "user@example.test", resetCode, "new-valid-password"); err != nil {
		t.Fatalf("ConfirmPasswordReset() error = %v", err)
	}

	usedAt := latestCodeForTest(t, db, "user@example.test", PurposePasswordReset).UsedAt
	if !usedAt.Valid {
		t.Fatal("password reset code used_at is invalid, want used timestamp")
	}
	if _, err := service.Login(ctx, "user@example.test", "new-valid-password"); err != nil {
		t.Fatalf("Login() with reset password error = %v", err)
	}
}

func TestLoginReturnsLastLoginUpdateError(t *testing.T) {
	db := openAuthServiceTestDB(t)
	sender := &recordingSender{result: EmailResult{MessageId: "msg-5", Success: true}}
	service := newAuthServiceForTest(db, sender)
	ctx := context.Background()

	if err := service.Register(ctx, "User@Example.Test", "valid-password"); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := service.VerifyEmail(ctx, "user@example.test", codeFromLatestEmail(t, sender)); err != nil {
		t.Fatalf("VerifyEmail() error = %v", err)
	}
	if _, err := db.Exec(`CREATE TRIGGER fail_last_login BEFORE UPDATE OF last_login_at ON users BEGIN SELECT RAISE(ABORT, 'last login update failed'); END`); err != nil {
		t.Fatalf("create last login trigger: %v", err)
	}
	_, err := service.Login(ctx, "user@example.test", "valid-password")
	if err == nil || !strings.Contains(err.Error(), "last login update failed") {
		t.Fatalf("Login() error = %v, want last login update error", err)
	}
}

func newAuthServiceForTest(db *sql.DB, sender EmailSender) *Service {
	repo := repository.New(db, "sqlite")
	return New(repo, sharedauth.NewTokenService([]byte("0123456789abcdef0123456789abcdef")), Config{PasswordPolicy: PasswordPolicy{MinLength: 8, MaxLength: 128}, Code: CodePolicy{Length: 6, Ttl: 2 * time.Minute, ResendCooldown: 0, MaxAttempts: 5}}, sender, NewProviderRegistry())
}

func openAuthServiceTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	if err := clouddb.Migrate(context.Background(), db, "sqlite"); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	return db
}

func assertAuthTableCount(t *testing.T, db *sql.DB, table string, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&got); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if got != want {
		t.Fatalf("%s count = %d, want %d", table, got, want)
	}
}

func latestCodeForTest(t *testing.T, db *sql.DB, email string, purpose string) repository.AuthCode {
	t.Helper()
	code, err := repository.New(db, "sqlite").LatestCode(context.Background(), email, purpose)
	if err != nil {
		t.Fatalf("LatestCode(%s, %s) error = %v", email, purpose, err)
	}
	return code
}

func assertLatestEmailLogStatus(t *testing.T, db *sql.DB, email string, purpose string, want string) {
	t.Helper()
	var status string
	if err := db.QueryRow(`SELECT status FROM email_delivery_logs WHERE email_normalized=? AND purpose=? ORDER BY created_at DESC LIMIT 1`, email, purpose).Scan(&status); err != nil {
		t.Fatalf("query email log: %v", err)
	}
	if status != want {
		t.Fatalf("email log status = %q, want %q", status, want)
	}
}

func codeFromLatestEmail(t *testing.T, sender *recordingSender) string {
	t.Helper()
	if len(sender.sent) == 0 {
		t.Fatal("no sent emails recorded")
	}
	matches := regexp.MustCompile(`<strong>([A-Z0-9]+)</strong>`).FindStringSubmatch(sender.sent[len(sender.sent)-1].HTML)
	if len(matches) != 2 {
		t.Fatalf("sent email HTML %q does not contain code", sender.sent[len(sender.sent)-1].HTML)
	}
	return matches[1]
}

type testExternalProvider struct {
	id       string
	identity ExternalIdentity
}

func (p testExternalProvider) Id() string { return p.id }

func (p testExternalProvider) AuthCodeURL(state string) string {
	return "https://provider.example.test/authorize?state=" + state
}

func (p testExternalProvider) ExchangeIdentity(context.Context, string) (ExternalIdentity, error) {
	return p.identity, nil
}

func TestExternalLoginAllowsSameEmailAcrossProviders(t *testing.T) {
	db := openAuthServiceTestDB(t)
	repo := repository.New(db, "sqlite")
	github := testExternalProvider{id: repository.ProviderGitHub, identity: ExternalIdentity{Provider: repository.ProviderGitHub, Subject: "1001", Email: "same@example.test", EmailVerified: true}}
	google := testExternalProvider{id: repository.ProviderGoogle, identity: ExternalIdentity{Provider: repository.ProviderGoogle, Subject: "google-subject", Email: "same@example.test", EmailVerified: true}}
	service := New(repo, sharedauth.NewTokenService([]byte("0123456789abcdef0123456789abcdef")), Config{PasswordPolicy: PasswordPolicy{MinLength: 8, MaxLength: 128}, Code: CodePolicy{Length: 6, Ttl: 2 * time.Minute, MaxAttempts: 5}}, nil, NewProviderRegistry(github, google))

	githubState := externalProviderState(t, service, repository.ProviderGitHub)
	githubLogin, err := service.ExternalCallback(context.Background(), repository.ProviderGitHub, "github-code", githubState)
	if err != nil || githubLogin.AccessToken == "" {
		t.Fatalf("GitHub ExternalCallback() = %#v, %v", githubLogin, err)
	}
	googleState := externalProviderState(t, service, repository.ProviderGoogle)
	googleLogin, err := service.ExternalCallback(context.Background(), repository.ProviderGoogle, "google-code", googleState)
	if err != nil || googleLogin.AccessToken == "" {
		t.Fatalf("Google ExternalCallback() = %#v, %v", googleLogin, err)
	}

	var users int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users WHERE email_normalized=?`, "same@example.test").Scan(&users); err != nil {
		t.Fatalf("count provider-scoped users: %v", err)
	}
	if users != 2 {
		t.Fatalf("provider-scoped users = %d, want 2", users)
	}
}

func TestExternalLoginRejectsStateFromAnotherProvider(t *testing.T) {
	db := openAuthServiceTestDB(t)
	github := testExternalProvider{id: repository.ProviderGitHub, identity: ExternalIdentity{Provider: repository.ProviderGitHub, Subject: "1001", Email: "github@example.test", EmailVerified: true}}
	google := testExternalProvider{id: repository.ProviderGoogle, identity: ExternalIdentity{Provider: repository.ProviderGoogle, Subject: "google-subject", Email: "google@example.test", EmailVerified: true}}
	service := New(repository.New(db, "sqlite"), sharedauth.NewTokenService([]byte("0123456789abcdef0123456789abcdef")), Config{}, nil, NewProviderRegistry(github, google))

	githubState := externalProviderState(t, service, repository.ProviderGitHub)
	_, err := service.ExternalCallback(context.Background(), repository.ProviderGoogle, "google-code", githubState)
	if !errors.Is(err, authmodel.ErrInvalidCredentials) {
		t.Fatalf("cross-provider ExternalCallback() error = %v, want invalid credentials", err)
	}
}

func TestIssueUserTokenPreservesAccountProvider(t *testing.T) {
	db := openAuthServiceTestDB(t)
	repo := repository.New(db, "sqlite")
	user, err := repo.CreateExternalUser(context.Background(), repository.ProviderGitHub, "github@example.test", "1001")
	if err != nil {
		t.Fatalf("CreateExternalUser() error = %v", err)
	}
	tokens := sharedauth.NewTokenService([]byte("0123456789abcdef0123456789abcdef"))
	service := New(repo, tokens, Config{}, nil, NewProviderRegistry())

	result, err := service.IssueUserToken(context.Background(), user.Id)
	if err != nil {
		t.Fatalf("IssueUserToken() error = %v", err)
	}
	claims, err := tokens.Verify(result.AccessToken)
	if err != nil {
		t.Fatalf("verify issued token: %v", err)
	}
	if claims.Provider != repository.ProviderGitHub {
		t.Fatalf("issued token provider = %q, want %q", claims.Provider, repository.ProviderGitHub)
	}
}

func externalProviderState(t *testing.T, service *Service, providerId string) string {
	t.Helper()
	authUrl, err := service.ExternalAuthURL(context.Background(), providerId)
	if err != nil {
		t.Fatalf("ExternalAuthURL(%q) error = %v", providerId, err)
	}
	parsed, err := url.Parse(authUrl)
	if err != nil {
		t.Fatalf("parse auth URL: %v", err)
	}
	state := parsed.Query().Get("state")
	if state == "" {
		t.Fatal("external auth URL did not include state")
	}
	return state
}
