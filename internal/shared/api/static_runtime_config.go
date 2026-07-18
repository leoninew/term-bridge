package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	browserdto "gitee.com/leoninew/TermBridge-go/internal/shared/dto/browser"
)

const runtimeConfigMarker = "<!-- __RUNTIME_CONFIG__ -->"

func ValidateStaticDir(staticDir string, runtimeConfig browserdto.RuntimeConfig) error {
	if staticDir == "" {
		return nil
	}
	info, err := os.Stat(staticDir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return os.ErrInvalid
	}
	indexPath := filepath.Join(staticDir, "index.html")
	index, err := os.ReadFile(indexPath)
	if err != nil {
		return err
	}
	if bytes.Count(index, []byte(runtimeConfigMarker)) != 1 {
		return fmt.Errorf("index.html must contain exactly one %s marker", runtimeConfigMarker)
	}
	_, err = renderIndexHTML(index, runtimeConfig)
	return err
}

func StaticHandler(staticDir string, runtimeConfig browserdto.RuntimeConfig) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		urlPath := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if urlPath == "." {
			urlPath = ""
		}
		if urlPath != "" {
			fullPath := filepath.Join(staticDir, filepath.FromSlash(urlPath))
			if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
				serveStaticFile(w, r, fullPath, urlPath, info)
				return
			}
			if path.Ext(urlPath) != "" {
				http.NotFound(w, r)
				return
			}
		}

		index, err := os.ReadFile(filepath.Join(staticDir, "index.html"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		content, err := renderIndexHTML(index, runtimeConfig)
		if err != nil {
			http.Error(w, "invalid runtime config entry", http.StatusInternalServerError)
			return
		}
		writeHTMLResponse(w, r, content)
	})
}

func renderIndexHTML(index []byte, runtimeConfig browserdto.RuntimeConfig) ([]byte, error) {
	if bytes.Count(index, []byte(runtimeConfigMarker)) != 1 {
		return nil, fmt.Errorf("index.html must contain exactly one %s marker", runtimeConfigMarker)
	}
	encoded, err := json.Marshal(runtimeConfig)
	if err != nil {
		return nil, fmt.Errorf("marshal browser runtime config: %w", err)
	}
	injection := []byte("<script>window.__CONFIG__ = " + string(encoded) + ";</script>")
	return bytes.Replace(index, []byte(runtimeConfigMarker), injection, 1), nil
}
