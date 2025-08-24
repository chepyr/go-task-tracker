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
- **Testing**: Unit and integration tests with `go test -race`, ~75% coverage --- TODO


## Usage

Start the services using Docker Compose:
```shell
docker compose up --build -d
```

Run database migrations using goose:
```shell
# auth-service
goose -dir auth-service/migrations postgres \
"host=localhost user=postgres password=pass dbname=auth_db port=5432 sslmode=disable" up

# tasks-service
goose -dir migrations postgres \
"host=localhost user=postgres password=pass dbname=tasks_db port=5433 sslmode=disable" up
```

Test the services using curl:
```shell
# User registration
curl -X POST http://localhost:8081/register \
  -H "Content-Type: application/json" \
  -d '{"email":"anya@example.com","password":"secret"}'

# Login and save token
TOKEN=$(curl -s -X POST http://localhost:8081/login \
  -H "Content-Type: application/json" \
  -d '{"email":"anya@example.com","password":"secret"}' | jq -r '.token')

# Create a board
curl -v -X POST http://localhost:8082/boards \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"title":"My Board","description":"For tasks"}'
  ```