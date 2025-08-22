package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/mail"
	"time"

	"github.com/chepyr/go-task-tracker/internal/db"
	"github.com/chepyr/go-task-tracker/internal/models"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	UserRepo    *db.UserRepository
	RateLimiter *RateLimiter
}

func (handler *Handler) Register(window http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		log.Printf("Invalid method for register: %s", request.Method)
		sendError(window, "Use POST method", http.StatusMethodNotAllowed)
		return
	}

	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
		log.Printf("Error decoding JSON: %v", err)
		sendError(window, "Bad JSON", http.StatusBadRequest)
		return
	}

	if !validateUserInput(input, window) {
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Error hashing password: %v", err)
		sendError(window, "Cannot hash password", http.StatusInternalServerError)
		return
	}

	user := &models.User{
		ID:           uuid.New(),
		Email:        input.Email,
		PasswordHash: string(hash),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := handler.UserRepo.Create(context.Background(), user); err != nil {
		sendError(window, "Cannot save user", http.StatusInternalServerError)
		return
	}

	window.Header().Set("Content-Type", "application/json")
	window.WriteHeader(http.StatusCreated)
	json.NewEncoder(window).Encode(map[string]any{
		"user_id": user.ID,
		"email":   user.Email,
	})
	log.Printf("User registered: %s", user.Email)
}

func validateUserInput(input struct {
	Email    string "json:\"email\""
	Password string "json:\"password\""
}, window http.ResponseWriter) bool {

	if !isValidEmail(input.Email) {
		log.Printf("Invalid email format: %s", input.Email)
		sendError(window, "Invalid email", http.StatusBadRequest)
		return true
	}
	if len(input.Password) < 4 {
		log.Printf("Password too short: %s", input.Password)
		sendError(window, "Password must be at least 4 characters long", http.StatusBadRequest)
		return true
	}
	return false
}

func isValidEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}
