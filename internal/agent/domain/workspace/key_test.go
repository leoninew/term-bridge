package workspace

import "testing"

func TestNormalizePathIsStable(t *testing.T) {
	path := t.TempDir()
	normalized1, err := NormalizePath(path)
	if err != nil {
		t.Fatalf("NormalizePath() error = %v", err)
	}
	normalized2, err := NormalizePath(path)
	if err != nil {
		t.Fatalf("NormalizePath() error = %v", err)
	}
	if normalized1 != normalized2 {
		t.Fatalf("normalized changed: %q != %q", normalized1, normalized2)
	}
}

func TestNameForPathUsesLastPathSegment(t *testing.T) {
	if got := NameForPath(`D:\SourceCodes\TermBridge-go`); got != "TermBridge-go" {
		t.Fatalf("NameForPath() = %q", got)
	}
}
