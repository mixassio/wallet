package httpapi

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

func authMiddleware(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			const prefix = "Bearer "

			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, prefix) {
				writeError(w, http.StatusUnauthorized, CodeUnauthorized, "missing or invalid token")
				return
			}

			got := strings.TrimPrefix(header, prefix)
			if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
				writeError(w, http.StatusUnauthorized, CodeUnauthorized, "missing or invalid token")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
