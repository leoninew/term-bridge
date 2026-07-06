package api

import (
	"context"
	"os"
	osuser "os/user"
	"strings"

	sharedauth "termbridge/internal/shared/common/auth"
)

type localAuthService struct {
	tokens sharedauth.TokenService
}

func NewLocalAuthService(tokens sharedauth.TokenService) AuthService {
	return localAuthService{tokens: tokens}
}

func (s localAuthService) Login(ctx context.Context, email, password string) (AuthResult, error) {
	_, _, _ = ctx, email, password
	return s.sign(s.localUser())
}

func (s localAuthService) UserFromClaims(ctx context.Context, claims sharedauth.Claims) (UserView, error) {
	_ = ctx
	if claims.Provider == ProviderLocalAdmin && claims.Sub == "local-agent" {
		return s.localUser(), nil
	}
	return UserView{}, ErrInvalidCredentials
}

func (s localAuthService) VerifyToken(token string) (sharedauth.Claims, error) {
	return s.tokens.Verify(token)
}

func (s localAuthService) sign(user UserView) (AuthResult, error) {
	token, err := s.tokens.Sign(sharedauth.Claims{Sub: user.Id, Email: user.Email, Provider: user.Provider})
	if err != nil {
		return AuthResult{}, err
	}
	return AuthResult{Token: token, User: user}, nil
}

func (s localAuthService) localUser() UserView {
	displayName := currentOSUsername()
	if displayName == "" {
		displayName = "TermBridge Agent"
	}
	return UserView{Id: "local-agent", DisplayName: displayName, Provider: ProviderLocalAdmin, EmailVerified: true}
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
