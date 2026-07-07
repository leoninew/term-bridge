package api

import (
	"context"

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
	return nil, ErrInvalidCredentials
}

func (s testAuthService) UserFromClaims(ctx context.Context, claims sharedauth.Claims) (*cloud.User, error) {
	return &cloud.User{Id: claims.Sub, Email: claims.Email, DisplayName: claims.Email, Provider: claims.Provider, EmailVerified: true}, nil
}

func (s testAuthService) VerifyToken(token string) (sharedauth.Claims, error) {
	return s.tokens.Verify(token)
}

func (s testAuthService) VerifyBasic(ctx context.Context, username, password string) bool {
	return false
}
