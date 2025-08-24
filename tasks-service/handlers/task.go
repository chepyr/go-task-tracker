package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"os/user"
	"strings"
	"time"

	"github.com/chepyr/go-task-tracker/shared/models"
	"github.com/google/uuid"
)

/*
handles routes:
- GET /tasks?board_id={board_id} - list tasks for a board
- POST /tasks - create a new task
*/
func (h *Handler) HandleTasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	 // GET /tasks?board_id={board_id}
	case http.MethodGet:
		h.listTasks(w, r)

	 // POST /tasks
	case http.MethodPost:
		h.createTask(w, r)
		
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) listTasks(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value("user_id").(string)
	if userID == "" {
		sendError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	boardIDStr := r.URL.Query().Get("board_id")
	if _, err := uuid.Parse(boardIDStr); err != nil {
		sendError(w, "board_id is required (uuid)", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	b, err := h.BoardRepo.GetByID(ctx, boardIDStr)
	if err != nil || b == nil {
		sendError(w, "Board not found", http.StatusNotFound)
		return
	}
	if b.OwnerID.String() != userID {
		sendError(w, "Forbidden", http.StatusForbidden)
		return
	}

	tasks, err := h.TaskRepo.ListByBoardID(ctx, boardIDStr)
	if err != nil {
		sendError(w, "Failed to list tasks", http.StatusInternalServerError)
		return
	}
	sendTasksJSON(w, tasks)
}

func (h *Handler) createTask(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value("user_id").(string)
		if userID == "" {
		sendError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !isJSONContentType(r) {
		sendError(w, "Content-Type must be application/json", http.StatusBadRequest)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB
	var input struct {
		BoardID     string `json:"board_id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Status 	string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		sendError(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}
	if input.Title == "" || input.BoardID == "" {
		sendError(w, "title and board_id are required", http.StatusBadRequest)
		return
	}

	boardID, err := uuid.Parse(input.BoardID)
	if err != nil {
		sendError(w, "board_id must be a valid uuid", http.StatusBadRequest)
		return
	}

	// check if board exists and belongs to user
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	board, err := h.BoardRepo.GetByID(ctx, input.BoardID)
	if err != nil || board == nil {
		sendError(w, "Board not found", http.StatusNotFound)
		return
	}
	if board.OwnerID.String() != userID {
		sendError(w, "Forbidden", http.StatusForbidden)
		return
	}

	status := normalizeStatus(input.Status)
	if status == "" {
		status = "todo"
	}
	now := time.Now().UTC()
	task := &models.Task{
		ID:          uuid.New(),
		BoardID:     boardID,
		Title:       input.Title,
		Description: input.Description,
		Status:      models.TaskStatus(status),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := h.TaskRepo.Create(ctx, task); err != nil {
		sendError(w, "Failed to create task", http.StatusInternalServerError)
		return
	}
	h.WSHub.BroadcastTaskUpdate(boardID, task)
	w.Header().Set("Location", "/tasks/"+task.ID.String())
	sendTasksJSON(w, []*models.Task{task})
}

/* 
routes:
- GET /tasks/{id}, 
- PUT/PATCH /tasks/{id}, 
- DELETE /tasks/{id}
*/
func (h *Handler) HandleTaskByID(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Not implemented", http.StatusNotImplemented)
}

func sendTasksJSON(w http.ResponseWriter, tasks []*models.Task) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

func normalizeStatus(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "todo":
		return "todo"
	case "in-progress", "in_progress", "inprogress":
		return "in-progress"
	case "done":
		return "done"
	default:
		return ""
	}
}

