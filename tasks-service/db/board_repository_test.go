package db

import (
	"context"
	"testing"
	"time"

	"github.com/chepyr/go-task-tracker/shared/models"
	"github.com/google/uuid"
)

func TestTaskRepository_CreateAndGetByID(t *testing.T) {
	dbx := setupTasksDB(t)
	defer dbx.Close()
	repo := NewTaskRepository(dbx)

	ownerID := uuid.New()
	board := insertBoard(t, dbx, ownerID)

	now := time.Now().UTC()
	task := &models.Task{
		ID:          uuid.New(),
		BoardID:     board.ID,
		Title:       "Task 1",
		Description: "Task description",
		Status:      "todo",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Test Create
	if err := repo.Create(context.Background(), task); err != nil {
		t.Fatalf("Create task: %v", err)
	}

	// Test GetByID
	got, err := repo.GetByID(context.Background(), task.ID.String())
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID != task.ID || got.BoardID != task.BoardID || got.Title != task.Title ||
		got.Description != task.Description || got.Status != task.Status {
		t.Errorf("GetByID returned incorrect data: got %+v, want %+v", got, task)
	}
}

// update and delete
func TestBoardRepository_Update_Delete(t *testing.T) {
	dbx := setupTasksDB(t)
	defer dbx.Close()
	repo := NewTaskRepository(dbx)

	ownerID := uuid.New()
	board := insertBoard(t, dbx, ownerID)

	now := time.Now().UTC()
	task := &models.Task{
		ID:          uuid.New(),
		BoardID:     board.ID,
		Title:       "Task 1",
		Description: "Task description",
		Status:      "todo",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := repo.Create(context.Background(), task); err != nil {
		t.Fatalf("Create task: %v", err)
	}

	// Update
	task.Title = "Updated Task"
	task.Status = "in-progress"
	task.UpdatedAt = time.Now().UTC()
	if err := repo.Update(context.Background(), task); err != nil {
		t.Fatalf("Update task: %v", err)
	}

	updated, err := repo.GetByID(context.Background(), task.ID.String())
	if err != nil {
		t.Fatalf("GetByID after update: %v", err)
	}
	if updated.Title != "Updated Task" || updated.Status != "in-progress" {
		t.Errorf("Update did not persist changes: got %+v", updated)
	}

	// Delete
	if err := repo.Delete(context.Background(), task.ID.String()); err != nil {
		t.Fatalf("Delete task: %v", err)
	}
	_, err = repo.GetByID(context.Background(), task.ID.String())
	if err == nil {
		t.Fatal("Expected error when getting deleted task, got nil")
	}
}

func TestBoardRepository_Create_InvalidBoardID(t *testing.T) {
	dbx := setupTasksDB(t)
	defer dbx.Close()
	repo := NewTaskRepository(dbx)

	now := time.Now().UTC()
	task := &models.Task{
		ID:          uuid.New(),
		BoardID:     uuid.New(), // Non-existent board ID
		Title:       "Task 1",
		Description: "Task description",
		Status:      "todo",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	err := repo.Create(context.Background(), task)
	if err == nil {
		t.Fatal("Expected error when creating task with non-existent board_id, got nil")
	}
}

func TestBoardRepository_Update_Delete_InvalidBoardID(t *testing.T) {
	dbx := setupTasksDB(t)
	defer dbx.Close()
	repo := NewTaskRepository(dbx)

	ownerID := uuid.New()
	board := insertBoard(t, dbx, ownerID)

	now := time.Now().UTC()
	task := &models.Task{
		ID:          uuid.New(),
		BoardID:     board.ID,
		Title:       "Task 1",
		Description: "Task description",
		Status:      "todo",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := repo.Create(context.Background(), task); err != nil {
		t.Fatalf("Create task: %v", err)
	}

	// Update with invalid BoardID
	task.BoardID = uuid.New() // Non-existent board ID
	task.UpdatedAt = time.Now().UTC()
	err := repo.Update(context.Background(), task)
	if err == nil {
		t.Fatal("Expected error when updating task with invalid board_id, got nil")
	}

	// Delete with invalid ID
	err = repo.Delete(context.Background(), uuid.New().String())
	if err == nil {
		t.Fatal("Expected error when deleting task with invalid id, got nil")
	}
}

func TestBoardRepository_GetByID_NonExistent(t *testing.T) {
	dbx := setupTasksDB(t)
	defer dbx.Close()
	repo := NewTaskRepository(dbx)

	// GetByID for non-existent task
	_, err := repo.GetByID(context.Background(), uuid.New().String())
	if err == nil {
		t.Fatal("Expected error when getting non-existent task, got nil")
	}
}
