package auth

import "testing"

func TestSelectGitHubEmailPrefersVerifiedPrimary(t *testing.T) {
	email, ok := selectGitHubEmail([]struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}{
		{Email: "secondary@example.test", Verified: true},
		{Email: "Primary@Example.Test", Primary: true, Verified: true},
	})
	if !ok || email != "primary@example.test" {
		t.Fatalf("selectGitHubEmail() = %q, %t", email, ok)
	}
}

func TestSelectGitHubEmailRejectsAmbiguousVerifiedAddresses(t *testing.T) {
	_, ok := selectGitHubEmail([]struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}{
		{Email: "one@example.test", Verified: true},
		{Email: "two@example.test", Verified: true},
	})
	if ok {
		t.Fatal("selectGitHubEmail() accepted ambiguous verified addresses")
	}
}
