package db

import (
	"context"
	"testing"
	"time"

	"github.com/chepyr/go-task-tracker/shared/models"
	"github.com/google/uuid"
)

func TestBoardRepository_CreateAndGetByID(t *testing.T) {
	dbx := setupTasksDB(t)
	defer dbx.Close()
	repo := NewBoardRepository(dbx)

	ownerID := uuid.New()
	board := &models.Board{
		ID:        uuid.New(),
		OwnerID:   ownerID,
		Title:     "Test Board",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if err := repo.Create(context.Background(), board); err != nil {
		t.Fatalf("Create board: %v", err)
	}

	created, err := repo.GetByID(context.Background(), board.ID.String())
	if err != nil {
		t.Fatalf("GetByID after create: %v", err)
	}

	if created.ID != board.ID || created.Title != board.Title || created.OwnerID != board.OwnerID {
		t.Errorf("Created board does not match: got %+v, want %+v", created, board)
	}
}

func TestBoardRepository_GetByInvalidID(t *testing.T) {
	dbx := setupTasksDB(t)
	defer dbx.Close()
	repo := NewBoardRepository(dbx)

	_, err := repo.GetByID(context.Background(), "invalid-uuid")
	if err == nil {
		t.Fatal("Expected error for invalid UUID, got nil")
	}
}

func TestBoardRepository_Delete(t *testing.T) {
	dbx := setupTasksDB(t)
	defer dbx.Close()
	repo := NewBoardRepository(dbx)

	ownerID := uuid.New()
	board := &models.Board{
		ID:        uuid.New(),
		OwnerID:   ownerID,
		Title:     "Board to Delete",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if err := repo.Create(context.Background(), board); err != nil {
		t.Fatalf("Create board: %v", err)
	}

	if err := repo.Delete(context.Background(), board.ID.String()); err != nil {
		t.Fatalf("Delete board: %v", err)
	}

	_, err := repo.GetByID(context.Background(), board.ID.String())
	if err == nil {
		t.Fatal("Expected error for deleted board, got nil")
	}
}

func TestBoardRepository_Update(t *testing.T) {
	dbx := setupTasksDB(t)
	defer dbx.Close()
	repo := NewBoardRepository(dbx)

	ownerID := uuid.New()
	board := &models.Board{
		ID:        uuid.New(),
		OwnerID:   ownerID,
		Title:     "Board to Update",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if err := repo.Create(context.Background(), board); err != nil {
		t.Fatalf("Create board: %v", err)
	}

	board.Title = "Updated Title"
	board.Description = "Updated Description"
	board.UpdatedAt = time.Now().UTC()

	if err := repo.Update(context.Background(), board); err != nil {
		t.Fatalf("Update board: %v", err)
	}

	updated, err := repo.GetByID(context.Background(), board.ID.String())
	if err != nil {
		t.Fatalf("GetByID after update: %v", err)
	}

	if updated.Title != "Updated Title" || updated.Description != "Updated Description" {
		t.Errorf("Board not updated correctly: got %+v", updated)
	}
}

func TestBoardRepository_ListByUserID(t *testing.T) {
	dbx := setupTasksDB(t)
	defer dbx.Close()
	repo := NewBoardRepository(dbx)

	ownerID := uuid.New()
	board1 := &models.Board{
		ID:        uuid.New(),
		OwnerID:   ownerID,
		Title:     "Board 1",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	board2 := &models.Board{
		ID:        uuid.New(),
		OwnerID:   ownerID,
		Title:     "Board 2",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if err := repo.Create(context.Background(), board1); err != nil {
		t.Fatalf("Create board1: %v", err)
	}
	if err := repo.Create(context.Background(), board2); err != nil {
		t.Fatalf("Create board2: %v", err)
	}

	boards, err := repo.ListByUserID(context.Background(), ownerID.String())
	if err != nil {
		t.Fatalf("ListByUserID: %v", err)
	}

	if len(boards) != 2 {
		t.Errorf("Expected 2 boards, got %d", len(boards))
	}
}

func TestBoardRepository_Delete_NonExistent(t *testing.T) {
	dbx := setupTasksDB(t)
	defer dbx.Close()
	repo := NewBoardRepository(dbx)

	err := repo.Delete(context.Background(), uuid.New().String())
	if err == nil {
		t.Fatal("Expected error when deleting non-existent board, got nil")
	}
}

func TestBoardRepository_Update_NonExistent(t *testing.T) {
	dbx := setupTasksDB(t)
	defer dbx.Close()
	repo := NewBoardRepository(dbx)

	board := &models.Board{
		ID:        uuid.New(),
		OwnerID:   uuid.New(),
		Title:     "Non-existent Board",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	err := repo.Update(context.Background(), board)
	if err == nil {
		t.Fatal("Expected error when updating non-existent board, got nil")
	}
}

// invalid title & description length
func TestBoardRepository_Create_InvalidData(t *testing.T) {
	dbx := setupTasksDB(t)
	defer dbx.Close()
	repo := NewBoardRepository(dbx)

	ownerID := uuid.New()
	boardEmptyTitle := &models.Board{
		ID:          uuid.New(),
		OwnerID:     ownerID,
		Title:       "", // invalid empty title
		Description: "Valid Description",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	err := repo.Create(context.Background(), boardEmptyTitle)
	if err == nil {
		t.Fatal("Expected error when creating board with invalid data, got nil")
	}

	boardLongTitle := &models.Board{
		ID:          uuid.New(),
		OwnerID:     ownerID,
		Title:       string(make([]byte, 101)), // invalid title > 100 chars
		Description: "Valid Description",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	err = repo.Create(context.Background(), boardLongTitle)
	if err == nil {
		t.Fatal("Expected error when creating board with too long title, got nil")
	}

	boardLongDesc := &models.Board{
		ID:          uuid.New(),
		OwnerID:     ownerID,
		Title:       "Valid Title",
		Description: string(make([]byte, 501)), // invalid description > 500 chars
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	err = repo.Create(context.Background(), boardLongDesc)
	if err == nil {
		t.Fatal("Expected error when creating board with too long description, got nil")
	}
}
