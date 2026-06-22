package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidCredentials(t *testing.T) {
	if !ValidCredentials("admin", "admin") {
		t.Fatal("admin/admin rejected")
	}
	if ValidCredentials("admin", "wrong") {
		t.Fatal("wrong password accepted")
	}
	if ValidCredentials("wrong", "admin") {
		t.Fatal("wrong username accepted")
	}
}

func TestLoginSetsCookieAndAuthenticates(t *testing.T) {
	manager := NewManager()
	response := httptest.NewRecorder()
	if !manager.Login(response, "admin", "admin") {
		t.Fatal("Login() = false, want true")
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != CookieName || cookies[0].Value == "" || !cookies[0].HttpOnly {
		t.Fatalf("cookies = %#v", cookies)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/gateway/devices", nil)
	request.AddCookie(cookies[0])
	if !manager.Authenticated(request) {
		t.Fatal("Authenticated() = false, want true")
	}
}

func TestLoginRejectsBadPassword(t *testing.T) {
	manager := NewManager()
	response := httptest.NewRecorder()
	if manager.Login(response, "admin", "bad") {
		t.Fatal("Login() = true, want false")
	}
	if got := response.Result().Cookies(); len(got) != 0 {
		t.Fatalf("cookies = %#v, want none", got)
	}
}

func TestLogoutClearsSession(t *testing.T) {
	manager := NewManager()
	loginResponse := httptest.NewRecorder()
	if !manager.Login(loginResponse, "admin", "admin") {
		t.Fatal("Login() = false")
	}
	cookie := loginResponse.Result().Cookies()[0]
	request := httptest.NewRequest(http.MethodPost, "/api/gateway/logout", nil)
	request.AddCookie(cookie)
	logoutResponse := httptest.NewRecorder()
	manager.Logout(logoutResponse, request)
	requestAfterLogout := httptest.NewRequest(http.MethodGet, "/api/gateway/devices", nil)
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
	manager := NewManager()
	handler := manager.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/gateway/devices", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
}
