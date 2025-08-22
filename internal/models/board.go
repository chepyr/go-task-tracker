package models

import (
	"github.com/google/uuid"
	"time"
)

type Board struct {
	ID          uuid.UUID
	OwnerID     uuid.UUID
	Title       string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
