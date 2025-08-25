package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"time"

	"github.com/chepyr/go-task-tracker/shared/models"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func (handler *Handler) Register(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		log.Printf("Invalid method for register: %s", request.Method)
		http.Error(writer, "Use POST method", http.StatusMethodNotAllowed)
		return
	}

	clientIP := request.RemoteAddr
	if handler.RateLimiter != nil && !handler.RateLimiter.Allow(clientIP) {
		log.Printf("Rate limit exceeded for IP: %s", clientIP)
		http.Error(writer, "Too many register attempts. Please try again later.", http.StatusTooManyRequests)
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

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Error hashing password: %v", err)
		http.Error(writer, "Cannot hash password", http.StatusInternalServerError)
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
		http.Error(writer, "Cannot save user", http.StatusInternalServerError)
		return
	}

	log.Printf("User registered: %s", user.Email)
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusCreated)
	json.NewEncoder(writer).Encode(map[string]any{
		"user_id": user.ID,
		"email":   user.Email,
	})

}

func validateUserEmailAndPassword(input struct {
	Email    string "json:\"email\""
	Password string "json:\"password\""
}, writer http.ResponseWriter) bool {

	if !isValidEmail(input.Email) {
		log.Printf("Invalid email format")
		http.Error(writer, "Invalid email", http.StatusBadRequest)
		return false
	}
	if len(input.Password) < 4 {
		log.Printf("Password too short")
		http.Error(writer, "Password must be at least 4 characters long", http.StatusBadRequest)
		return false
	}
	return true
}

func isValidEmail(email string) bool {
	// _, err := mail.ParseAddress(email)
	// return err == nil

	const emailRegex = `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	re := regexp.MustCompile(emailRegex)
	return re.MatchString(email)
}
