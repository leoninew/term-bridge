package gatewayapi

import (
	"bytes"
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
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
}
