package handlers

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/chepyr/go-task-tracker/internal/models"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type MockUserRepository struct {
	users     map[string]*models.User
	createErr error
	getErr    error
	mutex     sync.Mutex
}

func NewMockUserRepository() *MockUserRepository {
	return &MockUserRepository{users: make(map[string]*models.User)}
}

func (m *MockUserRepository) Create(ctx context.Context, user *models.User) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.createErr != nil {
		return m.createErr
	}
	if _, exists := m.users[user.Email]; exists {
		return errors.New("email exists")
	}
	m.users[user.Email] = user
	return nil
}

func (m *MockUserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.getErr != nil {
		return nil, m.getErr
	}
	user, exists := m.users[email]
	if !exists {
		return nil, errors.New("user not found")
	}
	return user, nil
}

func SetupMockUser(email, password string) *MockUserRepository {
	repo := NewMockUserRepository()
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	repo.users[email] = &models.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: string(hash),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	return repo
}
