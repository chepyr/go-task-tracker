package db

import (
	"context"
	"database/sql"

	"github.com/chepyr/go-task-tracker/shared/models"
)

// defines methods for board db operations
type TaskRepositoryInterface interface {
	Create(ctx context.Context, board *models.Board) error
	GetByID(ctx context.Context, id string) (*models.Board, error)
}

type TaskRepository struct {
	db *sql.DB
}

func NewTaskRepository(db *sql.DB) *TaskRepository {
	return &TaskRepository{db: db}
}

func (r *TaskRepository) Create(ctx context.Context, task *models.Task) error {
	query := `INSERT INTO tasks (id, board_id, title, description, status, created_at, updated_at)
	 VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := r.db.ExecContext(
		ctx, query, task.ID, task.BoardID, task.Title, task.Description, task.Status, task.CreatedAt, task.UpdatedAt)
	return err
}

func (r *TaskRepository) GetByID(ctx context.Context, id string) (*models.Task, error) {
	query := `SELECT id, board_id, title, description, status, created_at, updated_at FROM tasks WHERE id = $1`
	task := &models.Task{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&task.ID, &task.BoardID, &task.Title, &task.Description, &task.Status, &task.CreatedAt, &task.UpdatedAt,
	)
	return task, err
}
