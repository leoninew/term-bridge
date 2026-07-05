package middleware

import (
	"net/http"
	"strings"
)

const (
	corsAllowHeaders = "Authorization, Content-Type"
	corsAllowMethods = "GET, POST, PUT, PATCH, DELETE, OPTIONS"
	corsMaxAge       = "600"
)

func Cors(allowedOrigins []string) func(http.Handler) http.Handler {
	return CorsForPaths(allowedOrigins, nil)
}

func CorsForPaths(allowedOrigins []string, appliesTo func(string) bool) func(http.Handler) http.Handler {
	allowed := originSet(allowedOrigins)
	return func(next http.Handler) http.Handler {
		if len(allowed) == 0 {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if appliesTo != nil && !appliesTo(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			origin := normalizeOrigin(r.Header.Get("Origin"))
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}
			if !allowed[origin] {
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				next.ServeHTTP(w, r)
				return
			}
			setCorsHeaders(w.Header(), origin)
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func originSet(origins []string) map[string]bool {
	out := map[string]bool{}
	for _, origin := range origins {
		origin = normalizeOrigin(origin)
		if origin != "" {
			out[origin] = true
		}
	}
	return out
}

func normalizeOrigin(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "/")
}

func setCorsHeaders(headers http.Header, origin string) {
	headers.Set("Access-Control-Allow-Origin", origin)
	headers.Set("Access-Control-Allow-Headers", corsAllowHeaders)
	headers.Set("Access-Control-Allow-Methods", corsAllowMethods)
	headers.Set("Access-Control-Max-Age", corsMaxAge)
	headers.Add("Vary", "Origin")
}
