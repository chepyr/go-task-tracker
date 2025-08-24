package handlers

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

/*
Verify JWT tokens by making HTTP requests to the auth service
Extract the user ID from the token and add it to the request context
*/
func (h *Handler) AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ah := r.Header.Get("Authorization")
		if ah == "" {
			sendError(w, "Missing Authorization header", http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(ah, "Bearer ")

		claims := jwt.MapClaims{}
		parser := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
		token, err := parser.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
			return []byte(os.Getenv("JWT_SECRET")), nil
		})
		if err != nil || !token.Valid {
			sendError(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		if _, ok := claims["exp"].(float64); !ok {
			sendError(w, "Token missing exp", http.StatusUnauthorized)
			return
		}
		uid, _ := claims["sub"].(string)
		if uid == "" {
			sendError(w, "Invalid token claims", http.StatusUnauthorized)
			return
		}

		type contextKey string
		const userIDKey contextKey = "user_id"
		ctx := context.WithValue(r.Context(), userIDKey, uid)
		next(w, r.WithContext(ctx))
	}
}
