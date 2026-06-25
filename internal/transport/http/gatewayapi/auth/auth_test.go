package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidCredentials(t *testing.T) {
	a := NewAuther(Credentials{Username: "admin", Password: "admin"}, NewTokenService("test-secret"))
	if !a.ValidCredentials("admin", "admin") {
		t.Fatal("admin/admin rejected")
	}
	if a.ValidCredentials("admin", "wrong") {
		t.Fatal("wrong password accepted")
	}
	if a.ValidCredentials("wrong", "admin") {
		t.Fatal("wrong username accepted")
	}
}

func TestSignAndVerify(t *testing.T) {
	a := NewAuther(Credentials{Username: "admin", Password: "admin"}, NewTokenService("test-secret"))
	token, err := a.SignToken("admin")
	if err != nil {
		t.Fatalf("SignToken() error = %v", err)
	}
	claims, err := a.Verify(token)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if claims.Sub != "admin" {
		t.Fatalf("claims.Sub = %q, want admin", claims.Sub)
	}
}

func TestVerifyRejectsInvalidToken(t *testing.T) {
	a := NewAuther(Credentials{Username: "admin", Password: "admin"}, NewTokenService("test-secret"))
	if _, err := a.Verify("invalid.token.here"); err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestVerifyRejectsWrongSecret(t *testing.T) {
	a1 := NewAuther(Credentials{Username: "admin", Password: "admin"}, NewTokenService("secret-1"))
	a2 := NewAuther(Credentials{Username: "admin", Password: "admin"}, NewTokenService("secret-2"))
	token, _ := a1.SignToken("admin")
	if _, err := a2.Verify(token); err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestAuthenticatedRequest(t *testing.T) {
	a := NewAuther(Credentials{Username: "admin", Password: "admin"}, NewTokenService("test-secret"))
	token, _ := a.SignToken("admin")
	request := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	if !a.Authenticated(request) {
		t.Fatal("Authenticated() = false, want true")
	}
}

func TestAuthenticatedRejectsMissingToken(t *testing.T) {
	a := NewAuther(Credentials{Username: "admin", Password: "admin"}, NewTokenService("test-secret"))
	request := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	if a.Authenticated(request) {
		t.Fatal("Authenticated() = true, want false")
	}
}

func TestMiddlewareRejectsUnauthenticated(t *testing.T) {
	a := NewAuther(Credentials{Username: "admin", Password: "admin"}, NewTokenService("test-secret"))
	handler := a.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestUsernameFromRequest(t *testing.T) {
	a := NewAuther(Credentials{Username: "admin", Password: "admin"}, NewTokenService("test-secret"))
	token, _ := a.SignToken("admin")
	request := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	if got := a.UsernameFromRequest(request); got != "admin" {
		t.Fatalf("UsernameFromRequest() = %q, want admin", got)
	}
}

func TestUsernameFromRequestWithoutToken(t *testing.T) {
	a := NewAuther(Credentials{Username: "admin", Password: "admin"}, NewTokenService("test-secret"))
	request := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	if got := a.UsernameFromRequest(request); got != "" {
		t.Fatalf("UsernameFromRequest() = %q, want empty", got)
	}
}
