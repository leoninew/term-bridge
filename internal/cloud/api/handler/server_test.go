package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	cloud "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/cloud/v1"
	sharedauth "gitee.com/leoninew/TermBridge-go/internal/shared/common/auth"
	"gitee.com/leoninew/TermBridge-go/internal/shared/common/utils/codec"
)

type testErrorResponse struct {
	Code      string `json:"code"`
	Error     string `json:"error"`
	RequestId string `json:"request_id"`
}

func testCloudConfig() Config {
	return Config{Username: "admin", Password: "admin", JWTSecret: testJWTKey, Logger: slog.Default()}
}

func TestHealth(t *testing.T) {
	server := New(testCloudConfig())
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/cloud-api/health", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if !bytes.Contains(response.Body.Bytes(), []byte(`"status":"ok"`)) {
		t.Fatalf("body = %s", response.Body.String())
	}
}

func TestAuthEndpoints(t *testing.T) {
	server := New(testCloudConfig())
	devicesResponse := httptest.NewRecorder()
	server.ServeHTTP(devicesResponse, httptest.NewRequest(http.MethodGet, "/cloud-api/devices", nil))
	if devicesResponse.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated devices status = %d, want 401", devicesResponse.Code)
	}
	assertAPIError(t, devicesResponse, http.StatusUnauthorized, errorCodeUnauthorized)

	loginResponse := httptest.NewRecorder()
	server.ServeHTTP(loginResponse, httptest.NewRequest(http.MethodPost, "/cloud-api/auth/login", bytes.NewBufferString(`{"username":"admin","password":"admin"}`)))
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200; body=%s", loginResponse.Code, loginResponse.Body.String())
	}
	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if err := json.Unmarshal(loginResponse.Body.Bytes(), &tokenResp); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	if tokenResp.AccessToken == "" {
		t.Fatal("access_token is empty")
	}

	meRequest := httptest.NewRequest(http.MethodGet, "/cloud-api/auth/me", nil)
	meRequest.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
	meResponse := httptest.NewRecorder()
	server.ServeHTTP(meResponse, meRequest)
	if meResponse.Code != http.StatusOK {
		t.Fatalf("me status = %d, want 200; body=%s", meResponse.Code, meResponse.Body.String())
	}

	logoutRequest := httptest.NewRequest(http.MethodPost, "/cloud-api/auth/logout", nil)
	logoutRequest.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
	logoutResponse := httptest.NewRecorder()
	server.ServeHTTP(logoutResponse, logoutRequest)
	if logoutResponse.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, want 204", logoutResponse.Code)
	}

	devicesRequestAfterExpired := httptest.NewRequest(http.MethodGet, "/cloud-api/devices", nil)
	devicesRequestAfterExpired.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
	devicesResponseAfterExpired := httptest.NewRecorder()
	server.ServeHTTP(devicesResponseAfterExpired, devicesRequestAfterExpired)
	if devicesResponseAfterExpired.Code != http.StatusOK {
		t.Fatalf("devices with valid token status = %d, want 200", devicesResponseAfterExpired.Code)
	}
	var devicesBody cloud.ListDevicesResp
	if err := codec.UnmarshalProtoJSON(devicesResponseAfterExpired.Body.Bytes(), &devicesBody); err != nil {
		t.Fatalf("decode devices response: %v; body=%s", err, devicesResponseAfterExpired.Body.String())
	}
	if len(devicesBody.GetItems()) != 0 {
		t.Fatalf("devices count = %d, want 0", len(devicesBody.GetItems()))
	}
}

func TestOAuthAuthorizationCodeCanBeExchangedOnce(t *testing.T) {
	tokens := sharedauth.NewTokenService(testJWTKey)
	server := New(Config{
		JWTSecret:   testJWTKey,
		Logger:      slog.Default(),
		AuthService: newTestAuthService(tokens),
		CloudOAuth: CloudOAuthConfig{
			Clients: []CloudOAuthClientConfig{{
				ClientId:     "termbridge-agent",
				ClientSecret: "agent-secret",
				RedirectUrl:  "http://127.0.0.1:9033/oauth/callback",
				Scopes:       []string{"openid", "email", "profile"},
			}},
		},
	})
	userToken, err := tokens.Sign(sharedauth.Claims{Sub: "user-1", Email: "user-1@example.test", Provider: "email"})
	if err != nil {
		t.Fatalf("sign user token: %v", err)
	}
	callbackUrl := "http://127.0.0.1:9033/oauth/callback"
	authorizeValues := url.Values{}
	authorizeValues.Set("response_type", "code")
	authorizeValues.Set("client_id", "termbridge-agent")
	authorizeValues.Set("redirect_uri", callbackUrl)
	authorizeValues.Set("state", "state-1")
	authorizeRequest := httptest.NewRequest(http.MethodPost, "/cloud-api/oauth2/authorize?"+authorizeValues.Encode(), nil)
	authorizeRequest.Header.Set("Authorization", "Bearer "+userToken)
	authorizeResponse := httptest.NewRecorder()
	server.ServeHTTP(authorizeResponse, authorizeRequest)
	if authorizeResponse.Code != http.StatusOK {
		t.Fatalf("authorize status = %d, want 200; body=%s", authorizeResponse.Code, authorizeResponse.Body.String())
	}
	var authorizeBody struct {
		RedirectUrl string `json:"redirect_url"`
	}
	if err := json.Unmarshal(authorizeResponse.Body.Bytes(), &authorizeBody); err != nil {
		t.Fatalf("decode authorize response: %v", err)
	}
	callback, err := url.Parse(authorizeBody.RedirectUrl)
	if err != nil {
		t.Fatalf("parse callback redirect: %v", err)
	}
	if callback.Query().Get("state") != "state-1" {
		t.Fatalf("callback state = %q, want original state", callback.Query().Get("state"))
	}
	code := callback.Query().Get("code")
	if code == "" {
		t.Fatal("authorization code is empty")
	}
	tokenForm := url.Values{}
	tokenForm.Set("grant_type", "authorization_code")
	tokenForm.Set("client_id", "termbridge-agent")
	tokenForm.Set("client_secret", "agent-secret")
	tokenForm.Set("redirect_uri", callbackUrl)
	tokenForm.Set("code", code)
	tokenRequest := httptest.NewRequest(http.MethodPost, "/cloud-api/oauth2/token", strings.NewReader(tokenForm.Encode()))
	tokenRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenResponse := httptest.NewRecorder()
	server.ServeHTTP(tokenResponse, tokenRequest)
	if tokenResponse.Code != http.StatusOK {
		t.Fatalf("token status = %d, want 200; body=%s", tokenResponse.Code, tokenResponse.Body.String())
	}
	var tokenBody struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(tokenResponse.Body.Bytes(), &tokenBody); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	if tokenBody.AccessToken == "" {
		t.Fatal("access token is empty")
	}

	replayRequest := httptest.NewRequest(http.MethodPost, "/cloud-api/oauth2/token", strings.NewReader(tokenForm.Encode()))
	replayRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	replayResponse := httptest.NewRecorder()
	server.ServeHTTP(replayResponse, replayRequest)
	assertAPIError(t, replayResponse, http.StatusBadRequest, errorCodeBadRequest)
}

func TestLoginRejectsBadPassword(t *testing.T) {
	server := New(testCloudConfig())
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/cloud-api/auth/login", bytes.NewBufferString(`{"username":"admin","password":"bad"}`)))
	assertAPIError(t, response, http.StatusUnauthorized, errorCodeUnauthorized)
}

func TestLoginRejectsInvalidJSON(t *testing.T) {
	server := New(testCloudConfig())
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/cloud-api/auth/login", bytes.NewBufferString(`{"username"`)))
	assertAPIError(t, response, http.StatusBadRequest, errorCodeBadRequest)
}

func TestHealthRejectsUnsupportedMethod(t *testing.T) {
	server := New(testCloudConfig())
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/cloud-api/health", nil))
	assertAPIError(t, response, http.StatusMethodNotAllowed, errorCodeMethodNotAllowed)
}

func TestUnknownAPIPathReturnsStructuredNotFound(t *testing.T) {
	server := New(testCloudConfig())
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/cloud-api/missing", nil))
	assertAPIError(t, response, http.StatusNotFound, errorCodeNotFound)
}

func assertAPIError(t *testing.T, response *httptest.ResponseRecorder, status int, code string) testErrorResponse {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, status, response.Body.String())
	}
	var body testErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v; body=%s", err, response.Body.String())
	}
	if body.Code != code {
		t.Fatalf("code = %q, want %q; body=%s", body.Code, code, response.Body.String())
	}
	if body.Error == "" {
		t.Fatalf("error is empty; body=%s", response.Body.String())
	}
	if body.RequestId == "" {
		t.Fatalf("requestId is empty; body=%s", response.Body.String())
	}
	var raw map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode raw error response: %v", err)
	}
	if _, ok := raw["message"]; ok {
		t.Fatalf("error response must not include legacy message field: %s", response.Body.String())
	}
	return body
}
