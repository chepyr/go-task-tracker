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
GET /boards - list boards
POST /boards - create board
*/
func (h *Handler) HandleBoards(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listBoards(w, r)
	case http.MethodPost:
		h.createBoard(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) HandleBoardByID(w http.ResponseWriter, r *http.Request) {
	boardID := strings.TrimPrefix(r.URL.Path, "/boards/")
	if boardID == "" {
		http.Error(w, "Board ID is required", http.StatusBadRequest)
		return
	}
	if _, err := uuid.Parse(boardID); err != nil {
		http.Error(w, "Invalid board ID", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.GetBoard(w, r, boardID)
	case http.MethodPut:
		h.UpdateBoard(w, r, boardID)
	case http.MethodDelete:
		h.DeleteBoard(w, r, boardID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) DeleteBoard(w http.ResponseWriter, r *http.Request, boardID string) {
	userId, _ := r.Context().Value("user_id").(string)
	if userId == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	board, err := h.BoardRepo.GetByID(ctx, boardID)
	if err != nil || board == nil {
		http.Error(w, "Board not found", http.StatusNotFound)
		return
	}
	if board.OwnerID.String() != userId {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := h.BoardRepo.Delete(ctx, board.ID); err != nil {
		http.Error(w, "Failed to delete board", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) UpdateBoard(w http.ResponseWriter, r *http.Request, boardID string) {
	userId, _ := r.Context().Value("user_id").(string)
	if userId == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	board, err := h.BoardRepo.GetByID(ctx, boardID)
	if err != nil || board == nil {
		http.Error(w, "Board not found", http.StatusNotFound)
		return
	}
	if board.OwnerID.String() != userId {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if !isJSONContentType(r) {
		http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var input struct{ Title, Description *string }
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid JSON body", 400)
		return
	}
	updated := *board
	if input.Title != nil {
		updatedTitle := strings.TrimSpace(*input.Title)
		if updatedTitle == "" || len(updatedTitle) > 100 {
			http.Error(w, "Title is required and must be <= 100 characters", http.StatusBadRequest)
			return
		}
		updated.Title = updatedTitle
	}
	if input.Description != nil {
		if len(*input.Description) > 500 {
			http.Error(w, "Description must be <= 500 characters", http.StatusBadRequest)
			return
		}
		updated.Description = *input.Description
	}
	updated.UpdatedAt = time.Now().UTC()
	if err := h.BoardRepo.Update(ctx, &updated); err != nil {
		http.Error(w, "Failed to update board", 500)
		return
	}
	sendBoardsJSON(w, []*models.Board{&updated})
}

func (h *Handler) GetBoard(w http.ResponseWriter, r *http.Request, boardID string) {
	userId, _ := r.Context().Value("user_id").(string)
	if userId == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	board, err := h.BoardRepo.GetByID(ctx, boardID)
	if err != nil || board == nil {
		http.Error(w, "Board not found", http.StatusNotFound)
		return
	}
	if board.OwnerID.String() != userId {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	sendBoardsJSON(w, []*models.Board{board})
}

func (h *Handler) listBoards(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value("user_id").(string)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	boards, err := h.BoardRepo.ListByUserID(ctx, userID)
	if err != nil {
		http.Error(w, "Failed to fetch boards", http.StatusInternalServerError)
		return
	}
	sendBoardsJSON(w, boards)
}

func (h *Handler) createBoard(w http.ResponseWriter, r *http.Request) {
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

	var newBoard struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&newBoard); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}
	newBoard.Title = strings.TrimSpace(newBoard.Title)
	if newBoard.Title == "" || len(newBoard.Title) > 100 {
		http.Error(w, "Title is required and must be <= 100 characters", http.StatusBadRequest)
		return
	}
	if len(newBoard.Description) > 500 {
		http.Error(w, "Description must be <= 500 characters", http.StatusBadRequest)
		return
	}

	now := time.Now().UTC()
	board := &models.Board{
		ID:          uuid.New(),
		OwnerID:     uuid.MustParse(userID),
		Title:       newBoard.Title,
		Description: newBoard.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := h.BoardRepo.Create(ctx, board); err != nil {
		http.Error(w, "Failed to create board", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Location", "/boards/"+board.ID.String())
	w.WriteHeader(http.StatusCreated)
}

func isJSONContentType(r *http.Request) bool {
	ct := r.Header.Get("Content-Type")
	return strings.HasPrefix(strings.ToLower(ct), "application/json")
}

func sendBoardsJSON(w http.ResponseWriter, boards []*models.Board) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(boards)
}
