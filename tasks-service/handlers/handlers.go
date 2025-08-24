package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
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

func clientIP(r *http.Request) string {
	if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
		parts := strings.Split(xf, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}

// BroadcastTaskUpdate sends a task update to all WebSocket connections for a given board.
func (h *WSHub) BroadcastTaskUpdate(boardID uuid.UUID, task *models.Task) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	conns, exists := h.connections[boardID]
	if !exists {
		return
	}

	message, err := json.Marshal(map[string]any{
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
	clientIP := clientIP(r)
	if !h.RateLimiter.Allow(clientIP) {
		sendError(w, "Too many WebSocket connection attempts", http.StatusTooManyRequests)
		return
	}

	conn, boardID, _, err := h.upgradeAndAuthorize(w, r)
	if err != nil {
		log.Printf("WebSocket auth/upgrade failed: %v", err)
		return
	}

	h.WSHub.register(boardID, conn)
	h.setupKeepAlive(boardID, conn)

	h.readLoop(boardID, conn)
}

func (h *Handler) upgradeAndAuthorize(w http.ResponseWriter, r *http.Request) (*websocket.Conn, uuid.UUID, string, error) {
	upgrader := websocket.Upgrader{CheckOrigin: checkOrigin}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, uuid.Nil, "", err
	}

	boardIDStr := r.URL.Query().Get("board_id")
	boardID, err := uuid.Parse(boardIDStr)
	if err != nil {
		conn.Close()
		return nil, uuid.Nil, "", fmt.Errorf("invalid board id")
	}

	uid, _ := r.Context().Value("user_id").(string)
	board, err := h.BoardRepo.GetByID(r.Context(), boardIDStr)
	if err != nil || board.OwnerID.String() != uid {
		conn.Close()
		return nil, uuid.Nil, "", fmt.Errorf("forbidden")
	}

	return conn, boardID, uid, nil
}

func checkOrigin(r *http.Request) bool {
	allowed := strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",")
	origin := r.Header.Get("Origin")

	if len(allowed) == 0 || (len(allowed) == 1 && strings.TrimSpace(allowed[0]) == "") {
		return true
	}

	for _, a := range allowed {
		if strings.TrimSpace(a) == origin {
			return true
		}
	}
	return false
}

func (hub *WSHub) register(boardID uuid.UUID, conn *websocket.Conn) {
	hub.mutex.Lock()
	defer hub.mutex.Unlock()
	if hub.connections[boardID] == nil {
		hub.connections[boardID] = make(map[*websocket.Conn]bool)
	}
	hub.connections[boardID][conn] = true
}

func (hub *WSHub) unregister(boardID uuid.UUID, conn *websocket.Conn) {
	hub.mutex.Lock()
	defer hub.mutex.Unlock()
	delete(hub.connections[boardID], conn)
}

func (h *Handler) setupKeepAlive(boardID uuid.UUID, conn *websocket.Conn) {
	conn.SetReadLimit(1 << 20)
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			<-ticker.C
			if err := conn.WriteControl(
				websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second),
			); err != nil {
				h.WSHub.unregister(boardID, conn)
				conn.Close()
				return
			}
		}
	}()
}

func (h *Handler) readLoop(boardID uuid.UUID, conn *websocket.Conn) {
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket closed: %v", err)
			h.WSHub.unregister(boardID, conn)
			conn.Close()
			break
		}
	}
}

func sendError(w http.ResponseWriter, msg string, code int) {
	http.Error(w, msg, code)
}
