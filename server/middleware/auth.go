package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

var JWTSecret = loadJWTSecret()

// loadJWTSecret resolves the key used to sign auth tokens. It prefers the
// JWT_SECRET environment variable (for production deployments), and
// otherwise falls back to a key persisted in jwt_secret.key, generating one
// on first run. Persisting it (rather than regenerating per-run) matters
// because this server restarts often via the admin redeploy button — a
// secret that changed on every restart would invalidate every signed-in
// session each time.
func loadJWTSecret() []byte {
	if s := os.Getenv("JWT_SECRET"); s != "" {
		return []byte(s)
	}

	const path = "jwt_secret.key"
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		return data
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		log.Fatal("Failed to generate JWT secret:", err)
	}
	secret := []byte(hex.EncodeToString(raw))
	if err := os.WriteFile(path, secret, 0600); err != nil {
		log.Printf("Warning: could not persist JWT secret to %s, it will regenerate on next restart and invalidate existing sessions: %v", path, err)
	}
	return secret
}

type contextKey string
const UserKey contextKey = "user"

type Claims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
			return JWTSecret, nil
		})
		if err != nil || !token.Valid {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), UserKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func AdminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value(UserKey).(*Claims)
		if !ok || claims.Role != "admin" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func OptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			claims := &Claims{}
			token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
				return JWTSecret, nil
			})
			if err == nil && token.Valid {
				ctx := context.WithValue(r.Context(), UserKey, claims)
				r = r.WithContext(ctx)
			}
		}
		next.ServeHTTP(w, r)
	})
}