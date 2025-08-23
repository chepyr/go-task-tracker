package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chepyr/go-task-tracker/shared/models"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func TestLogin(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		body           string
		mockRepo       *MockUserRepository
		rateLimitAllow bool
		setEnv         bool
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "Success",
			method:         http.MethodPost,
			body:           `{"email": "test@example.com", "password": "strongpass"}`,
			mockRepo:       setupMockUser("test@example.com", "strongpass"),
			rateLimitAllow: true,
			setEnv:         true,
			expectedStatus: http.StatusOK,
			expectedBody:   `"user_email":"test@example.com"`,
		},
		{
			name:           "Invalid method",
			method:         http.MethodGet,
			body:           ``,
			mockRepo:       NewMockUserRepository(),
			rateLimitAllow: true,
			setEnv:         true,
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   `"error":"Use POST method for login"`,
		},
		{
			name:           "Invalid JSON",
			method:         http.MethodPost,
			body:           `{"email": "test@example.com", "password": }`,
			mockRepo:       NewMockUserRepository(),
			rateLimitAllow: true,
			setEnv:         true,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `"error":"Bad JSON"`,
		},
		{
			name:           "Invalid email",
			method:         http.MethodPost,
			body:           `{"email": "invalid", "password": "strongpass"}`,
			mockRepo:       NewMockUserRepository(),
			rateLimitAllow: true,
			setEnv:         true,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `"error":"Invalid email"`,
		},
		{
			name:           "Password too short",
			method:         http.MethodPost,
			body:           `{"email": "test@example.com", "password": "abc"}`,
			mockRepo:       NewMockUserRepository(),
			rateLimitAllow: true,
			setEnv:         true,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `"error":"Password must be at least 4 characters long"`,
		},
		{
			name:           "Rate limit exceeded",
			method:         http.MethodPost,
			body:           `{"email": "test@example.com", "password": "strongpass"}`,
			mockRepo:       NewMockUserRepository(),
			rateLimitAllow: false,
			setEnv:         true,
			expectedStatus: http.StatusTooManyRequests,
			expectedBody:   `"error":"Too many login attempts`,
		},
		{
			name:           "User not found",
			method:         http.MethodPost,
			body:           `{"email": "test@example.com", "password": "strongpass"}`,
			mockRepo:       NewMockUserRepository(),
			rateLimitAllow: true,
			setEnv:         true,
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `"error":"Invalid email or password"`,
		},
		{
			name:           "Invalid password",
			method:         http.MethodPost,
			body:           `{"email": "test@example.com", "password": "wrongpass"}`,
			mockRepo:       setupMockUser("test@example.com", "strongpass"),
			rateLimitAllow: true,
			setEnv:         true,
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `"error":"Invalid email or password"`,
		},
		{
			name:           "Missing JWT_SECRET",
			method:         http.MethodPost,
			body:           `{"email": "test@example.com", "password": "strongpass"}`,
			mockRepo:       setupMockUser("test@example.com", "strongpass"),
			rateLimitAllow: true,
			setEnv:         false,
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   `"error":"Cannot create token"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv {
				os.Setenv("JWT_SECRET", "test-secret-32-bytes-long-1234567890")
			} else {
				os.Unsetenv("JWT_SECRET")
			}

			rl := NewRateLimiter(5, 1*time.Second)
			if !tt.rateLimitAllow {
				for i := 0; i < 5; i++ {
					rl.Allow("192.168.1.1")
				}
			}
			handler := &Handler{UserRepo: tt.mockRepo, RateLimiter: rl}

			req := httptest.NewRequest(tt.method, "/login", bytes.NewBufferString(tt.body))
			req.RemoteAddr = "192.168.1.1"
			rr := httptest.NewRecorder()

			handler.Login(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d, body: %s", tt.expectedStatus, rr.Code, rr.Body.String())
			}
			if body := strings.TrimSpace(rr.Body.String()); !strings.Contains(body, tt.expectedBody) {
				t.Errorf("Expected body to contain %q, got %q", tt.expectedBody, body)
			}
		})
	}
}

func setupMockUser(email, password string) *MockUserRepository {
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

func TestLoginConcurrent(t *testing.T) {
	repo := setupMockUser("test@example.com", "strongpass")
	rl := NewRateLimiter(3, 100*time.Millisecond)
	handler := &Handler{UserRepo: repo, RateLimiter: rl}
	os.Setenv("JWT_SECRET", "test-secret-32-bytes-long-1234567890")

	var wg sync.WaitGroup
	results := make([]int, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"email": "test@example.com", "password": "strongpass"}`))
			req.RemoteAddr = "192.168.1.1"
			rr := httptest.NewRecorder()
			handler.Login(rr, req)
			results[i] = rr.Code
		}(i)
	}
	wg.Wait()

	allowed := 0
	for _, code := range results {
		if code == http.StatusOK {
			allowed++
		}
	}
	if allowed > 3 {
		t.Errorf("Expected at most 3 successes, got %d", allowed)
	}
}
