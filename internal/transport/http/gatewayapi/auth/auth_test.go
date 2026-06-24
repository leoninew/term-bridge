package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func testManager() *Manager {
	return NewManager(Credentials{Username: "admin", Password: "admin"})
}

func TestValidCredentials(t *testing.T) {
	manager := testManager()
	if !manager.ValidCredentials("admin", "admin") {
		t.Fatal("admin/admin rejected")
	}
	if manager.ValidCredentials("admin", "wrong") {
		t.Fatal("wrong password accepted")
	}
	if manager.ValidCredentials("wrong", "admin") {
		t.Fatal("wrong username accepted")
	}
}

func TestLoginSetsCookieAndAuthenticates(t *testing.T) {
	manager := testManager()
	response := httptest.NewRecorder()
	if !manager.Login(response, "admin", "admin") {
		t.Fatal("Login() = false, want true")
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != CookieName || cookies[0].Value == "" || !cookies[0].HttpOnly {
		t.Fatalf("cookies = %#v", cookies)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	request.AddCookie(cookies[0])
	if !manager.Authenticated(request) {
		t.Fatal("Authenticated() = false, want true")
	}
}

func TestLoginRejectsBadPassword(t *testing.T) {
	manager := testManager()
	response := httptest.NewRecorder()
	if manager.Login(response, "admin", "bad") {
		t.Fatal("Login() = true, want false")
	}
	if got := response.Result().Cookies(); len(got) != 0 {
		t.Fatalf("cookies = %#v, want none", got)
	}
}

func TestLogoutClearsSession(t *testing.T) {
	manager := testManager()
	loginResponse := httptest.NewRecorder()
	if !manager.Login(loginResponse, "admin", "admin") {
		t.Fatal("Login() = false")
	}
	cookie := loginResponse.Result().Cookies()[0]
	request := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	request.AddCookie(cookie)
	logoutResponse := httptest.NewRecorder()
	manager.Logout(logoutResponse, request)
	requestAfterLogout := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	requestAfterLogout.AddCookie(cookie)
	if manager.Authenticated(requestAfterLogout) {
		t.Fatal("Authenticated() = true after logout")
	}
	clearCookie := logoutResponse.Result().Cookies()[0]
	if clearCookie.Name != CookieName || clearCookie.MaxAge != -1 {
		t.Fatalf("clear cookie = %#v", clearCookie)
	}
}

func TestMiddlewareRejectsUnauthenticated(t *testing.T) {
	manager := testManager()
	handler := manager.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/devices", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
}
