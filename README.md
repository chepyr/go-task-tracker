# Task Tracker

### About
A microservices-based task management system built with Go, using database/sql for low-level database access and goose for migrations. 

### Features

- User registration with bcrypt password hashing (REST API: POST /register).
- (In progress) User login with JWT, task/board CRUD, WebSockets.
- Rate limiting to prevent brute-force attacks.
- Docker support for easy setup.

## Architecture
- **Multi-module**: `auth-service`, `tasks-service`, and `shared` in a single repository (`github.com/chepyr/go-task-tracker`).
- **Services**:
  - `auth-service`: User registration/login with JWT, rate limiting, bcrypt.
  - `tasks-service`: (WIP) Boards/tasks CRUD, WebSocket updates.
- **Database**: PostgreSQL with goose migrations.
- **Concurrency**: Thread-safe operations using mutexes.
- **Testing**: Unit and integration tests with `go test -race`, ~75% coverage.

