module github.com/chepyr/go-task-tracker/tasks-service

go 1.25.0

require (
	github.com/chepyr/go-task-tracker/shared v0.1.0
	github.com/joho/godotenv v1.5.1
	github.com/lib/pq v1.10.9
)

require (
	github.com/golang-jwt/jwt/v5 v5.3.0
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.5.3
)

replace github.com/chepyr/go-task-tracker/shared v0.1.0 => ../shared
