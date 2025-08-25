package handlers

import (
	"context"
	"encoding/json"
	"net/http"
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
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	boardIDStr := r.URL.Query().Get("board_id")
	if _, err := uuid.Parse(boardIDStr); err != nil {
		http.Error(w, "board_id is required (uuid)", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	b, err := h.BoardRepo.GetByID(ctx, boardIDStr)
	if err != nil || b == nil {
		http.Error(w, "Board not found", http.StatusNotFound)
		return
	}
	if b.OwnerID.String() != userID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	tasks, err := h.TaskRepo.ListByBoardID(ctx, boardIDStr)
	if err != nil {
		http.Error(w, "Failed to list tasks", http.StatusInternalServerError)
		return
	}
	sendTasksJSON(w, tasks)
}

func (h *Handler) createTask(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value("user_id").(string)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !isJSONContentType(r) {
		http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB
	var input struct {
		BoardID     string `json:"board_id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Status      string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}
	if input.Title == "" || input.BoardID == "" {
		http.Error(w, "title and board_id are required", http.StatusBadRequest)
		return
	}

	boardID, err := uuid.Parse(input.BoardID)
	if err != nil {
		http.Error(w, "board_id must be a valid uuid", http.StatusBadRequest)
		return
	}

	// check if board exists and belongs to user
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	board, err := h.BoardRepo.GetByID(ctx, input.BoardID)
	if err != nil || board == nil {
		http.Error(w, "Board not found", http.StatusNotFound)
		return
	}
	if board.OwnerID.String() != userID {
		http.Error(w, "Forbidden", http.StatusForbidden)
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
		http.Error(w, "Failed to create task", http.StatusInternalServerError)
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
	taskIDstr := r.URL.Path[len("/tasks/"):]
	if taskIDstr == "" {
		// TODO http.Error => http.Error
		http.Error(w, "task_id is required", http.StatusBadRequest)
		return
	}
	taskID, err := uuid.Parse(taskIDstr)
	if err != nil {
		http.Error(w, "task_id must be a valid uuid", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getTaskByID(w, r, taskID)
	case http.MethodPut, http.MethodPatch:
		h.updateTaskByID(w, r, taskID)
	case http.MethodDelete:
		h.deleteTaskByID(w, r, taskID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
}

func (h *Handler) getTaskByID(w http.ResponseWriter, r *http.Request, taskID uuid.UUID) {
	userID, _ := r.Context().Value("user_id").(string)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	task, err := h.TaskRepo.GetByID(ctx, taskID.String())
	if err != nil || task == nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	board, err := h.BoardRepo.GetByID(ctx, task.BoardID.String())
	if err != nil || board == nil {
		http.Error(w, "Board not found", http.StatusNotFound)
		return
	}
	if board.OwnerID.String() != userID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	sendTasksJSON(w, []*models.Task{task})
}

func (h *Handler) updateTaskByID(w http.ResponseWriter, r *http.Request, taskID uuid.UUID) {
	userID, _ := r.Context().Value("user_id").(string)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !isJSONContentType(r) {
		http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	existingTask, err := h.TaskRepo.GetByID(ctx, taskID.String())
	if err != nil || existingTask == nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	board, err := h.BoardRepo.GetByID(ctx, existingTask.BoardID.String())
	if err != nil || board == nil {
		http.Error(w, "Board not found", http.StatusNotFound)
		return
	}
	if board.OwnerID.String() != userID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var input struct {
		Title       *string `json:"title"`
		Description *string `json:"description"`
		Status      *string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	// TODO: move validation to new functions
	if input.Title != nil {
		title := strings.TrimSpace(*input.Title)
		if title == "" {
			http.Error(w, "title cannot be empty", http.StatusBadRequest)
			return
		}
		if len(title) > 200 {
			http.Error(w, "title too long (max 200 chars)", http.StatusBadRequest)
			return
		}
		existingTask.Title = title
	}
	if input.Description != nil {
		desc := strings.TrimSpace(*input.Description)
		if len(desc) > 1000 {
			http.Error(w, "description too long (max 1000 chars)", http.StatusBadRequest)
			return
		}
		existingTask.Description = desc
	}
	if input.Status != nil {
		status := normalizeStatus(*input.Status)
		if status == "" {
			http.Error(w, "Invalid status value", http.StatusBadRequest)
			return
		}
		existingTask.Status = models.TaskStatus(status)
	}
	existingTask.UpdatedAt = time.Now().UTC()

	if err := h.TaskRepo.Update(ctx, existingTask); err != nil {
		http.Error(w, "Failed to update task", http.StatusInternalServerError)
		return
	}
	h.WSHub.BroadcastTaskUpdate(existingTask.BoardID, existingTask)
	sendTasksJSON(w, []*models.Task{existingTask})
}

func (h *Handler) deleteTaskByID(w http.ResponseWriter, r *http.Request, taskID uuid.UUID) {
	userID, _ := r.Context().Value("user_id").(string)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	existingTask, err := h.TaskRepo.GetByID(ctx, taskID.String())
	if err != nil || existingTask == nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	board, err := h.BoardRepo.GetByID(ctx, existingTask.BoardID.String())
	if err != nil || board == nil {
		http.Error(w, "Board not found", http.StatusNotFound)
		return
	}
	if board.OwnerID.String() != userID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := h.TaskRepo.Delete(ctx, taskID.String()); err != nil {
		http.Error(w, "Failed to delete task", http.StatusInternalServerError)
		return
	}
	// TODO: add WS notification for deletion
	// h.WSHub.BroadcastTaskDeletion(existingTask.BoardID, taskID)
	w.WriteHeader(http.StatusNoContent)
}

func sendTasksJSON(w http.ResponseWriter, tasks []*models.Task) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

// convert various user inputs to standard status values
func normalizeStatus(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "todo":
		return "todo"
	case "in-progress", "in_progress", "inprogress", "in progress":
		return "in-progress"
	case "done":
		return "done"
	default:
		return ""
	}
}
