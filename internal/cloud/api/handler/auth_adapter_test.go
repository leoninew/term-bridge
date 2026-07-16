package api

import (
	"context"

	cloudauth "gitee.com/leoninew/TermBridge-go/internal/cloud/model/user/auth"
	cloud "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/cloud/v1"
	sharedauth "gitee.com/leoninew/TermBridge-go/internal/shared/common/auth"
)

type testAuthService struct {
	tokens                  sharedauth.TokenService
	loginErr                error
	registerErr             error
	verifyEmailErr          error
	resendVerificationErr   error
	changePasswordErr       error
	requestPasswordResetErr error
	confirmPasswordResetErr error
}

func newTestAuthService(tokens sharedauth.TokenService) AuthService {
	return &testAuthService{tokens: tokens}
}

func (s *testAuthService) Login(ctx context.Context, email, password string) (*cloud.AuthLoginResp, error) {
	if s.loginErr != nil {
		return nil, s.loginErr
	}
	token, err := s.tokens.Sign(sharedauth.Claims{Sub: email, Email: email, Provider: "email"})
	if err != nil {
		return nil, err
	}
	return &cloud.AuthLoginResp{AccessToken: token, TokenType: "bearer"}, nil
}

func (s *testAuthService) UserFromClaims(ctx context.Context, claims sharedauth.Claims) (*cloud.User, error) {
	return &cloud.User{Id: claims.Sub, Email: claims.Email, DisplayName: claims.Email, Provider: claims.Provider, EmailVerified: true}, nil
}

func (s *testAuthService) Register(ctx context.Context, email, password string) error {
	if s.registerErr != nil {
		return s.registerErr
	}
	return cloudauth.ErrProviderUnsupported
}

func (s *testAuthService) VerifyEmail(ctx context.Context, email, code string) error {
	if s.verifyEmailErr != nil {
		return s.verifyEmailErr
	}
	return cloudauth.ErrProviderUnsupported
}

func (s *testAuthService) ResendVerification(ctx context.Context, email string) error {
	if s.resendVerificationErr != nil {
		return s.resendVerificationErr
	}
	return cloudauth.ErrProviderUnsupported
}

func (s *testAuthService) ChangePassword(ctx context.Context, userId, currentPassword, newPassword string) error {
	if s.changePasswordErr != nil {
		return s.changePasswordErr
	}
	return cloudauth.ErrProviderUnsupported
}

func (s *testAuthService) RequestPasswordReset(ctx context.Context, email string) error {
	return s.requestPasswordResetErr
}

func (s *testAuthService) ConfirmPasswordReset(ctx context.Context, email, code, newPassword string) error {
	if s.confirmPasswordResetErr != nil {
		return s.confirmPasswordResetErr
	}
	return cloudauth.ErrProviderUnsupported
}

func (s *testAuthService) ExternalAuthURL(ctx context.Context, providerId string) (string, error) {
	return "https://auth.example/" + providerId, nil
}

func (s *testAuthService) ExternalCallback(ctx context.Context, providerId, code, state string) (*cloud.AuthLoginResp, error) {
	return nil, cloudauth.ErrProviderUnsupported
}

func (s *testAuthService) IssueUserToken(ctx context.Context, userId string) (*cloud.CloudOAuthTokenResp, error) {
	email := userId + "@example.test"
	token, err := s.tokens.Sign(sharedauth.Claims{Sub: userId, Email: email, Provider: "email"})
	if err != nil {
		return nil, err
	}
	return &cloud.CloudOAuthTokenResp{AccessToken: token, TokenType: "bearer"}, nil
}

func (s *testAuthService) VerifyToken(token string) (sharedauth.Claims, error) {
	return s.tokens.Verify(token)
}
