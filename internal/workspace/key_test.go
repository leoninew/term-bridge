package workspace

import (
	"regexp"
	"testing"
)

func TestKeyForPathIsStableAndSafe(t *testing.T) {
	path := t.TempDir()
	key1, normalized1, err := KeyForPath(path)
	if err != nil {
		t.Fatalf("KeyForPath() error = %v", err)
	}
	key2, normalized2, err := KeyForPath(path)
	if err != nil {
		t.Fatalf("KeyForPath() error = %v", err)
	}
	if key1 != key2 {
		t.Fatalf("key changed: %q != %q", key1, key2)
	}
	if normalized1 != normalized2 {
		t.Fatalf("normalized changed: %q != %q", normalized1, normalized2)
	}
	if len(key1) != 26 {
		t.Fatalf("len(key) = %d, want 26", len(key1))
	}
	if !regexp.MustCompile(`^[a-z2-7]+$`).MatchString(key1) {
		t.Fatalf("key contains unsafe characters: %q", key1)
	}
}

func TestKeyForPathDiffersForDifferentPaths(t *testing.T) {
	key1, _, err := KeyForPath(t.TempDir())
	if err != nil {
		t.Fatalf("KeyForPath() error = %v", err)
	}
	key2, _, err := KeyForPath(t.TempDir())
	if err != nil {
		t.Fatalf("KeyForPath() error = %v", err)
	}
	if key1 == key2 {
		t.Fatalf("keys unexpectedly equal: %q", key1)
	}
}

func TestNameForPathUsesLastPathSegment(t *testing.T) {
	key := "abcdefghijklmnopqrstuvwxyz"
	if got := NameForPath(`D:\SourceCodes\TermBridge-go`, key); got != "TermBridge-go" {
		t.Fatalf("NameForPath() = %q", got)
	}
}
