package db

import (
	"context"
	"database/sql"

	"github.com/chepyr/go-task-tracker/shared/models"
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
		ctx, query, board.ID, board.OwnerID, board.Title, board.Description, board.CreatedAt, board.UpdatedAt)
	return err
}

func (r *BoardRepository) GetByID(ctx context.Context, id string) (*models.Board, error) {
	query := `SELECT id, owner_id, title, description, created_at, updated_at FROM boards WHERE id = $1`
	board := &models.Board{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&board.ID, &board.OwnerID, &board.Title, &board.Description, &board.CreatedAt, &board.UpdatedAt,
	)
	return board, err
}
