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

func (r *TaskRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM tasks WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *TaskRepository) Update(ctx context.Context, task *models.Task) error {
	query := `UPDATE tasks SET title = $1, description = $2, status = $3, updated_at = $4 WHERE id = $5`
	_, err := r.db.ExecContext(ctx, query, task.Title, task.Description, task.Status, task.UpdatedAt, task.ID)
	return err
}

func (r *TaskRepository) ListByBoardID(ctx context.Context, boardID string) ([]*models.Task, error) {
	query := `SELECT id, board_id, title, description, status, created_at, updated_at
	 FROM tasks WHERE board_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*models.Task
	for rows.Next() {
		task := &models.Task{}
		if err := rows.Scan(
			&task.ID, &task.BoardID, &task.Title, &task.Description,
			&task.Status, &task.CreatedAt, &task.UpdatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tasks, nil
}


