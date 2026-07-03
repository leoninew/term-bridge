package security

import "testing"

const testBase64Key32 = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="

func TestParseBase64KeyAcceptsPaddedURLKey(t *testing.T) {
	key, err := ParseBase64Key(testBase64Key32, 32)
	if err != nil {
		t.Fatalf("ParseBase64Key() error = %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("len(key) = %d, want 32", len(key))
	}
}

func TestParseBase64KeyAcceptsRawURLKey(t *testing.T) {
	key, err := ParseBase64Key("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", 32)
	if err != nil {
		t.Fatalf("ParseBase64Key() error = %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("len(key) = %d, want 32", len(key))
	}
}

func TestParseBase64KeyRejectsPlainSecret(t *testing.T) {
	if _, err := ParseBase64Key("test-secret-key-for-tests", 32); err == nil {
		t.Fatal("ParseBase64Key() error = nil, want error")
	}
}

func TestParseBase64KeyRejectsWrongLength(t *testing.T) {
	if _, err := ParseBase64Key("AAAA", 32); err == nil {
		t.Fatal("ParseBase64Key() error = nil, want error")
	}
}
