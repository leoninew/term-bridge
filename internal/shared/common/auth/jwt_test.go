package auth

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTokenServiceSignUsesConfiguredTtl(t *testing.T) {
	service := NewTokenService([]byte("0123456789abcdef0123456789abcdef"), 90*time.Minute)
	token, err := service.Sign(Claims{Sub: "user-1"})
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	claims, err := service.Verify(token)
	if err != nil {
		t.Fatalf("verify token: %v", err)
	}
	if got, want := claims.Exp-claims.Iat, int64((90*time.Minute)/time.Second); got != want {
		t.Fatalf("token TTL = %ds, want %ds", got, want)
	}
}

func TestTokenServiceVerifyRejectsTokenAtExpirationTime(t *testing.T) {
	service := NewTokenService([]byte("0123456789abcdef0123456789abcdef"))
	payload, err := json.Marshal(Claims{Sub: "user-1", Exp: time.Now().Unix()})
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	if _, err := service.Verify(service.signJWT(payload)); err == nil {
		t.Fatal("Verify() accepted a token at its expiration time")
	}
}
