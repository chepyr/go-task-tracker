package db

import (
	"context"
	"database/sql"
	"testing"

	"github.com/chepyr/go-task-tracker/internal/models"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
		return nil
	}

	_, err = db.Exec(`CREATE TABLE users (
		id TEXT PRIMARY KEY,
		email VARCHAR(255) NOT NULL UNIQUE,
		password_hash VARCHAR(255) NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		t.Fatalf("Failed to create users table: %v", err)
	}

	return db
}

func TestUserRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewUserRepository(db)
	user := &models.User{
		ID:           uuid.New(),
		Email:        "test_1@example.com",
		PasswordHash: "password",
	}

	err := repo.Create(context.Background(), user)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// verify user was created
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM users WHERE email = $1", user.Email).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query user: %v", err)
	}
	if count != 1 {
		t.Fatalf("Expected 1 user, got %d", count)
	}
}

func TestUserRepository_GetByEmail(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewUserRepository(db)
	user := &models.User{
		ID:           uuid.New(),
		Email:        "test_1@example.com",
		PasswordHash: "password",
	}

	// Insert user directly
	_, err := db.Exec(
		"INSERT INTO users (id, email, password_hash, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		user.ID, user.Email, user.PasswordHash, user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		t.Fatalf("Failed to insert test user: %v", err)
	}

	// check
	fetchedUser, err := repo.GetByEmail(context.Background(), user.Email)
	if err != nil {
		t.Errorf("GetByEmail failed: %v", err)
	}
	if fetchedUser.ID != user.ID {
		t.Errorf("Expected ID %v, got %v", user.ID, fetchedUser.ID)
	}
	if fetchedUser.Email != user.Email {
		t.Errorf("Expected email %v, got %v", user.Email, fetchedUser.Email)
	}
	if fetchedUser.PasswordHash != user.PasswordHash {
		t.Errorf("Expected password hash %v, got %v", user.PasswordHash, fetchedUser.PasswordHash)
	}
}

func TestUserRepository_GetByEmail_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewUserRepository(db)
	_, err := repo.GetByEmail(context.Background(), "nonexistent@example.com")
	if err == nil {
		t.Error("Expected error for non-existent email, got none")
	}
	if err != sql.ErrNoRows {
		t.Errorf("Expected sql.ErrNoRows, got %v", err)
	}
}
