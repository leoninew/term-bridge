package api

import (
	"context"

	cloudauth "gitee.com/leoninew/TermBridge-go/internal/cloud/model/user/auth"
	cloud "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/cloud/v1"
	sharedauth "gitee.com/leoninew/TermBridge-go/internal/shared/common/auth"
)

type testAuthService struct {
	tokens sharedauth.TokenService
}

func newTestAuthService(tokens sharedauth.TokenService) AuthService {
	return testAuthService{tokens: tokens}
}

func (s testAuthService) Login(ctx context.Context, email, password string) (*cloud.TokenResp, error) {
	return nil, cloudauth.ErrInvalidCredentials
}

func (s testAuthService) UserFromClaims(ctx context.Context, claims sharedauth.Claims) (*cloud.User, error) {
	return &cloud.User{Id: claims.Sub, Email: claims.Email, DisplayName: claims.Email, Provider: claims.Provider, EmailVerified: true}, nil
}

func (s testAuthService) Register(ctx context.Context, email, password string) error {
	return cloudauth.ErrProviderUnsupported
}

func (s testAuthService) VerifyEmail(ctx context.Context, email, code string) error {
	return cloudauth.ErrProviderUnsupported
}

func (s testAuthService) ResendVerification(ctx context.Context, email string) error {
	return cloudauth.ErrProviderUnsupported
}

func (s testAuthService) ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	return cloudauth.ErrProviderUnsupported
}

func (s testAuthService) RequestPasswordReset(ctx context.Context, email string) error {
	return nil
}

func (s testAuthService) ConfirmPasswordReset(ctx context.Context, email, code, newPassword string) error {
	return cloudauth.ErrProviderUnsupported
}

func (s testAuthService) GoogleAuthURL(ctx context.Context) (string, error) {
	return "", cloudauth.ErrProviderUnsupported
}

func (s testAuthService) GoogleCallback(ctx context.Context, code, state string) (*cloud.TokenResp, error) {
	return nil, cloudauth.ErrProviderUnsupported
}

func (s testAuthService) IssueUserToken(ctx context.Context, userID string) (*cloud.TokenResp, error) {
	email := userID + "@example.test"
	token, err := s.tokens.Sign(sharedauth.Claims{Sub: userID, Email: email, Provider: "email"})
	if err != nil {
		return nil, err
	}
	return &cloud.TokenResp{AccessToken: token, TokenType: "bearer"}, nil
}

func (s testAuthService) VerifyToken(token string) (sharedauth.Claims, error) {
	return s.tokens.Verify(token)
}

func (s testAuthService) VerifyBasic(ctx context.Context, username, password string) bool {
	return false
}
