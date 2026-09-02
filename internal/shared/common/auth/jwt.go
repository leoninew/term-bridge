package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"gitee.com/leoninew/TermBridge-go/internal/shared/common/security"
)

type Claims struct {
	Sub      string `json:"sub"`
	Email    string `json:"email,omitempty"`
	Provider string `json:"provider,omitempty"`
	Iat      int64  `json:"iat"`
	Exp      int64  `json:"exp"`
}

type TokenService struct {
	secret []byte
	ttl    time.Duration
}

func NewTokenService(secret []byte, ttl ...time.Duration) TokenService {
	if len(secret) != sha256.Size {
		panic("JWT secret key must be 32 bytes")
	}
	duration := 24 * time.Hour
	if len(ttl) > 0 && ttl[0] > 0 {
		duration = ttl[0]
	}
	return TokenService{secret: append([]byte(nil), secret...), ttl: duration}
}

func NewTokenServiceFromBase64Key(secret string, ttl ...time.Duration) (TokenService, error) {
	key, err := security.ParseBase64Key(secret, sha256.Size)
	if err != nil {
		return TokenService{}, err
	}
	return NewTokenService(key, ttl...), nil
}

func (s TokenService) SecretKey() []byte {
	return append([]byte(nil), s.secret...)
}

func (s TokenService) Sign(claims Claims) (string, error) {
	now := time.Now().UTC()
	claims.Iat = now.Unix()
	claims.Exp = now.Add(s.ttl).Unix()
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	return s.signJWT(payload), nil
}

func (s TokenService) Verify(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, errors.New("invalid token format")
	}
	signed := parts[0] + "." + parts[1]
	expected := s.jwtSignature(signed)
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return Claims{}, errors.New("invalid token signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, err
	}
	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return Claims{}, err
	}
	if claims.Sub == "" || claims.Exp <= time.Now().Unix() {
		return Claims{}, errors.New("token expired")
	}
	return claims, nil
}

func (s TokenService) signJWT(payload []byte) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg": "HS256", "typ": "JWT"}`))
	body := base64.RawURLEncoding.EncodeToString(payload)
	signed := header + "." + body
	return signed + "." + s.jwtSignature(signed)
}

func (s TokenService) jwtSignature(signed string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(signed))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func ExtractBearerToken(r *http.Request) string {
	header := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if header != "" {
		return header
	}
	return r.URL.Query().Get("token")
}
