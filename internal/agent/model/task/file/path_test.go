package file

import "testing"

func TestParseRelativePathAcceptsCanonicalLogicalPath(t *testing.T) {
	parsed, err := ParseRelativePath("directory/file.txt", false)
	if err != nil {
		t.Fatalf("parse canonical path: %v", err)
	}
	if parsed != "directory/file.txt" {
		t.Fatalf("path = %q, want %q", parsed, "directory/file.txt")
	}
}

func TestParseRelativePathRejectsFilesystemEscapes(t *testing.T) {
	invalidPaths := []string{
		"",
		"/absolute.txt",
		"C:/workspace/file.txt",
		"\\\\server\\share",
		"directory\\file.txt",
		"directory/../file.txt",
		"directory//file.txt",
		"./file.txt",
		"file\x00.txt",
	}
	for _, value := range invalidPaths {
		t.Run(value, func(t *testing.T) {
			if _, err := ParseRelativePath(value, false); err == nil {
				t.Fatalf("path %q was accepted", value)
			}
		})
	}
}

func TestParseNameRequiresOneLogicalSegment(t *testing.T) {
	if _, err := ParseName("renamed.txt"); err != nil {
		t.Fatalf("parse name: %v", err)
	}
	if _, err := ParseName("directory/renamed.txt"); err == nil {
		t.Fatal("multi-segment name was accepted")
	}
}
