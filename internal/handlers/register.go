package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/chepyr/go-task-tracker/internal/db"
	"github.com/chepyr/go-task-tracker/internal/models"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	UserRepo *db.UserRepository
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendError(w, "Use POST method", http.StatusMethodNotAllowed)
		return
	}

	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		sendError(w, "Bad JSON", http.StatusBadRequest)
		return
	}

	// Валидация
	if input.Email == "" || len(input.Password) < 4 {
		sendError(w, "Invalid email or password (min length 8)", http.StatusBadRequest)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		sendError(w, "Cannot hash password", http.StatusInternalServerError)
		return
	}

	user := &models.User{
		ID:           uuid.New(),
		Email:        input.Email,
		PasswordHash: string(hash),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := h.UserRepo.Create(context.Background(), user); err != nil {
		sendError(w, "Cannot save user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user_id": user.ID,
		"email":   user.Email,
	})
}
