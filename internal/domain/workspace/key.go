package workspace

import (
	"path/filepath"
	"strings"
)

func NormalizePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

func NameForPath(path string) string {
	name := lastPathSegment(filepath.Clean(path))
	if name == "." || name == string(filepath.Separator) || name == "" {
		return "workspace"
	}
	return name
}

func lastPathSegment(path string) string {
	path = strings.TrimRight(path, `/\`)
	if path == "" {
		return ""
	}
	index := strings.LastIndexAny(path, `/\`)
	if index == -1 {
		return path
	}
	return path[index+1:]
}
