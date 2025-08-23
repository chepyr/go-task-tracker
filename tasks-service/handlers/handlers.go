package handlers

import (
	"net/http"
	"sync"
	"time"

	"github.com/chepyr/go-task-tracker/shared/models"
	"github.com/chepyr/go-task-tracker/tasks-service/db"
	"github.com/gorilla/websocket"
)

type Handler struct {
	BoardRepo   *db.BoardRepository
	TaskRepo    *db.TaskRepository
	RateLimiter *RateLimiter
	WSHub       *WSHub
}

type WSHub struct {
	connections map[models.UUID]map[*websocket.Conn]bool
	mutex       sync.Mutex
}

func NewWSHub() *WSHub {
	return &WSHub{connections: make(map[models.UUID]map[*websocket.Conn]bool)}
}

type RateLimiter struct {
	attempts map[string]int
	limit    int
	mutex    sync.Mutex
	window   time.Duration
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		attempts: make(map[string]int),
		limit:    limit,
		window:   window,
	}
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) Allow(ip string) bool {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	count, exists := rl.attempts[ip]
	if !exists {
		rl.attempts[ip] = 1
		return true
	}
	if count >= rl.limit {
		return false
	}
	rl.attempts[ip]++
	return true
}

func (rl *RateLimiter) cleanup() {
	for {
		time.Sleep(rl.window)
		rl.mutex.Lock()
		rl.attempts = make(map[string]int)
		rl.mutex.Unlock()
	}
}

func sendError(w http.ResponseWriter, msg string, code int) {
	http.Error(w, msg, code)
}
