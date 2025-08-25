package db

import (
	"context"
	"database/sql"
	"log"
	"testing"
	"time"

	"github.com/chepyr/go-task-tracker/shared/models"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

func setupTasksDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// minimal schema for boards and tasks
	ddl := `
CREATE TABLE boards (
  id TEXT PRIMARY KEY,
  owner_id TEXT NOT NULL,
  title TEXT NOT NULL,
  description TEXT,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL
);
CREATE TABLE tasks (
  id TEXT PRIMARY KEY,
  board_id TEXT NOT NULL,
  title TEXT NOT NULL,
  description TEXT,
  status TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL
);
CREATE INDEX idx_boards_owner_id ON boards(owner_id);
CREATE INDEX idx_tasks_board_id ON tasks(board_id);
`
	if _, err := db.Exec(ddl); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return db
}

func insertBoard(t *testing.T, dbx *sql.DB, owner uuid.UUID) models.Board {
	t.Helper()
	now := time.Now().UTC()
	b := models.Board{
		ID:          uuid.New(),
		OwnerID:     owner,
		Title:       "Board A",
		Description: "desc",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	_, err := dbx.Exec(`INSERT INTO boards (id, owner_id, title, description, created_at, updated_at)
	                    VALUES ($1,$2,$3,$4,$5,$6)`,
		b.ID, b.OwnerID, b.Title, b.Description, b.CreatedAt, b.UpdatedAt)
	if err != nil {
		t.Fatalf("insert board: %v", err)
	}
	return b
}

func TestTaskRepository_Create_Get_Update_Delete_List(t *testing.T) {
	dbx := setupTasksDB(t)
	defer func() {
		if err := dbx.Close(); err != nil {
			log.Printf("close db: %v", err)
		}
	}()

	taskRepo := NewTaskRepository(dbx)
	boardRepo := NewBoardRepository(dbx)

	owner := uuid.New()
	b := insertBoard(t, dbx, owner)

	// sanity check GetByID for board
	if _, err := boardRepo.GetByID(context.Background(), b.ID.String()); err != nil {
		t.Fatalf("board GetByID failed: %v", err)
	}

	// Create
	now := time.Now().UTC()
	task := &models.Task{
		ID:          uuid.New(),
		BoardID:     b.ID,
		Title:       "First task",
		Description: "hello",
		Status:      "todo",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := taskRepo.Create(context.Background(), task); err != nil {
		t.Fatalf("TaskRepository.Create: %v", err)
	}

	// GetByID
	got, err := taskRepo.GetByID(context.Background(), task.ID.String())
	if err != nil {
		t.Fatalf("TaskRepository.GetByID: %v", err)
	}
	if got.ID != task.ID || got.Title != "First task" || got.Status != "todo" {
		t.Errorf("GetByID mismatch: %#v", got)
	}

	// Update
	got.Title = "Updated"
	got.Status = "in-progress"
	got.UpdatedAt = time.Now().UTC()
	if err := taskRepo.Update(context.Background(), got); err != nil {
		t.Fatalf("TaskRepository.Update: %v", err)
	}
	after, err := taskRepo.GetByID(context.Background(), task.ID.String())
	if err != nil {
		t.Fatalf("TaskRepository.GetByID after update: %v", err)
	}
	if after.Title != "Updated" || after.Status != "in-progress" {
		t.Errorf("Update not applied: %#v", after)
	}

	// ListByBoardID
	list, err := taskRepo.ListByBoardID(context.Background(), b.ID.String())
	if err != nil {
		t.Fatalf("TaskRepository.ListByBoardID: %v", err)
	}
	if len(list) != 1 || list[0].ID != task.ID {
		t.Errorf("ListByBoardID unexpected: %+v", list)
	}

	// Delete
	if err := taskRepo.Delete(context.Background(), task.ID.String()); err != nil {
		t.Fatalf("TaskRepository.Delete: %v", err)
	}
	_, err = taskRepo.GetByID(context.Background(), task.ID.String())
	if err == nil {
		t.Errorf("expected error on GetByID after delete, got nil")
	}
}

func TestTaskRepository_Create_InvalidBoard(t *testing.T) {
	dbx := setupTasksDB(t)
	defer func() {
		if err := dbx.Close(); err != nil {
			log.Printf("close db: %v", err)
		}
	}()

	taskRepo := NewTaskRepository(dbx)

	// Create task with non-existing board_id
	now := time.Now().UTC()
	task := &models.Task{
		ID:          uuid.New(),
		BoardID:     uuid.New(), // random board ID, does not exist
		Title:       "Orphan task",
		Description: "no board",
		Status:      "todo",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	err := taskRepo.Create(context.Background(), task)
	if err == nil {
		t.Fatal("expected error when creating task with invalid board_id, got nil")
	}
}

func TestTaskRepository_GetByID_NonExistent(t *testing.T) {
	dbx := setupTasksDB(t)
	defer func() {
		if err := dbx.Close(); err != nil {
			log.Printf("close db: %v", err)
		}
	}()

	taskRepo := NewTaskRepository(dbx)

	// GetByID for non-existent task
	_, err := taskRepo.GetByID(context.Background(), uuid.New().String())
	if err == nil {
		t.Fatal("expected error when getting non-existent task, got nil")
	}
}

func TestTaskRepository_Delete_NonExistent(t *testing.T) {
	dbx := setupTasksDB(t)
	defer func() {
		if err := dbx.Close(); err != nil {
			log.Printf("close db: %v", err)
		}
	}()

	taskRepo := NewTaskRepository(dbx)

	// Delete non-existent task
	err := taskRepo.Delete(context.Background(), uuid.New().String())
	if err == nil {
		t.Fatal("expected error when deleting non-existent task, got nil")
	}
}

func TestTaskRepository_Update_NonExistent(t *testing.T) {
	dbx := setupTasksDB(t)
	defer func() {
		if err := dbx.Close(); err != nil {
			log.Printf("close db: %v", err)
		}
	}()

	taskRepo := NewTaskRepository(dbx)

	// Update non-existent task
	now := time.Now().UTC()
	task := &models.Task{
		ID:          uuid.New(),
		BoardID:     uuid.New(),
		Title:       "Non-existent",
		Description: "nope",
		Status:      "todo",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	err := taskRepo.Update(context.Background(), task)
	if err == nil {
		t.Fatal("expected error when updating non-existent task, got nil")
	}
}

func TestTaskRepository_ListByBoardID_Empty(t *testing.T) {
	dbx := setupTasksDB(t)
	defer func() {
		if err := dbx.Close(); err != nil {
			log.Printf("close db: %v", err)
		}
	}()

	taskRepo := NewTaskRepository(dbx)

	// ListByBoardID for board with no tasks
	boardID := uuid.New().String()
	list, err := taskRepo.ListByBoardID(context.Background(), boardID)
	if err != nil {
		t.Fatalf("TaskRepository.ListByBoardID: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected empty list for board with no tasks, got %+v", list)
	}
}

// TODO: benchmark?
