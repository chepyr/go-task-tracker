package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	tdb "github.com/chepyr/go-task-tracker/tasks-service/db"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

func setupHTTP(t *testing.T) (*Handler, *http.ServeMux, *sql.DB, string) {
	t.Helper()

	secret := strings.Repeat("a", 32)
	_ = os.Setenv("JWT_SECRET", secret)

	// in-memory sqlite DB
	dbx, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	ddl := `
CREATE TABLE boards (
  id TEXT PRIMARY KEY,
  owner_id TEXT NOT NULL,
  title TEXT NOT NULL,
  description TEXT,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL
);
CREATE TABLE tasks (
  id TEXT PRIMARY KEY,
  board_id TEXT NOT NULL,
  title TEXT NOT NULL,
  description TEXT,
  status TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL
);
`
	if _, err := dbx.Exec(ddl); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	h := &Handler{
		BoardRepo:   tdb.NewBoardRepository(dbx),
		TaskRepo:    tdb.NewTaskRepository(dbx),
		RateLimiter: NewRateLimiter(5, time.Second),
		WSHub:       NewWSHub(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/boards", h.AuthMiddleware(h.HandleBoards))
	mux.HandleFunc("/boards/", h.AuthMiddleware(h.HandleBoardByID))
	mux.HandleFunc("/tasks", h.AuthMiddleware(h.HandleTasks))
	mux.HandleFunc("/tasks/", h.AuthMiddleware(h.HandleTaskByID))
	mux.HandleFunc("/ws", h.AuthMiddleware(h.HandleWebSocket))

	return h, mux, dbx, secret
}

func bearerForUser(t *testing.T, secret, userID string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign jwt: %v", err)
	}
	return "Bearer " + signed
}

func TestBoardsAndTasks_HappyPath(t *testing.T) {
	_, mux, dbx, secret := setupHTTP(t)
	defer dbx.Close()

	// user - UUID (middleware puts user_id in context,
	// 		and board is created with OwnerID=uuid.MustParse(userID))
	userID := uuid.New().String()
	authz := bearerForUser(t, secret, userID)

	// 1) make board: POST /boards
	body := `{"title":"My board","description":"for tasks"}`
	req := httptest.NewRequest(http.MethodPost, "/boards", bytes.NewBufferString(body))
	req.Header.Set("Authorization", authz)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /boards status=%d body=%s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if loc == "" || !strings.HasPrefix(loc, "/boards/") {
		t.Fatalf("no Location header with board id: %q", loc)
	}
	boardID := strings.TrimPrefix(loc, "/boards/")

	// 2) make task: POST /tasks
	taskReq := map[string]any{
		"board_id":    boardID,
		"title":       "Task #1",
		"description": "desc",
		"status":      "todo",
	}
	buf, _ := json.Marshal(taskReq)
	req2 := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewBuffer(buf))
	req2.Header.Set("Authorization", authz)
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("POST /tasks status=%d body=%s", rec2.Code, rec2.Body.String())
	}
	var created []*struct {
		ID     string `json:"id"`
		Title  string `json:"title"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created task: %v", err)
	}
	if len(created) != 1 || created[0].Title != "Task #1" || created[0].Status != "todo" {
		t.Fatalf("unexpected created task: %+v", created)
	}

	// 3) list tasks for board: GET /tasks?board_id=...
	req3 := httptest.NewRequest(http.MethodGet, "/tasks?board_id="+boardID, nil)
	req3.Header.Set("Authorization", authz)
	rec3 := httptest.NewRecorder()
	mux.ServeHTTP(rec3, req3)

	if rec3.Code != http.StatusOK {
		t.Fatalf("GET /tasks status=%d body=%s", rec3.Code, rec3.Body.String())
	}
	var listed []*struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(rec3.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listed) != 1 || listed[0].Title != "Task #1" {
		t.Fatalf("unexpected list: %+v", listed)
	}
}

// board belongs to userA
// userB tries to create task on that board -> 403 Forbidden
func TestTasks_Create_ForbiddenForForeignBoard(t *testing.T) {
	_, mux, dbx, secret := setupHTTP(t)
	defer dbx.Close()

	userA := uuid.New().String()
	userB := uuid.New().String()
	authA := bearerForUser(t, secret, userA)
	authB := bearerForUser(t, secret, userB)

	req := httptest.NewRequest(http.MethodPost, "/boards", bytes.NewBufferString(`{"title":"A"}`))
	req.Header.Set("Authorization", authA)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create board status=%d", rec.Code)
	}
	boardID := strings.TrimPrefix(rec.Header().Get("Location"), "/boards/")

	// userB tries to create task on userA's board
	task := `{"board_id":"` + boardID + `","title":"x"}`
	req2 := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewBufferString(task))
	req2.Header.Set("Authorization", authB)
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec2.Code, rec2.Body.String())
	}
}

// board belongs to userA
// userB tries to get/update/delete task on that board -> 403 Forbidden
func TestTask_ByID_ForbiddenForNonOwner(t *testing.T) {
	_, mux, dbx, secret := setupHTTP(t)
	defer dbx.Close()

	userA := uuid.New().String()
	userB := uuid.New().String()
	authA := bearerForUser(t, secret, userA)
	authB := bearerForUser(t, secret, userB)

	// userA creates a board
	reqBoard := httptest.NewRequest(http.MethodPost, "/boards", bytes.NewBufferString(`{"title":"A"}`))
	reqBoard.Header.Set("Authorization", authA)
	reqBoard.Header.Set("Content-Type", "application/json")
	recBoard := httptest.NewRecorder()
	mux.ServeHTTP(recBoard, reqBoard)
	if recBoard.Code != http.StatusCreated {
		t.Fatalf("create board status=%d", recBoard.Code)
	}
	boardID := strings.TrimPrefix(recBoard.Header().Get("Location"), "/boards/")

	// userA creates a task
	taskBody := `{"board_id":"` + boardID + `","title":"Task 1"}`
	reqTask := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewBufferString(taskBody))
	reqTask.Header.Set("Authorization", authA)
	reqTask.Header.Set("Content-Type", "application/json")
	recTask := httptest.NewRecorder()
	mux.ServeHTTP(recTask, reqTask)
	if recTask.Code != http.StatusOK {
		t.Fatalf("create task status=%d", recTask.Code)
	}
	var createdTasks []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(recTask.Body.Bytes(), &createdTasks); err != nil || len(createdTasks) == 0 {
		t.Fatalf("decode created task: %v", err)
	}
	taskID := createdTasks[0].ID

	// userB trieds to get userA's task
	reqGet := httptest.NewRequest(http.MethodGet, "/tasks/"+taskID, nil)
	reqGet.Header.Set("Authorization", authB)
	reqGet.Header.Set("Content-Type", "application/json")
	recGet := httptest.NewRecorder()
	mux.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", recGet.Code,
			recGet.Body.String())
	}

	// userB tries to update userA's task
	updateBody := `{"title":"updated title"}`
	reqUpdate := httptest.NewRequest(http.MethodPut, "/tasks/"+taskID, bytes.NewBufferString(updateBody))
	reqUpdate.Header.Set("Authorization", authB)
	reqUpdate.Header.Set("Content-Type", "application/json")
	recUpdate := httptest.NewRecorder()
	mux.ServeHTTP(recUpdate, reqUpdate)

	if recUpdate.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", recUpdate.Code, recUpdate.Body.String())
	}

	// userB tries to delete userA's task
	reqDelete := httptest.NewRequest(http.MethodDelete, "/tasks/"+taskID, nil)
	reqDelete.Header.Set("Authorization", authB)
	reqDelete.Header.Set("Content-Type", "application/json")
	recDelete := httptest.NewRecorder()
	mux.ServeHTTP(recDelete, reqDelete)
	if recDelete.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", recDelete.Code, recDelete.Body.String())
	}
}

// no Authorization header -> 401 Unauthorized
// for GET /tasks/{id}, POST /tasks, PUT /tasks/{id}, DELETE /tasks/{id}
func TestTask_ByID_Unauthorized(t *testing.T) {
	_, mux, dbx, _ := setupHTTP(t)
	defer dbx.Close()

	endpoints := []struct {
		method string
		url    string
		body   string
	}{
		{method: http.MethodGet, url: "/tasks/some-id"},
		{method: http.MethodPost, url: "/tasks", body: `{"board
_id":"some-board","title":"x"}`},
		{method: http.MethodPut, url: "/tasks/some-id", body: `{"title":"x"}`},
		{method: http.MethodDelete, url: "/tasks/some-id"},
	}

	for _, ep := range endpoints {
		req := httptest.NewRequest(ep.method, ep.url, bytes.NewBufferString(ep.body))
		// no Authorization header
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
		}
	}
}
