package authapp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"termbridge-go/internal/infrastructure/config"
	authrepo "termbridge-go/internal/infrastructure/repository/auth"
	jwtauth "termbridge-go/internal/transport/http/gatewayapi/auth"
)

const (
	ProviderLocalAdmin       = "local_admin"
	ProviderLocalAccess      = "local_access"
	PurposeEmailVerification = "email_verification"
	PurposePasswordReset     = "password_reset"
)

var (
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrEmailNotVerified    = errors.New("email not verified")
	ErrEmailAlreadyUsed    = errors.New("email already used")
	ErrCodeInvalid         = errors.New("code invalid or expired")
	ErrCodeCooldown        = errors.New("code resend cooldown is active")
	ErrProviderUnsupported = errors.New("provider unsupported")
	ErrMailDisabled        = errors.New("mail sender is not configured")
	ErrOAuthDisabled       = errors.New("google oauth is not configured")
)

type EmailSender interface {
	Send(ctx context.Context, to, subject, html string) (EmailResult, error)
}

type EmailResult struct {
	MessageID    string
	Success      bool
	ResponseBody string
}

type GoogleUser struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
}

type GoogleClient interface {
	AuthCodeURL(state string) string
	ExchangeUser(ctx context.Context, code string) (GoogleUser, error)
}

type Service struct {
	repo       *authrepo.Repository
	tokens     jwtauth.TokenService
	cfg        config.AuthConfig
	mode       string
	sender     EmailSender
	google     GoogleClient
	setupStore *SetupTokenStore
	deviceID   string
	deviceName string
}

type UserView struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	DisplayName   string `json:"display_name"`
	Provider      string `json:"provider"`
	EmailVerified bool   `json:"email_verified"`
}

type Capabilities struct {
	Mode                     string   `json:"mode"`
	Providers                []string `json:"providers"`
	PasswordResetEnabled     bool     `json:"password_reset_enabled"`
	EmailVerificationEnabled bool     `json:"email_verification_enabled"`
}

type AuthResult struct {
	Token string
	User  UserView
}

func New(repo *authrepo.Repository, tokens jwtauth.TokenService, cfg config.AuthConfig, mode string, sender EmailSender, google GoogleClient) *Service {
	return &Service{repo: repo, tokens: tokens, cfg: cfg, mode: mode, sender: sender, google: google}
}

func (s *Service) WithLocalSetup(store *SetupTokenStore, deviceID string, deviceName string) *Service {
	s.setupStore = store
	s.deviceID = strings.TrimSpace(deviceID)
	s.deviceName = strings.TrimSpace(deviceName)
	return s
}

func (s *Service) Capabilities() Capabilities {
	providers := []string{"email"}
	if s.google != nil {
		providers = append(providers, "google")
	}
	if s.mode == "local" && s.cfg.LocalAdmin.Username != "" && s.cfg.LocalAdmin.Password != "" {
		providers = append(providers, ProviderLocalAdmin)
	}
	if s.mode == "local" && s.setupStore != nil {
		providers = append(providers, ProviderLocalAccess)
	}
	mail := s.sender != nil
	return Capabilities{Mode: s.mode, Providers: providers, PasswordResetEnabled: mail, EmailVerificationEnabled: mail}
}

func (s *Service) Register(ctx context.Context, email, password string) error {
	email = authrepo.NormalizeEmail(email)
	if email == "" {
		return ErrInvalidCredentials
	}
	if err := s.validatePassword(password); err != nil {
		return err
	}
	if _, err := s.repo.FindUserByEmail(ctx, email); err == nil {
		return ErrEmailAlreadyUsed
	} else if !authrepo.IsNotFound(err) {
		return err
	}
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	user, err := s.repo.CreateEmailUser(ctx, email, hash)
	if err != nil {
		return err
	}
	return s.sendCode(ctx, &user.ID, email, PurposeEmailVerification)
}

func (s *Service) ResendVerification(ctx context.Context, email string) error {
	user, err := s.repo.FindUserByEmail(ctx, email)
	if err != nil {
		return err
	}
	if user.Status == authrepo.StatusEnabled {
		return nil
	}
	return s.sendCode(ctx, &user.ID, user.EmailNormalized, PurposeEmailVerification)
}

func (s *Service) VerifyEmail(ctx context.Context, email, code string) error {
	codeRow, err := s.verifyCode(ctx, email, PurposeEmailVerification, code)
	if err != nil {
		return err
	}
	if !codeRow.UserID.Valid {
		return ErrCodeInvalid
	}
	if err := s.repo.MarkEmailVerified(ctx, codeRow.UserID.String); err != nil {
		return err
	}
	return s.repo.MarkCodeUsed(ctx, codeRow.ID)
}

func (s *Service) Login(ctx context.Context, email, password string) (AuthResult, error) {
	if s.mode == "local" && constantEqual(email, s.cfg.LocalAdmin.Username) && constantEqual(password, s.cfg.LocalAdmin.Password) {
		return s.sign(UserView{ID: "local-admin", Email: "", DisplayName: s.cfg.LocalAdmin.Username, Provider: ProviderLocalAdmin, EmailVerified: true})
	}
	user, err := s.repo.FindUserByEmail(ctx, email)
	if err != nil {
		return AuthResult{}, ErrInvalidCredentials
	}
	identity, err := s.repo.FindIdentityForUser(ctx, user.ID, authrepo.ProviderEmail)
	if err != nil || !identity.PasswordHash.Valid {
		return AuthResult{}, ErrInvalidCredentials
	}
	if !checkPassword(identity.PasswordHash.String, password) {
		return AuthResult{}, ErrInvalidCredentials
	}
	if user.Status != authrepo.StatusEnabled || !user.EmailVerifiedAt.Valid {
		return AuthResult{}, ErrEmailNotVerified
	}
	_ = s.repo.UpdateLastLogin(ctx, user.ID)
	return s.sign(userView(user, authrepo.ProviderEmail))
}

func (s *Service) VerifyBasic(ctx context.Context, username, password string) bool {
	_, err := s.Login(ctx, username, password)
	return err == nil
}

func (s *Service) CompleteLocalSetup(ctx context.Context, token string) (AuthResult, error) {
	if s.mode != "local" || s.setupStore == nil || s.deviceID == "" {
		return AuthResult{}, ErrInvalidCredentials
	}
	ok, err := s.setupStore.Use(token)
	if err != nil {
		return AuthResult{}, err
	}
	if !ok {
		return AuthResult{}, ErrInvalidCredentials
	}
	name := s.deviceName
	if name == "" {
		name = "Local device"
	}
	return s.sign(UserView{ID: "local:" + s.deviceID, DisplayName: name, Provider: ProviderLocalAccess, EmailVerified: true})
}

func (s *Service) HasAvailableSetupToken() (bool, error) {
	if s.mode != "local" || s.setupStore == nil {
		return false, nil
	}
	return s.setupStore.Available()
}

func (s *Service) ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	if userID == "local-admin" {
		return ErrProviderUnsupported
	}
	identity, err := s.repo.FindIdentityForUser(ctx, userID, authrepo.ProviderEmail)
	if err != nil || !identity.PasswordHash.Valid {
		return ErrProviderUnsupported
	}
	if !checkPassword(identity.PasswordHash.String, currentPassword) {
		return ErrInvalidCredentials
	}
	if err := s.validatePassword(newPassword); err != nil {
		return err
	}
	hash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}
	return s.repo.UpdatePassword(ctx, userID, hash)
}

func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	user, err := s.repo.FindUserByEmail(ctx, email)
	if err != nil {
		return nil
	}
	if user.Status != authrepo.StatusEnabled || !user.EmailVerifiedAt.Valid {
		return nil
	}
	if _, err := s.repo.FindIdentityForUser(ctx, user.ID, authrepo.ProviderEmail); err != nil {
		return nil
	}
	return s.sendCode(ctx, &user.ID, user.EmailNormalized, PurposePasswordReset)
}

func (s *Service) ConfirmPasswordReset(ctx context.Context, email, code, newPassword string) error {
	if err := s.validatePassword(newPassword); err != nil {
		return err
	}
	codeRow, err := s.verifyCode(ctx, email, PurposePasswordReset, code)
	if err != nil {
		return err
	}
	if !codeRow.UserID.Valid {
		return ErrCodeInvalid
	}
	hash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}
	if err := s.repo.UpdatePassword(ctx, codeRow.UserID.String, hash); err != nil {
		return err
	}
	return s.repo.MarkCodeUsed(ctx, codeRow.ID)
}

func (s *Service) GoogleAuthURL(ctx context.Context) (string, error) {
	if s.google == nil {
		return "", ErrOAuthDisabled
	}
	state, err := randomURLToken(32)
	if err != nil {
		return "", err
	}
	if err := s.repo.CreateOAuthState(ctx, state, time.Now().UTC().Add(10*time.Minute)); err != nil {
		return "", err
	}
	return s.google.AuthCodeURL(state), nil
}

func (s *Service) GoogleCallback(ctx context.Context, code, state string) (AuthResult, error) {
	if s.google == nil {
		return AuthResult{}, ErrOAuthDisabled
	}
	ok, err := s.repo.UseOAuthState(ctx, state)
	if err != nil {
		return AuthResult{}, err
	}
	if !ok {
		return AuthResult{}, ErrInvalidCredentials
	}
	googleUser, err := s.google.ExchangeUser(ctx, code)
	if err != nil {
		return AuthResult{}, err
	}
	if !googleUser.EmailVerified {
		return AuthResult{}, ErrInvalidCredentials
	}
	if strings.TrimSpace(googleUser.Subject) == "" || authrepo.NormalizeEmail(googleUser.Email) == "" {
		return AuthResult{}, ErrInvalidCredentials
	}
	if ident, err := s.repo.FindIdentity(ctx, authrepo.ProviderGoogle, googleUser.Subject); err == nil {
		user, err := s.repo.FindUserByID(ctx, ident.UserID)
		if err != nil {
			return AuthResult{}, err
		}
		_ = s.repo.UpdateLastLogin(ctx, user.ID)
		return s.sign(userView(user, authrepo.ProviderGoogle))
	} else if !authrepo.IsNotFound(err) {
		return AuthResult{}, err
	}
	if _, err := s.repo.FindUserByEmail(ctx, googleUser.Email); err == nil {
		return AuthResult{}, ErrEmailAlreadyUsed
	} else if !authrepo.IsNotFound(err) {
		return AuthResult{}, err
	}
	user, err := s.repo.CreateGoogleUser(ctx, googleUser.Email, googleUser.Subject)
	if err != nil {
		return AuthResult{}, err
	}
	return s.sign(userView(user, authrepo.ProviderGoogle))
}

func (s *Service) UserFromClaims(ctx context.Context, claims jwtauth.Claims) (UserView, error) {
	if claims.Provider == ProviderLocalAdmin && claims.Sub == "local-admin" {
		return UserView{ID: "local-admin", DisplayName: s.cfg.LocalAdmin.Username, Provider: ProviderLocalAdmin, EmailVerified: true}, nil
	}
	if claims.Provider == ProviderLocalAccess && strings.HasPrefix(claims.Sub, "local:") {
		name := s.deviceName
		if name == "" {
			name = "Local device"
		}
		return UserView{ID: claims.Sub, DisplayName: name, Provider: ProviderLocalAccess, EmailVerified: true}, nil
	}
	user, err := s.repo.FindUserByID(ctx, claims.Sub)
	if err != nil {
		return UserView{}, err
	}
	return userView(user, claims.Provider), nil
}

func (s *Service) VerifyToken(token string) (jwtauth.Claims, error) { return s.tokens.Verify(token) }

func (s *Service) sign(user UserView) (AuthResult, error) {
	token, err := s.tokens.Sign(jwtauth.Claims{Sub: user.ID, Email: user.Email, Provider: user.Provider})
	if err != nil {
		return AuthResult{}, err
	}
	return AuthResult{Token: token, User: user}, nil
}

func (s *Service) validatePassword(password string) error {
	if len(password) < s.cfg.PasswordPolicy.MinLength || len(password) > s.cfg.PasswordPolicy.MaxLength {
		return fmt.Errorf("password length must be between %d and %d", s.cfg.PasswordPolicy.MinLength, s.cfg.PasswordPolicy.MaxLength)
	}
	return nil
}

func (s *Service) sendCode(ctx context.Context, userID *string, email, purpose string) error {
	if s.sender == nil {
		return ErrMailDisabled
	}
	email = authrepo.NormalizeEmail(email)
	if latest, err := s.repo.LatestCode(ctx, email, purpose); err == nil && !latest.UsedAt.Valid && !latest.InvalidatedAt.Valid && time.Since(latest.CreatedAt) < s.cfg.Code.ResendCooldown {
		return ErrCodeCooldown
	}
	code, err := generateCode(s.cfg.Code.Length)
	if err != nil {
		return err
	}
	hash := hashCode(email, purpose, code)
	expires := time.Now().UTC().Add(s.cfg.Code.TTL)
	codeID, err := s.repo.CreateCode(ctx, userID, email, purpose, hash, expires)
	if err != nil {
		return err
	}
	subject := "TermBridge verification code"
	if purpose == PurposePasswordReset {
		subject = "TermBridge password reset code"
	}
	result, sendErr := s.sender.Send(ctx, email, subject, fmt.Sprintf("<p>Your TermBridge code is <strong>%s</strong>. It expires in 2 minutes.</p>", code))
	status := "success"
	if sendErr != nil || !result.Success {
		status = "failed"
		_ = s.repo.InvalidateCode(ctx, codeID)
	}
	_ = s.repo.SaveEmailLog(ctx, authrepo.EmailLog{EmailNormalized: email, Purpose: purpose, ProviderMessageID: result.MessageID, Status: status, ResponseBody: result.ResponseBody})
	if sendErr != nil {
		return sendErr
	}
	if !result.Success {
		return ErrMailDisabled
	}
	return nil
}

func (s *Service) verifyCode(ctx context.Context, email, purpose, input string) (authrepo.AuthCode, error) {
	row, err := s.repo.LatestCode(ctx, email, purpose)
	if err != nil {
		return authrepo.AuthCode{}, ErrCodeInvalid
	}
	now := time.Now().UTC()
	if row.UsedAt.Valid || row.InvalidatedAt.Valid || row.ExpiresAt.Before(now) || row.Attempts >= s.cfg.Code.MaxAttempts {
		return authrepo.AuthCode{}, ErrCodeInvalid
	}
	if hashCode(email, purpose, strings.ToUpper(strings.TrimSpace(input))) != row.CodeHash {
		_ = s.repo.IncrementCodeAttempts(ctx, row.ID)
		return authrepo.AuthCode{}, ErrCodeInvalid
	}
	return row, nil
}

func userView(user authrepo.User, provider string) UserView {
	return UserView{ID: user.ID, Email: user.EmailNormalized, DisplayName: user.DisplayName, Provider: provider, EmailVerified: user.EmailVerifiedAt.Valid}
}

func hashPassword(password string) (string, error) {
	data, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(data), err
}

func checkPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func generateCode(length int) (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	buf := make([]byte, length)
	random := make([]byte, length)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	for i, b := range random {
		buf[i] = charset[int(b)%len(charset)]
	}
	return string(buf), nil
}

func hashCode(email, purpose, code string) string {
	sum := sha256.Sum256([]byte(authrepo.NormalizeEmail(email) + ":" + purpose + ":" + strings.ToUpper(strings.TrimSpace(code))))
	return hex.EncodeToString(sum[:])
}

func randomURLToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func constantEqual(a, b string) bool {
	return a != "" && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

type OAuthGoogleClient struct{ cfg *oauth2.Config }

func NewOAuthGoogleClient(cfg config.GoogleConfig) GoogleClient {
	if cfg.ClientID == "" || cfg.ClientSecret == "" || cfg.RedirectURL == "" {
		return nil
	}
	return &OAuthGoogleClient{cfg: &oauth2.Config{ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret, RedirectURL: cfg.RedirectURL, Scopes: []string{"openid", "email", "profile"}, Endpoint: google.Endpoint}}
}

func (g *OAuthGoogleClient) AuthCodeURL(state string) string {
	return g.cfg.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

func (g *OAuthGoogleClient) ExchangeUser(ctx context.Context, code string) (GoogleUser, error) {
	token, err := g.cfg.Exchange(ctx, code)
	if err != nil {
		return GoogleUser{}, err
	}
	client := g.cfg.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		return GoogleUser{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return GoogleUser{}, fmt.Errorf("google userinfo status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return GoogleUser{}, err
	}
	var payload struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return GoogleUser{}, err
	}
	return GoogleUser{Subject: payload.Sub, Email: payload.Email, EmailVerified: payload.EmailVerified, Name: payload.Name}, nil
}

func AuthURLFromBase(base string, params map[string]string) string {
	u, _ := url.Parse(base)
	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func IgnoreNotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	return err
}
