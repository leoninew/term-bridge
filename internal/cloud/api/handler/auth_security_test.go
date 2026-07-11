package api

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	cloud "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/cloud/v1"
	sharedauth "gitee.com/leoninew/TermBridge-go/internal/shared/common/auth"
)

func testLogger() *slog.Logger {
	return slog.Default()
}

type countingAuthService struct {
	loginCalls    atomic.Int32
	registerCalls atomic.Int32
}

func (s *countingAuthService) Login(context.Context, string, string) (*cloud.AuthLoginResp, error) {
	s.loginCalls.Add(1)
	return &cloud.AuthLoginResp{AccessToken: "user-token", TokenType: "bearer"}, nil
}

func (s *countingAuthService) Register(context.Context, string, string) error {
	s.registerCalls.Add(1)
	return nil
}

func (s *countingAuthService) UserFromClaims(context.Context, sharedauth.Claims) (*cloud.User, error) {
	return nil, nil
}
func (s *countingAuthService) VerifyEmail(context.Context, string, string) error { return nil }
func (s *countingAuthService) ResendVerification(context.Context, string) error  { return nil }
func (s *countingAuthService) ChangePassword(context.Context, string, string, string) error {
	return nil
}
func (s *countingAuthService) RequestPasswordReset(context.Context, string) error { return nil }
func (s *countingAuthService) ConfirmPasswordReset(context.Context, string, string, string) error {
	return nil
}
func (s *countingAuthService) GoogleAuthURL(context.Context) (string, error) { return "", nil }
func (s *countingAuthService) GoogleCallback(context.Context, string, string) (*cloud.AuthGoogleCallbackResp, error) {
	return nil, nil
}
func (s *countingAuthService) IssueUserToken(context.Context, string) (*cloud.CloudOAuthTokenResp, error) {
	return nil, nil
}
func (s *countingAuthService) VerifyToken(string) (sharedauth.Claims, error) {
	return sharedauth.Claims{}, nil
}
func (s *countingAuthService) VerifyBasic(context.Context, string, string) bool { return false }

type rejectingTurnstileVerifier struct{}

func (rejectingTurnstileVerifier) Verify(context.Context, string, string) error {
	return errors.New("verification rejected")
}

func TestLoginSecurityRejectsInvalidCSRFBeforeCredentialAuthentication(t *testing.T) {
	service := &countingAuthService{}
	turnstile, csrf := testAuthSecurityConfig()
	handler := New(Config{JWTSecret: testJWTKey, Logger: testLogger(), AuthService: service, Turnstile: turnstile, CSRF: csrf})

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/cloud-api/auth/login", bytes.NewBufferString(`{"email":"user@example.test","password":"password","turnstile_token":"challenge","csrf_token":"invalid"}`)))

	assertAPIError(t, response, http.StatusBadRequest, errorCodeBadRequest)
	if service.loginCalls.Load() != 0 {
		t.Fatalf("credential authentication ran %d times after invalid CSRF", service.loginCalls.Load())
	}
}

func TestLoginSecurityConsumesCSRFTokenAndRequiresTurnstile(t *testing.T) {
	service := &countingAuthService{}
	_, csrf := testAuthSecurityConfig()
	handler := New(Config{JWTSecret: testJWTKey, Logger: testLogger(), AuthService: service, Turnstile: TurnstileConfig{Verify: rejectingTurnstileVerifier{}}, CSRF: csrf})
	csrfToken, err := handler.config.CSRF.Tokens.Issue()
	if err != nil {
		t.Fatalf("issue csrf token: %v", err)
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/cloud-api/auth/login", bytes.NewBufferString(`{"email":"user@example.test","password":"password","turnstile_token":"","csrf_token":"`+csrfToken+`"}`)))

	assertAPIError(t, response, http.StatusBadRequest, errorCodeBadRequest)
	if service.loginCalls.Load() != 0 {
		t.Fatalf("credential authentication ran %d times after missing Turnstile token", service.loginCalls.Load())
	}
	if handler.config.CSRF.Tokens.Consume(csrfToken) {
		t.Fatal("CSRF token remained usable after a login attempt")
	}
}

func TestRegistrationSecurityRejectsChallengeBeforeAccountCreation(t *testing.T) {
	service := &countingAuthService{}
	_, csrf := testAuthSecurityConfig()
	handler := New(Config{JWTSecret: testJWTKey, Logger: testLogger(), AuthService: service, Turnstile: TurnstileConfig{Verify: rejectingTurnstileVerifier{}}, CSRF: csrf})

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/cloud-api/auth/register", bytes.NewBufferString(`{"email":"user@example.test","password":"password","turnstile_token":"challenge"}`)))

	assertAPIError(t, response, http.StatusBadRequest, errorCodeBadRequest)
	if service.registerCalls.Load() != 0 {
		t.Fatalf("account creation ran %d times after rejected Turnstile challenge", service.registerCalls.Load())
	}
}

func TestAuthSecurityEndpointsExposeOnlyPublicValues(t *testing.T) {
	turnstile, csrf := testAuthSecurityConfig()
	turnstile.SiteKey = "public-site-key"
	turnstile.SecretKey = "private-secret"
	handler := New(Config{JWTSecret: testJWTKey, Logger: testLogger(), Turnstile: turnstile, CSRF: csrf})

	configResponse := httptest.NewRecorder()
	handler.ServeHTTP(configResponse, httptest.NewRequest(http.MethodGet, "/cloud-api/auth/turnstile/config", nil))
	if configResponse.Code != http.StatusOK || !bytes.Contains(configResponse.Body.Bytes(), []byte("public-site-key")) || bytes.Contains(configResponse.Body.Bytes(), []byte("private-secret")) {
		t.Fatalf("Turnstile config response = %d %s", configResponse.Code, configResponse.Body.String())
	}

	csrfResponse := httptest.NewRecorder()
	handler.ServeHTTP(csrfResponse, httptest.NewRequest(http.MethodGet, "/cloud-api/auth/login/csrf", nil))
	if csrfResponse.Code != http.StatusOK || csrfResponse.Header().Get("Cache-Control") != "no-store" || !bytes.Contains(csrfResponse.Body.Bytes(), []byte(`"token"`)) {
		t.Fatalf("CSRF response = %d headers=%v body=%s", csrfResponse.Code, csrfResponse.Header(), csrfResponse.Body.String())
	}
}
