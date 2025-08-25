package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/chepyr/go-task-tracker/shared/models"
	"github.com/google/uuid"
)

// defines methods for board db operations
type BoardRepositoryInterface interface {
	Create(ctx context.Context, board *models.Board) error
	GetByID(ctx context.Context, id string) (*models.Board, error)
}

type BoardRepository struct {
	db *sql.DB
}

func NewBoardRepository(db *sql.DB) *BoardRepository {
	return &BoardRepository{db: db}
}

func (r *BoardRepository) Create(ctx context.Context, board *models.Board) error {
	query := `INSERT INTO boards (id, owner_id, title, description, created_at, updated_at)
	 VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := r.db.ExecContext(
		ctx, query, board.ID, board.OwnerID, board.Title, board.Description,
		board.CreatedAt, board.UpdatedAt)
	return err
}

func (r *BoardRepository) GetByID(ctx context.Context, id string) (*models.Board, error) {
	query := `SELECT id, owner_id, title, description, created_at, updated_at
	 FROM boards WHERE id = $1`
	board := &models.Board{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&board.ID, &board.OwnerID, &board.Title, &board.Description,
		&board.CreatedAt, &board.UpdatedAt,
	)
	return board, err
}

func (r *BoardRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// check if exists
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM boards WHERE id = $1)`
	err := r.db.QueryRowContext(ctx, query, id).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("board with id %s does not exist", id)
	}

	query = `DELETE FROM boards WHERE id = $1`
	_, err = r.db.ExecContext(ctx, query, id)
	return err
}

func (r *BoardRepository) Update(ctx context.Context, board *models.Board) error {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM boards WHERE id = $1)`
	err := r.db.QueryRowContext(ctx, query, board.ID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("board with id %s does not exist", board.ID)
	}

	query = `UPDATE boards SET title = $1, description = $2, updated_at = $3 WHERE id = $4`
	_, err = r.db.ExecContext(ctx, query, board.Title, board.Description, board.UpdatedAt, board.ID)
	return err
}

func (r *BoardRepository) ListByUserID(ctx context.Context, ownerID string) ([]*models.Board, error) {
	query := `SELECT id, owner_id, title, description, created_at, updated_at
	 FROM boards WHERE owner_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var boards []*models.Board
	for rows.Next() {
		board := &models.Board{}
		if err := rows.Scan(
			&board.ID, &board.OwnerID, &board.Title, &board.Description,
			&board.CreatedAt, &board.UpdatedAt,
		); err != nil {
			return nil, err
		}
		boards = append(boards, board)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return boards, nil
}
