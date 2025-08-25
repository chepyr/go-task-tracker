package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// checks that returns 401 if Authorization header is missing
func TestAuthMiddleware_MissingAuthorizationHeader(t *testing.T) {
	h := &Handler{}
	nextCalled := false
	next := func(w http.ResponseWriter, r *http.Request) { nextCalled = true }

	req := httptest.NewRequest(http.MethodGet, "/any", nil)
	rec := httptest.NewRecorder()

	h.AuthMiddleware(next)(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", rec.Code, rec.Body.String())
	}
	if nextCalled {
		t.Fatalf("next should NOT be called")
	}
}

// checks that returns 401 if token is invalid
func TestAuthMiddleware_InvalidToken(t *testing.T) {
	_ = os.Setenv("JWT_SECRET", "super_secret_for_tests")
	h := &Handler{}
	next := func(w http.ResponseWriter, r *http.Request) { t.Fatalf("next must not be called on invalid token") }

	req := httptest.NewRequest(http.MethodGet, "/any", nil)
	req.Header.Set("Authorization", "Bearer obviously.invalid.token")
	rec := httptest.NewRecorder()

	h.AuthMiddleware(next)(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// checks that returns 401 if "exp" claim is missing
func TestAuthMiddleware_MissingExp(t *testing.T) {
	secret := "super_secret_for_tests"
	_ = os.Setenv("JWT_SECRET", secret)

	claims := jwt.MapClaims{
		"sub": "11111111-1111-1111-1111-111111111111",
		// "exp" is missing
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	h := &Handler{}
	next := func(w http.ResponseWriter, r *http.Request) { t.Fatalf("next must not be called when exp missing") }

	req := httptest.NewRequest(http.MethodGet, "/any", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	rec := httptest.NewRecorder()

	h.AuthMiddleware(next)(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 (missing exp), got %d body=%s", rec.Code, rec.Body.String())
	}
}

// checks that returns 401 if "sub" claim is missing
func TestAuthMiddleware_MissingSub(t *testing.T) {
	secret := "super_secret_for_tests"
	_ = os.Setenv("JWT_SECRET", secret)

	claims := jwt.MapClaims{
		// "sub" is missing
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	h := &Handler{}
	next := func(w http.ResponseWriter, r *http.Request) { t.Fatalf("next must not be called when sub missing") }

	req := httptest.NewRequest(http.MethodGet, "/any", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	rec := httptest.NewRecorder()

	h.AuthMiddleware(next)(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 (invalid claims), got %d body=%s", rec.Code, rec.Body.String())
	}
}

// checks that returns 201 if token is valid, and user_id is put into context
func TestAuthMiddleware_Valid_PassesUserIDInContext(t *testing.T) {
	secret := "super_secret_for_tests"
	_ = os.Setenv("JWT_SECRET", secret)

	wantSub := "22222222-2222-2222-2222-222222222222"
	claims := jwt.MapClaims{
		"sub": wantSub,
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	h := &Handler{}
	nextCalled := false
	next := func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		got, _ := r.Context().Value("user_id").(string)
		if got != wantSub {
			t.Fatalf("user_id in ctx = %q, want %q", got, wantSub)
		}
		w.WriteHeader(http.StatusOK)
	}

	req := httptest.NewRequest(http.MethodGet, "/any", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	rec := httptest.NewRecorder()

	h.AuthMiddleware(next)(rec, req)

	if !nextCalled {
		t.Fatalf("next should be called")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
}
