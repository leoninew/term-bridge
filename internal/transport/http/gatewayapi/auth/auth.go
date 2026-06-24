package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"
)

const CookieName = "termbridge_gateway_session"

type Credentials struct {
	Username string
	Password string
}

type Manager struct {
	username string
	password string
	mu       sync.Mutex
	sessions map[string]time.Time
	now      func() time.Time
}

func NewManager(credentials Credentials) *Manager {
	return &Manager{
		username: strings.TrimSpace(credentials.Username),
		password: strings.TrimSpace(credentials.Password),
		sessions: map[string]time.Time{},
		now:      time.Now,
	}
}

func (m *Manager) Username() string {
	return m.username
}

func (m *Manager) ValidCredentials(username string, password string) bool {
	if m.username == "" || m.password == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(username), []byte(m.username)) == 1 &&
		subtle.ConstantTimeCompare([]byte(password), []byte(m.password)) == 1
}

func (m *Manager) Login(w http.ResponseWriter, username string, password string) bool {
	if !m.ValidCredentials(username, password) {
		return false
	}
	sessionId := randomSessionId()
	m.mu.Lock()
	m.sessions[sessionId] = m.now().UTC()
	m.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: sessionId, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode})
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

func (m *Manager) Middleware(next http.Handler, unauthorized http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.Authenticated(r) {
			unauthorized(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func randomSessionId() string {
	var data [32]byte
	if _, err := rand.Read(data[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(data[:])
}
