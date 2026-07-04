package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
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

	authmodel "termbridge-go/internal/cloud/model/user/auth"
	"termbridge-go/internal/cloud/repository/user/auth"
	sharedauth "termbridge-go/internal/shared/common/auth"
)

const (
	PurposeEmailVerification = "email_verification"
	PurposePasswordReset     = "password_reset"
)

var (
	ErrMailDisabled  = errors.New("mail sender is not configured")
	ErrOAuthDisabled = errors.New("google oauth is not configured")
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

type Config struct {
	PasswordPolicy PasswordPolicy
	Code           CodePolicy
}

type PasswordPolicy struct {
	MinLength int
	MaxLength int
}

type CodePolicy struct {
	Length         int
	TTL            time.Duration
	ResendCooldown time.Duration
	MaxAttempts    int
}

type Service struct {
	repo   *repository.Repository
	tokens sharedauth.TokenService
	cfg    Config
	sender EmailSender
	google GoogleClient
}

func New(repo *repository.Repository, tokens sharedauth.TokenService, cfg Config, sender EmailSender, google GoogleClient) *Service {
	return &Service{repo: repo, tokens: tokens, cfg: cfg, sender: sender, google: google}
}

func (s *Service) Capabilities() authmodel.Capabilities {
	providers := []string{"email"}
	if s.google != nil {
		providers = append(providers, "google")
	}
	mail := s.sender != nil
	return authmodel.Capabilities{Providers: providers, PasswordResetEnabled: mail, EmailVerificationEnabled: mail, AccountAuthEnabled: true, CloudOAuthEnabled: false}
}

func (s *Service) Register(ctx context.Context, email, password string) error {
	if err := s.requireRepository(); err != nil {
		return err
	}
	email = repository.NormalizeEmail(email)
	if email == "" {
		return authmodel.ErrInvalidCredentials
	}
	if err := s.validatePassword(password); err != nil {
		return err
	}
	if _, err := s.repo.FindUserByEmail(ctx, email); err == nil {
		return authmodel.ErrEmailAlreadyUsed
	} else if !repository.IsNotFound(err) {
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
	if err := s.requireRepository(); err != nil {
		return err
	}
	user, err := s.repo.FindUserByEmail(ctx, email)
	if err != nil {
		return err
	}
	if user.Status == repository.StatusEnabled {
		return nil
	}
	return s.sendCode(ctx, &user.ID, user.EmailNormalized, PurposeEmailVerification)
}

func (s *Service) VerifyEmail(ctx context.Context, email, code string) error {
	if err := s.requireRepository(); err != nil {
		return err
	}
	codeRow, err := s.verifyCode(ctx, email, PurposeEmailVerification, code)
	if err != nil {
		return err
	}
	if !codeRow.UserId.Valid {
		return authmodel.ErrCodeInvalid
	}
	if err := s.repo.MarkEmailVerified(ctx, codeRow.UserId.String); err != nil {
		return err
	}
	return s.repo.MarkCodeUsed(ctx, codeRow.ID)
}

func (s *Service) Login(ctx context.Context, email, password string) (authmodel.Result, error) {
	if err := s.requireRepository(); err != nil {
		return authmodel.Result{}, authmodel.ErrInvalidCredentials
	}
	user, err := s.repo.FindUserByEmail(ctx, email)
	if err != nil {
		return authmodel.Result{}, authmodel.ErrInvalidCredentials
	}
	identity, err := s.repo.FindIdentityForUser(ctx, user.ID, repository.ProviderEmail)
	if err != nil || !identity.PasswordHash.Valid {
		return authmodel.Result{}, authmodel.ErrInvalidCredentials
	}
	if !checkPassword(identity.PasswordHash.String, password) {
		return authmodel.Result{}, authmodel.ErrInvalidCredentials
	}
	if user.Status != repository.StatusEnabled || !user.EmailVerifiedAt.Valid {
		return authmodel.Result{}, authmodel.ErrEmailNotVerified
	}
	_ = s.repo.UpdateLastLogin(ctx, user.ID)
	return s.sign(userView(user, repository.ProviderEmail))
}

func (s *Service) VerifyBasic(ctx context.Context, username, password string) bool {
	_, err := s.Login(ctx, username, password)
	return err == nil
}

func (s *Service) ChangePassword(ctx context.Context, userId, currentPassword, newPassword string) error {
	if err := s.requireRepository(); err != nil {
		return err
	}
	identity, err := s.repo.FindIdentityForUser(ctx, userId, repository.ProviderEmail)
	if err != nil || !identity.PasswordHash.Valid {
		return authmodel.ErrProviderUnsupported
	}
	if !checkPassword(identity.PasswordHash.String, currentPassword) {
		return authmodel.ErrInvalidCredentials
	}
	if err := s.validatePassword(newPassword); err != nil {
		return err
	}
	hash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}
	return s.repo.UpdatePassword(ctx, userId, hash)
}

func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	if err := s.requireRepository(); err != nil {
		return nil
	}
	user, err := s.repo.FindUserByEmail(ctx, email)
	if err != nil {
		return nil
	}
	if user.Status != repository.StatusEnabled || !user.EmailVerifiedAt.Valid {
		return nil
	}
	if _, err := s.repo.FindIdentityForUser(ctx, user.ID, repository.ProviderEmail); err != nil {
		return nil
	}
	return s.sendCode(ctx, &user.ID, user.EmailNormalized, PurposePasswordReset)
}

func (s *Service) ConfirmPasswordReset(ctx context.Context, email, code, newPassword string) error {
	if err := s.requireRepository(); err != nil {
		return err
	}
	if err := s.validatePassword(newPassword); err != nil {
		return err
	}
	codeRow, err := s.verifyCode(ctx, email, PurposePasswordReset, code)
	if err != nil {
		return err
	}
	if !codeRow.UserId.Valid {
		return authmodel.ErrCodeInvalid
	}
	hash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}
	if err := s.repo.UpdatePassword(ctx, codeRow.UserId.String, hash); err != nil {
		return err
	}
	return s.repo.MarkCodeUsed(ctx, codeRow.ID)
}

func (s *Service) GoogleAuthURL(ctx context.Context) (string, error) {
	if err := s.requireRepository(); err != nil {
		return "", err
	}
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

func (s *Service) GoogleCallback(ctx context.Context, code, state string) (authmodel.Result, error) {
	if err := s.requireRepository(); err != nil {
		return authmodel.Result{}, err
	}
	if s.google == nil {
		return authmodel.Result{}, ErrOAuthDisabled
	}
	ok, err := s.repo.UseOAuthState(ctx, state)
	if err != nil {
		return authmodel.Result{}, err
	}
	if !ok {
		return authmodel.Result{}, authmodel.ErrInvalidCredentials
	}
	googleUser, err := s.google.ExchangeUser(ctx, code)
	if err != nil {
		return authmodel.Result{}, err
	}
	if !googleUser.EmailVerified {
		return authmodel.Result{}, authmodel.ErrInvalidCredentials
	}
	if strings.TrimSpace(googleUser.Subject) == "" || repository.NormalizeEmail(googleUser.Email) == "" {
		return authmodel.Result{}, authmodel.ErrInvalidCredentials
	}
	if ident, err := s.repo.FindIdentity(ctx, repository.ProviderGoogle, googleUser.Subject); err == nil {
		user, err := s.repo.FindUserByID(ctx, ident.UserId)
		if err != nil {
			return authmodel.Result{}, err
		}
		_ = s.repo.UpdateLastLogin(ctx, user.ID)
		return s.sign(userView(user, repository.ProviderGoogle))
	} else if !repository.IsNotFound(err) {
		return authmodel.Result{}, err
	}
	if _, err := s.repo.FindUserByEmail(ctx, googleUser.Email); err == nil {
		return authmodel.Result{}, authmodel.ErrEmailAlreadyUsed
	} else if !repository.IsNotFound(err) {
		return authmodel.Result{}, err
	}
	user, err := s.repo.CreateGoogleUser(ctx, googleUser.Email, googleUser.Subject)
	if err != nil {
		return authmodel.Result{}, err
	}
	return s.sign(userView(user, repository.ProviderGoogle))
}

func (s *Service) IssueUserToken(ctx context.Context, userId string) (authmodel.Result, error) {
	if err := s.requireRepository(); err != nil {
		return authmodel.Result{}, err
	}
	user, err := s.repo.FindUserByID(ctx, userId)
	if err != nil {
		return authmodel.Result{}, err
	}
	return s.sign(userView(user, repository.ProviderEmail))
}

func (s *Service) UserFromClaims(ctx context.Context, claims sharedauth.Claims) (authmodel.UserView, error) {
	if err := s.requireRepository(); err != nil {
		return authmodel.UserView{}, err
	}
	user, err := s.repo.FindUserByID(ctx, claims.Sub)
	if err != nil {
		return authmodel.UserView{}, err
	}
	return userView(user, claims.Provider), nil
}

func (s *Service) VerifyToken(token string) (sharedauth.Claims, error) {
	return s.tokens.Verify(token)
}

func (s *Service) requireRepository() error {
	if s.repo == nil {
		return authmodel.ErrProviderUnsupported
	}
	return nil
}

func (s *Service) sign(user authmodel.UserView) (authmodel.Result, error) {
	token, err := s.tokens.Sign(sharedauth.Claims{Sub: user.ID, Email: user.Email, Provider: user.Provider})
	if err != nil {
		return authmodel.Result{}, err
	}
	return authmodel.Result{Token: token, User: user}, nil
}

func (s *Service) validatePassword(password string) error {
	if len(password) < s.cfg.PasswordPolicy.MinLength || len(password) > s.cfg.PasswordPolicy.MaxLength {
		return fmt.Errorf("password length must be between %d and %d", s.cfg.PasswordPolicy.MinLength, s.cfg.PasswordPolicy.MaxLength)
	}
	return nil
}

func (s *Service) sendCode(ctx context.Context, userId *string, email, purpose string) error {
	if err := s.requireRepository(); err != nil {
		return err
	}
	if s.sender == nil {
		return ErrMailDisabled
	}
	email = repository.NormalizeEmail(email)
	if latest, err := s.repo.LatestCode(ctx, email, purpose); err == nil && !latest.UsedAt.Valid && !latest.InvalidatedAt.Valid && time.Since(latest.CreatedAt) < s.cfg.Code.ResendCooldown {
		return authmodel.ErrCodeCooldown
	}
	code, err := generateCode(s.cfg.Code.Length)
	if err != nil {
		return err
	}
	hash := hashCode(email, purpose, code)
	expires := time.Now().UTC().Add(s.cfg.Code.TTL)
	codeID, err := s.repo.CreateCode(ctx, userId, email, purpose, hash, expires)
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
	_ = s.repo.SaveEmailLog(ctx, repository.EmailLog{EmailNormalized: email, Purpose: purpose, ProviderMessageID: result.MessageID, Status: status, ResponseBody: result.ResponseBody})
	if sendErr != nil {
		return sendErr
	}
	if !result.Success {
		return ErrMailDisabled
	}
	return nil
}

func (s *Service) verifyCode(ctx context.Context, email, purpose, input string) (repository.AuthCode, error) {
	if err := s.requireRepository(); err != nil {
		return repository.AuthCode{}, authmodel.ErrCodeInvalid
	}
	row, err := s.repo.LatestCode(ctx, email, purpose)
	if err != nil {
		return repository.AuthCode{}, authmodel.ErrCodeInvalid
	}
	now := time.Now().UTC()
	if row.UsedAt.Valid || row.InvalidatedAt.Valid || row.ExpiresAt.Before(now) || row.Attempts >= s.cfg.Code.MaxAttempts {
		return repository.AuthCode{}, authmodel.ErrCodeInvalid
	}
	if hashCode(email, purpose, strings.ToUpper(strings.TrimSpace(input))) != row.CodeHash {
		_ = s.repo.IncrementCodeAttempts(ctx, row.ID)
		return repository.AuthCode{}, authmodel.ErrCodeInvalid
	}
	return row, nil
}

func userView(user repository.User, provider string) authmodel.UserView {
	return authmodel.UserView{ID: user.ID, Email: user.EmailNormalized, DisplayName: user.DisplayName, Provider: provider, EmailVerified: user.EmailVerifiedAt.Valid}
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
	sum := sha256.Sum256([]byte(repository.NormalizeEmail(email) + ":" + purpose + ":" + strings.ToUpper(strings.TrimSpace(code))))
	return hex.EncodeToString(sum[:])
}

func randomURLToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

type OAuthGoogleClient struct{ cfg *oauth2.Config }

type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

func NewOAuthGoogleClient(cfg GoogleConfig) GoogleClient {
	if strings.TrimSpace(cfg.ClientID) == "" {
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
	defer func() { _ = resp.Body.Close() }()
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
