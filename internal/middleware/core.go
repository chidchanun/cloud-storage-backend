package middleware

import (
	"net/http"
	"strings"
)

func CORSMiddleware(next http.Handler, origins ...string) http.Handler {
	allowedOrigins := map[string]bool{
		"http://localhost:4200": true,
	}

	for _, origin := range origins {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			allowedOrigins[origin] = true
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		if allowedOrigins[origin] {
			w.Header().Set(
				"Access-Control-Allow-Origin",
				origin,
			)
			w.Header().Set(
				"Access-Control-Allow-Credentials",
				"true",
			)
		}

		w.Header().Set(
			"Access-Control-Allow-Headers",
			"Content-Type, Authorization",
		)

		w.Header().Set(
			"Access-Control-Allow-Methods",
			"GET, POST, PUT, PATCH, DELETE, OPTIONS",
		)

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
