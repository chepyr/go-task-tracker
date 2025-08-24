package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/chepyr/go-task-tracker/shared/models"
	"github.com/chepyr/go-task-tracker/tasks-service/db"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type Handler struct {
	BoardRepo   *db.BoardRepository
	TaskRepo    *db.TaskRepository
	RateLimiter *RateLimiter
	WSHub       *WSHub
}

type WSHub struct {
	connections map[uuid.UUID]map[*websocket.Conn]bool
	mutex       sync.Mutex
}

func NewWSHub() *WSHub {
	return &WSHub{connections: make(map[uuid.UUID]map[*websocket.Conn]bool)}
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

// BroadcastTaskUpdate sends a task update to all WebSocket connections for a given board.
func (h *WSHub) BroadcastTaskUpdate(boardID uuid.UUID, task *models.Task) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	conns, exists := h.connections[boardID]
	if !exists {
		return
	}

	message, err := json.Marshal(map[string]interface{}{
		"event":   "task_updated",
		"task_id": task.ID,
		"title":   task.Title,
		"status":  task.Status,
	})
	if err != nil {
		log.Printf("Failed to marshal task update: %v", err)
		return
	}

	for conn := range conns {
		if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
			log.Printf("Failed to send WebSocket message: %v", err)
			delete(conns, conn)
			conn.Close()
		}
	}
}

func (h *Handler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Check rate limiting
	clientIP := r.RemoteAddr
	if !h.RateLimiter.Allow(clientIP) {
		sendError(w, "Too many WebSocket connection attempts", http.StatusTooManyRequests)
		return
	}

	// Upgrade HTTP connection to WebSocket
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Adjust for production (e.g., check specific origins)
		},
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		sendError(w, "WebSocket upgrade failed", http.StatusInternalServerError)
		return
	}

	// Extract board ID from query parameters
	boardIDStr := r.URL.Query().Get("board_id")
	boardID, err := uuid.Parse(boardIDStr)
	if err != nil {
		log.Printf("Invalid board ID: %v", err)
		conn.Close()
		return
	}

	// Verify board exists and user has access
	_, err = h.BoardRepo.GetByID(r.Context(), boardIDStr)
	if err != nil {
		log.Printf("Board not found or unauthorized: %v", err)
		conn.Close()
		return
	}

	// Register connection in WSHub
	h.WSHub.mutex.Lock()
	if h.WSHub.connections[boardID] == nil {
		h.WSHub.connections[boardID] = make(map[*websocket.Conn]bool)
	}
	h.WSHub.connections[boardID][conn] = true
	h.WSHub.mutex.Unlock()

	// Handle WebSocket messages
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket error: %v", err)
			h.WSHub.mutex.Lock()
			delete(h.WSHub.connections[boardID], conn)
			h.WSHub.mutex.Unlock()
			conn.Close()
			return
		}
		// Optionally handle incoming messages from clients
	}
}

func sendError(w http.ResponseWriter, msg string, code int) {
	http.Error(w, msg, code)
}
