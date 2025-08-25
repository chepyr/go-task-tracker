module github.com/chepyr/go-task-tracker/auth-service

go 1.25.0

require (
	github.com/google/uuid v1.6.0
	github.com/lib/pq v1.10.9
	golang.org/x/crypto v0.41.0
)

require (
	github.com/chepyr/go-task-tracker/shared v0.1.0
	github.com/golang-jwt/jwt/v5 v5.3.0
	github.com/mattn/go-sqlite3 v1.14.32
)

replace github.com/chepyr/go-task-tracker/shared v0.1.0 => ../shared
