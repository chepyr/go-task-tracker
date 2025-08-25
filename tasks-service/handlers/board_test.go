package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chepyr/go-task-tracker/shared/models"
	tdb "github.com/chepyr/go-task-tracker/tasks-service/db"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

func setupBoardsDB(t *testing.T) *sql.DB {
	t.Helper()
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
);`
	if _, err := dbx.Exec(ddl); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return dbx
}

func handlerWithBoardsRepo(t *testing.T) (*Handler, *sql.DB) {
	t.Helper()
	dbx := setupBoardsDB(t)
	return &Handler{
		BoardRepo: tdb.NewBoardRepository(dbx),
		WSHub:     NewWSHub(),
		// TaskRepo/RateLimiter not needed for board tests
	}, dbx
}

func ctxWithUser(id string, r *http.Request) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), "user_id", id))
}

// checks that unsupported methods return 405
func TestHandleBoards_MethodNotAllowed(t *testing.T) {
	h, dbx := handlerWithBoardsRepo(t)
	defer dbx.Close()

	req := httptest.NewRequest(http.MethodDelete, "/boards", nil)
	rec := httptest.NewRecorder()

	h.HandleBoards(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("want 405, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// checks that unauthorized requests return 401
func TestListBoards_Unauthorized_NoUserInContext(t *testing.T) {
	h, dbx := handlerWithBoardsRepo(t)
	defer dbx.Close()

	req := httptest.NewRequest(http.MethodGet, "/boards", nil)
	rec := httptest.NewRecorder()

	h.HandleBoards(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// checks that creating board validates Content-Type and JSON body
func TestCreateBoard_ContentTypeAndJSONValidation(t *testing.T) {
	h, dbx := handlerWithBoardsRepo(t)
	defer dbx.Close()

	userID := uuid.New().String()

	// 1) Content-Type is missing
	req1 := httptest.NewRequest(http.MethodPost, "/boards", bytes.NewBufferString(`{"title":"x"}`))
	req1 = ctxWithUser(userID, req1)
	rec1 := httptest.NewRecorder()
	h.HandleBoards(rec1, req1)
	if rec1.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for no content-type, got %d", rec1.Code)
	}

	// 2) invalid JSON
	req2 := httptest.NewRequest(http.MethodPost, "/boards", bytes.NewBufferString(`{bad json`))
	req2.Header.Set("Content-Type", "application/json")
	req2 = ctxWithUser(userID, req2)
	rec2 := httptest.NewRecorder()
	h.HandleBoards(rec2, req2)
	if rec2.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for invalid json, got %d", rec2.Code)
	}

	// 3) empty title
	req3 := httptest.NewRequest(http.MethodPost, "/boards", bytes.NewBufferString(`{"title":"   "}`))
	req3.Header.Set("Content-Type", "application/json")
	req3 = ctxWithUser(userID, req3)
	rec3 := httptest.NewRecorder()
	h.HandleBoards(rec3, req3)
	if rec3.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for empty title, got %d", rec3.Code)
	}

	// 4) title too long
	longTitle := strings.Repeat("a", 101)
	req4 := httptest.NewRequest(http.MethodPost, "/boards", bytes.NewBufferString(`{"title":"`+longTitle+`"}`))
	req4.Header.Set("Content-Type", "application/json")
	req4 = ctxWithUser(userID, req4)
	rec4 := httptest.NewRecorder()
	h.HandleBoards(rec4, req4)
	if rec4.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for long title, got %d", rec4.Code)
	}

	// 5) description too long
	longDesc := strings.Repeat("b", 501)
	req5 := httptest.NewRequest(http.MethodPost, "/boards", bytes.NewBufferString(`{"title":"ok","description":"`+longDesc+`"}`))
	req5.Header.Set("Content-Type", "application/json")
	req5 = ctxWithUser(userID, req5)
	rec5 := httptest.NewRecorder()
	h.HandleBoards(rec5, req5)
	if rec5.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for long description, got %d", rec5.Code)
	}
}

// successful creation returns 201 and Location header
func TestCreateBoard_Success(t *testing.T) {
	h, dbx := handlerWithBoardsRepo(t)
	defer dbx.Close()

	userID := uuid.New().String()

	req := httptest.NewRequest(http.MethodPost, "/boards", bytes.NewBufferString(`{"title":"My Board","description":"desc"}`))
	req.Header.Set("Content-Type", "application/json")
	req = ctxWithUser(userID, req)

	rec := httptest.NewRecorder()
	h.HandleBoards(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if loc == "" || !strings.HasPrefix(loc, "/boards/") {
		t.Fatalf("missing Location header, got %q", loc)
	}
}

// checks that returns 400 if board ID is invalid
func TestHandleBoardByID_InvalidID(t *testing.T) {
	h, dbx := handlerWithBoardsRepo(t)
	defer dbx.Close()

	req := httptest.NewRequest(http.MethodGet, "/boards/not-a-uuid", nil)
	req = ctxWithUser(uuid.New().String(), req)
	rec := httptest.NewRecorder()

	h.HandleBoardByID(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 invalid id, got %d", rec.Code)
	}
}

func createBoard(t *testing.T, h *Handler, userID uuid.UUID, title string) string {
	t.Helper()
	board := &models.Board{
		ID:          uuid.New(),
		OwnerID:     userID,
		Title:       title,
		Description: "d",
	}

	err := h.BoardRepo.Create(context.Background(), board)
	if err != nil {
		t.Fatalf("failed to create board for test: %v", err)
	}

	return board.ID.String()
}

// checks that returns 404 if board not found, and 403 if user is not owner
func TestGetBoard_NotFound_And_Forbidden(t *testing.T) {
	h, dbx := handlerWithBoardsRepo(t)
	defer dbx.Close()

	owner := uuid.New()
	otherUser := uuid.New()

	boardID := uuid.New().String() // invalid
	reqNF := httptest.NewRequest(http.MethodGet, "/boards/"+boardID, nil)
	reqNF = ctxWithUser(owner.String(), reqNF)
	recNF := httptest.NewRecorder()
	h.HandleBoardByID(recNF, reqNF)
	if recNF.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", recNF.Code)
	}

	// create board for owner
	boardID = createBoard(t, h, owner, "A")
	reqForbidden := httptest.NewRequest(http.MethodGet, "/boards/"+boardID, nil)
	reqForbidden = ctxWithUser(otherUser.String(), reqForbidden)
	recForbidden := httptest.NewRecorder()
	h.HandleBoardByID(recForbidden, reqForbidden)
	if recForbidden.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", recForbidden.Code)
	}
}

// successful deletion returns 204 and actually deletes the board
func TestDeleteBoard_Success(t *testing.T) {
	h, dbx := handlerWithBoardsRepo(t)
	defer dbx.Close()

	owner := uuid.New()
	boardID := createBoard(t, h, owner, "A")

	req := httptest.NewRequest(http.MethodDelete, "/boards/"+boardID, nil)
	req = ctxWithUser(owner.String(), req)
	rec := httptest.NewRecorder()

	h.HandleBoardByID(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d body=%s", rec.Code, rec.Body.String())
	}

	// checks that board is actually deleted
	if _, err := h.BoardRepo.GetByID(context.Background(), boardID); err == nil {
		t.Fatalf("expected GetByID error after delete")
	}
}

// checks that updating board validates Content-Type, JSON body, ownership, and returns 200 on success
func TestUpdateBoard_ValidationAndSuccess(t *testing.T) {
	h, dbx := handlerWithBoardsRepo(t)
	defer dbx.Close()

	owner := uuid.New()
	other := uuid.New()
	boardID := createBoard(t, h, owner, "Old")

	// 1) unsupported media type
	req1 := httptest.NewRequest(http.MethodPut, "/boards/"+boardID, bytes.NewBufferString(`{"title":"New"}`))
	req1 = ctxWithUser(owner.String(), req1)
	rec1 := httptest.NewRecorder()
	h.HandleBoardByID(rec1, req1)
	if rec1.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("want 415 for content-type missing, got %d", rec1.Code)
	}

	// 2) forbidden
	req2 := httptest.NewRequest(http.MethodPut, "/boards/"+boardID, bytes.NewBufferString(`{"title":"New"}`))
	req2.Header.Set("Content-Type", "application/json")
	req2 = ctxWithUser(other.String(), req2)
	rec2 := httptest.NewRecorder()
	h.HandleBoardByID(rec2, req2)
	if rec2.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", rec2.Code)
	}

	// 3) invalid JSON
	req3 := httptest.NewRequest(http.MethodPut, "/boards/"+boardID, bytes.NewBufferString(`{bad`))
	req3.Header.Set("Content-Type", "application/json")
	req3 = ctxWithUser(owner.String(), req3)
	rec3 := httptest.NewRecorder()
	h.HandleBoardByID(rec3, req3)
	if rec3.Code != http.StatusBadRequest {
		t.Fatalf("want 400 invalid json, got %d", rec3.Code)
	}

	// 4) empty title
	req4 := httptest.NewRequest(http.MethodPut, "/boards/"+boardID, bytes.NewBufferString(`{"title":"  "}`))
	req4.Header.Set("Content-Type", "application/json")
	req4 = ctxWithUser(owner.String(), req4)
	rec4 := httptest.NewRecorder()
	h.HandleBoardByID(rec4, req4)
	if rec4.Code != http.StatusBadRequest {
		t.Fatalf("want 400 empty title, got %d", rec4.Code)
	}

	// 5) too long description
	req5 := httptest.NewRequest(http.MethodPut, "/boards/"+boardID, bytes.NewBufferString(`{"description":"`+strings.Repeat("x", 501)+`"}`))
	req5.Header.Set("Content-Type", "application/json")
	req5 = ctxWithUser(owner.String(), req5)
	rec5 := httptest.NewRecorder()
	h.HandleBoardByID(rec5, req5)
	if rec5.Code != http.StatusBadRequest {
		t.Fatalf("want 400 long description, got %d", rec5.Code)
	}

	// 6) success (partial update title)
	req6 := httptest.NewRequest(http.MethodPut, "/boards/"+boardID, bytes.NewBufferString(`{"title":"New Title"}`))
	req6.Header.Set("Content-Type", "application/json")
	req6 = ctxWithUser(owner.String(), req6)
	rec6 := httptest.NewRecorder()
	h.HandleBoardByID(rec6, req6)
	if rec6.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec6.Code, rec6.Body.String())
	}
	var resp []*struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(rec6.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp) != 1 || resp[0].Title != "New Title" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

// checks
func TestBoardTest_listBoards(t *testing.T) {
	h, dbx := handlerWithBoardsRepo(t)
	defer dbx.Close()

	ownerID := uuid.New()
	otherID := uuid.New()

	// create 2 boards for owner
	createBoard(t, h, ownerID, "A")
	createBoard(t, h, ownerID, "B")

	// checks for owner
	reqOwner := httptest.NewRequest(http.MethodGet, "/boards", nil)
	reqOwner = ctxWithUser(ownerID.String(), reqOwner)
	recOwner := httptest.NewRecorder()
	h.HandleBoards(recOwner, reqOwner)
	if recOwner.Code != http.StatusOK {
		t.Fatalf("want 200 for owner, got %d body=%s", recOwner.Code, recOwner.Body.String())
	}
	var boardsOwner []*struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(recOwner.Body.Bytes(), &boardsOwner); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(boardsOwner) != 2 {
		t.Fatalf("want 2 boards for owner, got %d", len(boardsOwner))
	}

	// checks for other user
	reqOther := httptest.NewRequest(http.MethodGet, "/boards", nil)
	reqOther = ctxWithUser(otherID.String(), reqOther)
	recOther := httptest.NewRecorder()
	h.HandleBoards(recOther, reqOther)
	if recOther.Code != http.StatusOK {
		t.Fatalf("want 200 for other, got %d body=%s", recOther.Code, recOther.Body.String())
	}
	var boardsOther []*struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(recOther.Body.Bytes(), &boardsOther); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(boardsOther) != 0 {
		t.Fatalf("want 0 boards for other, got %d", len(boardsOther))
	}
}
