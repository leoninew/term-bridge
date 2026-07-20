package security

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

const DeviceIdLength = 32

func DeviceIdForEd25519PublicKey(publicKey ed25519.PublicKey) (string, error) {
	if len(publicKey) != ed25519.PublicKeySize {
		return "", fmt.Errorf("invalid Ed25519 public key length")
	}
	digest := sha256.Sum256(publicKey)
	return hex.EncodeToString(digest[:DeviceIdLength/2]), nil
}

func ParseCanonicalEd25519PublicKey(value string) (ed25519.PublicKey, string, error) {
	if strings.TrimSpace(value) != value || value == "" {
		return nil, "", fmt.Errorf("invalid Ed25519 public key")
	}
	publicKey, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(publicKey) != ed25519.PublicKeySize || base64.StdEncoding.EncodeToString(publicKey) != value {
		return nil, "", fmt.Errorf("invalid Ed25519 public key")
	}
	return ed25519.PublicKey(publicKey), value, nil
}

func IsCanonicalDeviceId(value string) bool {
	if len(value) != DeviceIdLength {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == DeviceIdLength/2 && hex.EncodeToString(decoded) == value
}
