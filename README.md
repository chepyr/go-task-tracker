# Task Tracker

### About
A microservices-based task management system built with Go, using database/sql for low-level database access and goose for migrations.

### Features

- User registration with bcrypt password hashing (REST API: POST /register).
- (In progress) User login with JWT, task/board CRUD, WebSockets.

### Tech Stack


- `Go` 1.25+
- `PostgreSQL` 15
- `database/sql` (optimized for production)
- `goose` (migrations for versioned schema changes)
- `net/http` (REST API, fast)
- `bcrypt` (password hashing) - ???
