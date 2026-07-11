package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const turnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

var errAuthSecurityValidation = errors.New("authentication security validation failed")

type siteverifyVerifier struct {
	secretKey        string
	expectedHostname string
	client           *http.Client
}

type siteverifyResponse struct {
	Success  bool   `json:"success"`
	Hostname string `json:"hostname"`
}

func NewTurnstileVerifier(secretKey string, expectedHostname string, client *http.Client) TurnstileVerifier {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return &siteverifyVerifier{
		secretKey:        strings.TrimSpace(secretKey),
		expectedHostname: normalizeHostname(expectedHostname),
		client:           client,
	}
}

func (v *siteverifyVerifier) Verify(ctx context.Context, token string, remoteIP string) error {
	if strings.TrimSpace(token) == "" || v.secretKey == "" {
		return errAuthSecurityValidation
	}
	values := url.Values{"secret": {v.secretKey}, "response": {token}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, turnstileVerifyURL, strings.NewReader(values.Encode()))
	if err != nil {
		return fmt.Errorf("create Turnstile verification request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := v.client.Do(req)
	if err != nil {
		return errAuthSecurityValidation
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode != http.StatusOK {
		return errAuthSecurityValidation
	}
	var result siteverifyResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 16*1024)).Decode(&result); err != nil || !result.Success {
		return errAuthSecurityValidation
	}
	if v.expectedHostname != "" && normalizeHostname(result.Hostname) != v.expectedHostname {
		return errAuthSecurityValidation
	}
	return nil
}

type inMemoryCSRFTokens struct {
	mu       sync.Mutex
	tokens   map[[sha256.Size]byte]time.Time
	ttl      time.Duration
	maxItems int
	now      func() time.Time
}

func NewCSRFTokens(ttl time.Duration, maxItems int) CSRFTokenService {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	if maxItems < 1 {
		maxItems = 1024
	}
	return &inMemoryCSRFTokens{tokens: make(map[[sha256.Size]byte]time.Time), ttl: ttl, maxItems: maxItems, now: time.Now}
}

func (s *inMemoryCSRFTokens) Issue() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw[:])
	digest := sha256.Sum256([]byte(encoded))
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.removeExpired(now)
	if len(s.tokens) >= s.maxItems {
		return "", errors.New("csrf token capacity reached")
	}
	s.tokens[digest] = now.Add(s.ttl)
	return encoded, nil
}

func (s *inMemoryCSRFTokens) Consume(token string) bool {
	if strings.TrimSpace(token) == "" {
		return false
	}
	digest := sha256.Sum256([]byte(token))
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	expiresAt, ok := s.tokens[digest]
	if !ok || !expiresAt.After(now) {
		delete(s.tokens, digest)
		return false
	}
	delete(s.tokens, digest)
	candidate := sha256.Sum256([]byte(token))
	return subtle.ConstantTimeCompare(digest[:], candidate[:]) == 1
}

func (s *inMemoryCSRFTokens) removeExpired(now time.Time) {
	for digest, expiresAt := range s.tokens {
		if !expiresAt.After(now) {
			delete(s.tokens, digest)
		}
	}
}

func normalizeHostname(hostname string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(hostname)), ".")
}
