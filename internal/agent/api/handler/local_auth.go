package api

import (
	"context"
	"crypto/subtle"

	sharedauth "termbridge-go/internal/shared/common/auth"
)

type LocalAuthConfig struct {
	Username string
	Password string
}

type localAuthService struct {
	cfg    LocalAuthConfig
	tokens sharedauth.TokenService
}

func NewLocalAuthService(cfg LocalAuthConfig, tokens sharedauth.TokenService) AuthService {
	return localAuthService{cfg: cfg, tokens: tokens}
}

func (s localAuthService) Login(ctx context.Context, email, password string) (AuthResult, error) {
	_ = ctx
	if constantEqual(email, s.cfg.Username) && constantEqual(password, s.cfg.Password) {
		return s.sign(UserView{ID: "local-admin", Email: "", DisplayName: s.cfg.Username, Provider: ProviderLocalAdmin, EmailVerified: true})
	}
	return AuthResult{}, ErrInvalidCredentials
}

func (s localAuthService) Capabilities() Capabilities {
	return Capabilities{Providers: []string{}, AccountAuthEnabled: false, CloudOAuthEnabled: false}
}

func (s localAuthService) UserFromClaims(ctx context.Context, claims sharedauth.Claims) (UserView, error) {
	_ = ctx
	if claims.Provider == ProviderLocalAdmin && claims.Sub == "local-admin" {
		return UserView{ID: "local-admin", DisplayName: s.cfg.Username, Provider: ProviderLocalAdmin, EmailVerified: true}, nil
	}
	return UserView{}, ErrInvalidCredentials
}

func (s localAuthService) VerifyToken(token string) (sharedauth.Claims, error) {
	return s.tokens.Verify(token)
}

func (s localAuthService) sign(user UserView) (AuthResult, error) {
	token, err := s.tokens.Sign(sharedauth.Claims{Sub: user.ID, Email: user.Email, Provider: user.Provider})
	if err != nil {
		return AuthResult{}, err
	}
	return AuthResult{Token: token, User: user}, nil
}

func constantEqual(left, right string) bool {
	if len(left) != len(right) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}
