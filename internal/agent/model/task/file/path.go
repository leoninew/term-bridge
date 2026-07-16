package file

import (
	"fmt"
	"path"
	"strings"
)

type RelativePath string

func ParseRelativePath(value string, allowRoot bool) (RelativePath, error) {
	if value == "" {
		if allowRoot {
			return "", nil
		}
		return "", InvalidPath("path is required")
	}
	if strings.ContainsRune(value, '\x00') || strings.Contains(value, "\\") || strings.HasPrefix(value, "/") {
		return "", InvalidPath("path must use slash-delimited relative segments")
	}
	if strings.Contains(value, ":") || strings.HasPrefix(value, "//") {
		return "", InvalidPath("path must not contain a volume or network prefix")
	}
	if path.Clean(value) != value {
		return "", InvalidPath("path is not canonical")
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return "", InvalidPath("path contains an invalid segment")
		}
	}
	return RelativePath(value), nil
}

func ParseName(value string) (string, error) {
	parsed, err := ParseRelativePath(value, false)
	if err != nil {
		return "", err
	}
	if strings.Contains(string(parsed), "/") {
		return "", InvalidPath("name must contain one segment")
	}
	return string(parsed), nil
}

func (p RelativePath) String() string {
	return string(p)
}

func (p RelativePath) Parent() RelativePath {
	value := string(p)
	if value == "" || !strings.Contains(value, "/") {
		return ""
	}
	return RelativePath(path.Dir(value))
}

func (p RelativePath) Base() string {
	if p == "" {
		return ""
	}
	return path.Base(string(p))
}

func (p RelativePath) IsWithin(prefix RelativePath) bool {
	if prefix == "" {
		return p != ""
	}
	return p == prefix || strings.HasPrefix(string(p), fmt.Sprintf("%s/", prefix))
}
