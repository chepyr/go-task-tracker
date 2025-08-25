package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func (handler *Handler) Login(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		log.Printf("Invalid method for login: %s", request.Method)
		http.Error(writer, "Use POST method for login", http.StatusMethodNotAllowed)
		return
	}

	clientIP := request.RemoteAddr
	if handler.RateLimiter != nil && !handler.RateLimiter.Allow(clientIP) {
		log.Printf("Rate limit exceeded for IP: %s", clientIP)
		http.Error(writer, "Too many login attempts. Please try again later.", http.StatusTooManyRequests)
		return
	}

	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
		log.Printf("Error decoding JSON: %v", err)
		http.Error(writer, "Bad JSON", http.StatusBadRequest)
		return
	}
	if !validateUserEmailAndPassword(input, writer) {
		return
	}

	// Retrieve user from the database
	user, err := handler.UserRepo.GetByEmail(context.Background(), input.Email)
	if err != nil {
		log.Printf("Error retrieving user by email %s: %v", input.Email, err)
		http.Error(writer, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	// Compare provided password with stored password hash
	if err := bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		log.Printf("Invalid password for email: %s", input.Email)
		http.Error(writer, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	tokenString, err := generateJWTToken(user.ID.String())
	if err != nil {
		log.Printf("Error generating token: %v", err)
		http.Error(writer, "Cannot create token", http.StatusInternalServerError)
		return
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	json.NewEncoder(writer).Encode(map[string]any{
		"user_email": input.Email,
		"user_id":    user.ID,
		"token":      tokenString,
	})
	log.Printf("User logged in: %s", input.Email)
}

func generateJWTToken(sub string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": sub,
		"exp": time.Now().Add(24 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	})

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return "", fmt.Errorf("JWT_SECRET environment variable is not set")
	}

	tokenString, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		return "", fmt.Errorf("error signing token: %w", err)
	}

	return tokenString, nil
}
