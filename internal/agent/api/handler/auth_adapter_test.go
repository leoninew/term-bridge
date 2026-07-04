package api

import (
	"context"

	sharedauth "termbridge-go/internal/shared/common/auth"
)

type testAuthService struct {
	tokens sharedauth.TokenService
}

func newTestAuthService(tokens sharedauth.TokenService) AuthService {
	return testAuthService{tokens: tokens}
}

func (s testAuthService) Login(ctx context.Context, email, password string) (AuthResult, error) {
	return AuthResult{}, ErrInvalidCredentials
}

func (s testAuthService) Capabilities() Capabilities {
	return Capabilities{Providers: []string{"email"}, AccountAuthEnabled: true}
}

func (s testAuthService) UserFromClaims(ctx context.Context, claims sharedauth.Claims) (UserView, error) {
	return UserView{ID: claims.Sub, Email: claims.Email, DisplayName: claims.Email, Provider: claims.Provider, EmailVerified: true}, nil
}

func (s testAuthService) VerifyToken(token string) (sharedauth.Claims, error) {
	return s.tokens.Verify(token)
}

func (s testAuthService) VerifyBasic(ctx context.Context, username, password string) bool {
	return false
}
