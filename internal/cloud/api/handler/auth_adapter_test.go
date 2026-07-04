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

func (s testAuthService) Register(ctx context.Context, email, password string) error {
	return ErrProviderUnsupported
}

func (s testAuthService) VerifyEmail(ctx context.Context, email, code string) error {
	return ErrProviderUnsupported
}

func (s testAuthService) ResendVerification(ctx context.Context, email string) error {
	return ErrProviderUnsupported
}

func (s testAuthService) ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	return ErrProviderUnsupported
}

func (s testAuthService) RequestPasswordReset(ctx context.Context, email string) error {
	return nil
}

func (s testAuthService) ConfirmPasswordReset(ctx context.Context, email, code, newPassword string) error {
	return ErrProviderUnsupported
}

func (s testAuthService) GoogleAuthURL(ctx context.Context) (string, error) {
	return "", ErrProviderUnsupported
}

func (s testAuthService) GoogleCallback(ctx context.Context, code, state string) (AuthResult, error) {
	return AuthResult{}, ErrProviderUnsupported
}

func (s testAuthService) IssueUserToken(ctx context.Context, userID string) (AuthResult, error) {
	email := userID + "@example.test"
	user := UserView{ID: userID, Email: email, DisplayName: email, Provider: "email", EmailVerified: true}
	token, err := s.tokens.Sign(sharedauth.Claims{Sub: user.ID, Email: user.Email, Provider: user.Provider})
	if err != nil {
		return AuthResult{}, err
	}
	return AuthResult{Token: token, User: user}, nil
}

func (s testAuthService) VerifyToken(token string) (sharedauth.Claims, error) {
	return s.tokens.Verify(token)
}

func (s testAuthService) VerifyBasic(ctx context.Context, username, password string) bool {
	return false
}
