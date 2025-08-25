package handlers

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chepyr/go-task-tracker/auth-service/db"
)

// TestRegister tests the Register handler with various scenarios.
// It uses table-driven tests to cover different cases like successful registration,
func TestRegister(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		body           string
		mockRepo       db.UserRepositoryInterface
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "Successful registration",
			method:         http.MethodPost,
			body:           `{"email": "test@example.com", "password": "strongpass"}`,
			mockRepo:       NewMockUserRepository(),
			expectedStatus: http.StatusCreated,
			expectedBody:   `"email":"test@example.com"`, // check email in response
		},
		{
			name:           "Invalid method (GET instead of POST)",
			method:         http.MethodGet,
			body:           ``,
			mockRepo:       NewMockUserRepository(),
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   `"error":"Use POST method"`,
		},
		{
			name:           "Invalid JSON",
			method:         http.MethodPost,
			body:           `{"email": "test@example.com", "password": }`, // Broken JSON
			mockRepo:       NewMockUserRepository(),
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `"error":"Bad JSON"`,
		},
		{
			name:           "Invalid email format",
			method:         http.MethodPost,
			body:           `{"email": "invalid", "password": "strongpass"}`,
			mockRepo:       NewMockUserRepository(),
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `"error":"Invalid email"`,
		},
		{
			name:           "Password too short",
			method:         http.MethodPost,
			body:           `{"email": "test@example.com", "password": "abc"}`,
			mockRepo:       NewMockUserRepository(),
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `"error":"Password must be at least 4 characters long"`,
		},
		{
			name:   "Email already exists (repo error)",
			method: http.MethodPost,
			body:   `{"email": "test@example.com", "password": "strongpass"}`,
			mockRepo: &MockUserRepository{
				createErr: errors.New("unique violation: email already exists"),
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   `"error":"Cannot save user"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/register", bytes.NewBufferString(tt.body))
			rr := httptest.NewRecorder()

			handler := &Handler{
				UserRepo: tt.mockRepo,
			}

			handler.Register(rr, req)

			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, status)
			}

			body := strings.TrimSpace(rr.Body.String())
			if !strings.Contains(body, tt.expectedBody) {
				t.Errorf("Expected body to contain %q, got %q", tt.expectedBody, body)
			}

			if tt.expectedStatus != http.StatusMethodNotAllowed {
				if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
					t.Errorf("Expected Content-Type application/json, got %s", ct)
				}
			}
		})
	}
}

func TestValidateUserEmailAndPassword(t *testing.T) {
	tests := []struct {
		name  string
		input struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		expected bool
	}{
		{
			name: "Valid email and password",
			input: struct {
				Email    string `json:"email"`
				Password string `json:"password"`
			}{
				Email:    "test@example.com",
				Password: "strongpass",
			},
			expected: true,
		},
		{
			name: "Invalid email (no @)",
			input: struct {
				Email    string `json:"email"`
				Password string `json:"password"`
			}{
				Email:    "invalid.com",
				Password: "strongpass",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			got := validateUserEmailAndPassword(tt.input, rr)
			if got != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, got)
			}
			if !tt.expected && rr.Code == 0 {
				t.Error("Expected http.Error to be called, but no response written")
			}
		})
	}
}

func TestIsValidEmail(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		expected bool
	}{
		{"Valid simple email", "user@example.com", true},
		{"Valid with subdomain", "user@sub.example.com", true},
		{"Valid with +", "user+tag@example.com", true},
		{"Valid with numbers", "user123@example.com", true},
		{"Invalid no @", "userexample.com", false},
		{"Invalid no domain", "user@", false},
		{"Invalid no TLD", "user@example", false},
		{"Invalid special chars", "user@exa!mple.com", false},
		{"Empty string", "", false},
		{"Only domain", "@example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidEmail(tt.email)
			if got != tt.expected {
				t.Errorf("For email %q, expected %v, got %v", tt.email, tt.expected, got)
			}
		})
	}
}

func BenchmarkRegister(b *testing.B) {
	mockRepo := NewMockUserRepository()
	handler := &Handler{UserRepo: mockRepo}
	body := `{"email": "test@example.com", "password": "strongpass"}`

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString(body))
		rr := httptest.NewRecorder()
		handler.Register(rr, req)
		if rr.Code != http.StatusCreated {
			b.Fatalf("Unexpected status: %d", rr.Code)
		}
	}
}

func TestRegisterConcurrent(t *testing.T) {
	mockRepo := NewMockUserRepository()
	handler := &Handler{UserRepo: mockRepo}

	numGoroutines := 100
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			email := fmt.Sprintf("concurrent%d@example.com", idx)
			body := fmt.Sprintf(`{"email": "%s", "password": "strongpass"}`, email)
			req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString(body))
			rr := httptest.NewRecorder()
			handler.Register(rr, req)

			if rr.Code != http.StatusCreated {
				t.Errorf("Concurrent registration failed: status %d", rr.Code)
			}
			done <- true
		}(i)
	}

	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	if len(mockRepo.users) != numGoroutines {
		t.Errorf("Expected %d users, got %d", numGoroutines, len(mockRepo.users))
	}
}
