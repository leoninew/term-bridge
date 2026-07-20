package security

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"
)

func TestDeviceIdForEd25519PublicKey(t *testing.T) {
	privateKey := ed25519.NewKeyFromSeed([]byte("12345678901234567890123456789012"))
	publicKey := privateKey.Public().(ed25519.PublicKey)

	id, err := DeviceIdForEd25519PublicKey(publicKey)
	if err != nil {
		t.Fatalf("DeviceIdForEd25519PublicKey() error = %v", err)
	}
	if id != "0f490dee643b01b06e0ea84c253a9005" {
		t.Fatalf("DeviceIdForEd25519PublicKey() = %q", id)
	}
	if !IsCanonicalDeviceId(id) {
		t.Fatalf("IsCanonicalDeviceId(%q) = false", id)
	}
}

func TestDeviceIdForEd25519PublicKeyRejectsInvalidLength(t *testing.T) {
	if _, err := DeviceIdForEd25519PublicKey(ed25519.PublicKey("short")); err == nil {
		t.Fatal("DeviceIdForEd25519PublicKey() error = nil, want error")
	}
}

func TestParseCanonicalEd25519PublicKey(t *testing.T) {
	privateKey := ed25519.NewKeyFromSeed([]byte("12345678901234567890123456789012"))
	value := base64.StdEncoding.EncodeToString(privateKey.Public().(ed25519.PublicKey))

	publicKey, canonical, err := ParseCanonicalEd25519PublicKey(value)
	if err != nil {
		t.Fatalf("ParseCanonicalEd25519PublicKey() error = %v", err)
	}
	if string(publicKey) != string(privateKey.Public().(ed25519.PublicKey)) || canonical != value {
		t.Fatalf("ParseCanonicalEd25519PublicKey() = %q, %q", publicKey, canonical)
	}
	for _, invalid := range []string{"", " " + value, value[:len(value)-1], base64.RawStdEncoding.EncodeToString(publicKey)} {
		if _, _, err := ParseCanonicalEd25519PublicKey(invalid); err == nil {
			t.Fatalf("ParseCanonicalEd25519PublicKey(%q) error = nil, want error", invalid)
		}
	}
}

func TestIsCanonicalDeviceId(t *testing.T) {
	for _, value := range []string{"0f490dee643b01b06e0ea84c253a9005", "0E770533588E7E80EF18679C3A2A9A71", "abc", "zz770533588e7e80ef18679c3a2a9a71"} {
		got := IsCanonicalDeviceId(value)
		want := value == "0f490dee643b01b06e0ea84c253a9005"
		if got != want {
			t.Fatalf("IsCanonicalDeviceId(%q) = %v, want %v", value, got, want)
		}
	}
}
