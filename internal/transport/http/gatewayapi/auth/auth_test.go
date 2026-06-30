package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

var (
	testJWTKeyOne = []byte("11111111111111111111111111111111")
	testJWTKeyTwo = []byte("22222222222222222222222222222222")
)

func TestValidCredentials(t *testing.T) {
	a := NewAuther(Credentials{Username: "admin", Password: "admin"}, NewTokenService(testJWTKeyOne))
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

func TestNewTokenServiceFromBase64KeyAcceptsStrictKey(t *testing.T) {
	service, err := NewTokenServiceFromBase64Key("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
	if err != nil {
		t.Fatalf("NewTokenServiceFromBase64Key() error = %v", err)
	}
	if len(service.SecretKey()) != 32 {
		t.Fatalf("len(SecretKey()) = %d, want 32", len(service.SecretKey()))
	}
}

func TestNewTokenServiceFromBase64KeyRejectsPlainSecret(t *testing.T) {
	if _, err := NewTokenServiceFromBase64Key("test-secret"); err == nil {
		t.Fatal("NewTokenServiceFromBase64Key() error = nil, want error")
	}
}

func TestSignAndVerify(t *testing.T) {
	a := NewAuther(Credentials{Username: "admin", Password: "admin"}, NewTokenService(testJWTKeyOne))
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
	a := NewAuther(Credentials{Username: "admin", Password: "admin"}, NewTokenService(testJWTKeyOne))
	if _, err := a.Verify("invalid.token.here"); err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestVerifyRejectsWrongSecret(t *testing.T) {
	a1 := NewAuther(Credentials{Username: "admin", Password: "admin"}, NewTokenService(testJWTKeyOne))
	a2 := NewAuther(Credentials{Username: "admin", Password: "admin"}, NewTokenService(testJWTKeyTwo))
	token, _ := a1.SignToken("admin")
	if _, err := a2.Verify(token); err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestAuthenticatedRequest(t *testing.T) {
	a := NewAuther(Credentials{Username: "admin", Password: "admin"}, NewTokenService(testJWTKeyOne))
	token, _ := a.SignToken("admin")
	request := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	if !a.Authenticated(request) {
		t.Fatal("Authenticated() = false, want true")
	}
}

func TestAuthenticatedRejectsMissingToken(t *testing.T) {
	a := NewAuther(Credentials{Username: "admin", Password: "admin"}, NewTokenService(testJWTKeyOne))
	request := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	if a.Authenticated(request) {
		t.Fatal("Authenticated() = true, want false")
	}
}

func TestMiddlewareRejectsUnauthenticated(t *testing.T) {
	a := NewAuther(Credentials{Username: "admin", Password: "admin"}, NewTokenService(testJWTKeyOne))
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
	a := NewAuther(Credentials{Username: "admin", Password: "admin"}, NewTokenService(testJWTKeyOne))
	token, _ := a.SignToken("admin")
	request := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	if got := a.UsernameFromRequest(request); got != "admin" {
		t.Fatalf("UsernameFromRequest() = %q, want admin", got)
	}
}

func TestUsernameFromRequestWithoutToken(t *testing.T) {
	a := NewAuther(Credentials{Username: "admin", Password: "admin"}, NewTokenService(testJWTKeyOne))
	request := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	if got := a.UsernameFromRequest(request); got != "" {
		t.Fatalf("UsernameFromRequest() = %q, want empty", got)
	}
}
