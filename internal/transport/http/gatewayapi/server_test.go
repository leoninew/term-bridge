package gatewayapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testGatewayConfig() Config {
	return Config{Username: "admin", Password: "admin", Logger: slog.Default()}
}

func TestHealth(t *testing.T) {
	server := New(testGatewayConfig())
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if !bytes.Contains(response.Body.Bytes(), []byte(`"status":"ok"`)) {
		t.Fatalf("body = %s", response.Body.String())
	}
}

func TestAuthEndpoints(t *testing.T) {
	server := New(testGatewayConfig())
	devicesResponse := httptest.NewRecorder()
	server.ServeHTTP(devicesResponse, httptest.NewRequest(http.MethodGet, "/api/devices", nil))
	if devicesResponse.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated devices status = %d, want 401", devicesResponse.Code)
	}
	assertAPIError(t, devicesResponse, http.StatusUnauthorized, errorCodeUnauthorized)

	loginResponse := httptest.NewRecorder()
	server.ServeHTTP(loginResponse, httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewBufferString(`{"username":"admin","password":"admin"}`)))
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200; body=%s", loginResponse.Code, loginResponse.Body.String())
	}
	cookies := loginResponse.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %#v", cookies)
	}

	meRequest := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	meRequest.AddCookie(cookies[0])
	meResponse := httptest.NewRecorder()
	server.ServeHTTP(meResponse, meRequest)
	if meResponse.Code != http.StatusOK {
		t.Fatalf("me status = %d, want 200; body=%s", meResponse.Code, meResponse.Body.String())
	}

	logoutRequest := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	logoutRequest.AddCookie(cookies[0])
	logoutResponse := httptest.NewRecorder()
	server.ServeHTTP(logoutResponse, logoutRequest)
	if logoutResponse.Code != http.StatusOK {
		t.Fatalf("logout status = %d, want 200", logoutResponse.Code)
	}

	devicesRequestAfterLogout := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	devicesRequestAfterLogout.AddCookie(cookies[0])
	devicesResponseAfterLogout := httptest.NewRecorder()
	server.ServeHTTP(devicesResponseAfterLogout, devicesRequestAfterLogout)
	if devicesResponseAfterLogout.Code != http.StatusUnauthorized {
		t.Fatalf("post-logout devices status = %d, want 401", devicesResponseAfterLogout.Code)
	}
}

func TestLoginRejectsBadPassword(t *testing.T) {
	server := New(testGatewayConfig())
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewBufferString(`{"username":"admin","password":"bad"}`)))
	assertAPIError(t, response, http.StatusUnauthorized, errorCodeUnauthorized)
}

func TestLoginRejectsInvalidJSON(t *testing.T) {
	server := New(testGatewayConfig())
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewBufferString(`{"username"`)))
	assertAPIError(t, response, http.StatusBadRequest, errorCodeBadRequest)
}

func TestHealthRejectsUnsupportedMethod(t *testing.T) {
	server := New(testGatewayConfig())
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/health", nil))
	assertAPIError(t, response, http.StatusMethodNotAllowed, errorCodeMethodNotAllowed)
}

func TestUnknownAPIPathReturnsStructuredNotFound(t *testing.T) {
	server := New(testGatewayConfig())
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/missing", nil))
	assertAPIError(t, response, http.StatusNotFound, errorCodeNotFound)
}

func assertAPIError(t *testing.T, response *httptest.ResponseRecorder, status int, code string) errorResponse {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, status, response.Body.String())
	}
	var body errorResponse
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
		t.Fatalf("error response contains message field: %s", response.Body.String())
	}
	return body
}
