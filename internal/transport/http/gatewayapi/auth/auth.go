package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

const (
	Username   = "admin"
	Password   = "admin"
	CookieName = "termbridge_gateway_session"
)

type Manager struct {
	mu       sync.Mutex
	sessions map[string]time.Time
	now      func() time.Time
}

func NewManager() *Manager {
	return &Manager{sessions: map[string]time.Time{}, now: time.Now}
}

func ValidCredentials(username string, password string) bool {
	return subtle.ConstantTimeCompare([]byte(username), []byte(Username)) == 1 && subtle.ConstantTimeCompare([]byte(password), []byte(Password)) == 1
}

func (m *Manager) Login(w http.ResponseWriter, username string, password string) bool {
	if !ValidCredentials(username, password) {
		return false
	}
	sessionID := randomSessionID()
	m.mu.Lock()
	m.sessions[sessionID] = m.now().UTC()
	m.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: sessionID, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode})
	return true
}

func (m *Manager) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(CookieName)
	if err == nil {
		m.mu.Lock()
		delete(m.sessions, cookie.Value)
		m.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
}

func (m *Manager) Authenticated(r *http.Request) bool {
	cookie, err := r.Cookie(CookieName)
	if err != nil || cookie.Value == "" {
		return false
	}
	m.mu.Lock()
	_, ok := m.sessions[cookie.Value]
	m.mu.Unlock()
	return ok
}

func (m *Manager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.Authenticated(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func randomSessionID() string {
	var data [32]byte
	if _, err := rand.Read(data[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(data[:])
}
