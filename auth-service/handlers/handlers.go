package handlers

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/chepyr/go-task-tracker/auth-service/db"
)

type Handler struct {
	UserRepo    db.UserRepositoryInterface
	RateLimiter *RateLimiter
}

type errorResponse struct {
	Error string `json:"error"`
}

func http.Error(window http.ResponseWriter, message string, status int) {
	window.Header().Set("Content-Type", "application/json")
	window.WriteHeader(status)
	json.NewEncoder(window).Encode(errorResponse{Error: message})
}

type RateLimiter struct {
	attempts map[string]int
	limit    int
	mutex    sync.Mutex
	window   time.Duration
}

// reset the attempts map every window duration
func (rateLimiter *RateLimiter) cleanup() {
	for range time.Tick(rateLimiter.window) {
		rateLimiter.mutex.Lock()
		rateLimiter.attempts = make(map[string]int)
		rateLimiter.mutex.Unlock()
	}
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rateLimiter := &RateLimiter{
		attempts: make(map[string]int),
		limit:    limit,
		window:   window,
	}
	go rateLimiter.cleanup()
	return rateLimiter
}

func (rateLimiter *RateLimiter) Allow(ip string) bool {
	rateLimiter.mutex.Lock()
	defer rateLimiter.mutex.Unlock()

	count, exists := rateLimiter.attempts[ip]
	if !exists {
		rateLimiter.attempts[ip] = 1
		return true
	}

	if count >= rateLimiter.limit {
		return false
	}
	rateLimiter.attempts[ip]++
	return true
}
