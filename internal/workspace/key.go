package workspace

import (
	"crypto/sha256"
	"encoding/base32"
	"path/filepath"
	"strings"
)

const PathHashInputVersion = 1

func NormalizePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

func KeyForPath(path string) (string, string, error) {
	normalized, err := NormalizePath(path)
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256([]byte(normalized))
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sum[:])
	key := strings.ToLower(encoded)
	if len(key) > 26 {
		key = key[:26]
	}
	return key, normalized, nil
}

func NameForPath(path string, key string) string {
	name := filepath.Base(filepath.Clean(path))
	if name == "." || name == string(filepath.Separator) || name == "" {
		if len(key) > 8 {
			return key[:8]
		}
		return key
	}
	return name
}
