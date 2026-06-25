package auth

import (
	"crypto/subtle"
	"net/http"
)

type Credentials struct {
	Username string
	Password string
}

type Auther struct {
	credentials Credentials
	tokens      TokenService
}

func NewAuther(credentials Credentials, tokens TokenService) *Auther {
	return &Auther{credentials: credentials, tokens: tokens}
}

func (a *Auther) ValidCredentials(username string, password string) bool {
	if a.credentials.Username == "" || a.credentials.Password == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(username), []byte(a.credentials.Username)) == 1 &&
		subtle.ConstantTimeCompare([]byte(password), []byte(a.credentials.Password)) == 1
}

func (a *Auther) Username() string {
	return a.credentials.Username
}

func (a *Auther) Authenticated(r *http.Request) bool {
	token := extractBearerToken(r)
	if token == "" {
		return false
	}
	_, err := a.tokens.Verify(token)
	return err == nil
}

func (a *Auther) SignToken(username string) (string, error) {
	return a.tokens.Sign(username)
}

func (a *Auther) Verify(token string) (Claims, error) {
	return a.tokens.Verify(token)
}

func (a *Auther) UsernameFromRequest(r *http.Request) string {
	token := extractBearerToken(r)
	if token == "" {
		return ""
	}
	claims, err := a.tokens.Verify(token)
	if err != nil {
		return ""
	}
	return claims.Sub
}

func (a *Auther) Middleware(next http.Handler, unauthorized http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.Authenticated(r) {
			unauthorized(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
