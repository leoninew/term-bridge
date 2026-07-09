package api

import (
	"context"
	"errors"
	"os"
	osuser "os/user"
	"strings"

	cloud "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/cloud/v1"
	sharedauth "gitee.com/leoninew/TermBridge-go/internal/shared/common/auth"
)

const ProviderLocalAdmin = "local_admin"

var ErrInvalidCredentials = errors.New("invalid credentials")

type localAuthService struct {
	tokens sharedauth.TokenService
}

func NewLocalAuthService(tokens sharedauth.TokenService) AuthService {
	return localAuthService{tokens: tokens}
}

func (s localAuthService) Login(ctx context.Context, email, password string) (*cloud.LocalAuthLoginResp, error) {
	_, _, _ = ctx, email, password
	return s.sign(s.localUser())
}

func (s localAuthService) UserFromClaims(ctx context.Context, claims sharedauth.Claims) (*cloud.User, error) {
	_ = ctx
	if claims.Provider == ProviderLocalAdmin && claims.Sub == "local-agent" {
		return s.localUser(), nil
	}
	return nil, ErrInvalidCredentials
}

func (s localAuthService) VerifyToken(token string) (sharedauth.Claims, error) {
	return s.tokens.Verify(token)
}

func (s localAuthService) sign(user *cloud.User) (*cloud.LocalAuthLoginResp, error) {
	token, err := s.tokens.Sign(sharedauth.Claims{Sub: user.GetId(), Email: user.GetEmail(), Provider: user.GetProvider()})
	if err != nil {
		return nil, err
	}
	return &cloud.LocalAuthLoginResp{AccessToken: token, TokenType: "bearer"}, nil
}

func (s localAuthService) localUser() *cloud.User {
	displayName := currentOSUsername()
	if displayName == "" {
		displayName = "TermBridge Agent"
	}
	return &cloud.User{Id: "local-agent", DisplayName: displayName, Provider: ProviderLocalAdmin, EmailVerified: true}
}

func currentOSUsername() string {
	if current, err := osuser.Current(); err == nil {
		if username := strings.TrimSpace(current.Username); username != "" {
			return username
		}
	}
	for _, key := range []string{"USERNAME", "USER"} {
		if username := strings.TrimSpace(os.Getenv(key)); username != "" {
			return username
		}
	}
	return ""
}
