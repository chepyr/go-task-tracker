package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/chepyr/go-task-tracker/tasks-service/db"
    "github.com/chepyr/go-task-tracker/tasks-service/handlers"
    "github.com/joho/godotenv"
    "github.com/lib/pq"
    "github.com/gin-gonic/gin"
)

func main() {
    if err := godotenv.Load(); err != nil {
        log.Fatalf("Error loading .env: %v", err)
    }
    validateEnv()
    dbConn := initDB()
    defer func() { _ = dbConn.Close() }()

    r := gin.Default()
    handlers.InitRoutes(r, dbConn)

    server := &http.Server{
        Addr:    ":" + os.Getenv("SERVER_PORT_TASKS"),
        Handler: r,
    }
    startServer(server)
}

func validateEnv() {
    required := []string{"POSTGRES_HOST", "POSTGRES_USER", "POSTGRES_PASSWORD", "POSTGRES_DB", "POSTGRES_PORT", "SERVER_PORT_TASKS", "JWT_SECRET"}
    for _, env := range required {
        if os.Getenv(env) == "" {
            log.Fatalf("Missing env variable: %s", env)
        }
    }
}

func initDB() *sql.DB {
    connStr := "host=" + os.Getenv("POSTGRES_HOST") +
        " port=" + os.Getenv("POSTGRES_PORT") +
        " user=" + os.Getenv("POSTGRES_USER") +
        " password=" + os.Getenv("POSTGRES_PASSWORD") +
        " dbname=" + os.Getenv("POSTGRES_DB") +
        " sslmode=disable"
    db, err := sql.Open("postgres", connStr)
    if err != nil {
        log.Fatalf("Failed to connect to DB: %v", err)
    }
    if err := db.Ping(); err != nil {
        log.Fatalf("Failed to ping DB: %v", err)
    }
    db.SetMaxOpenConns(10)
    return db
}

func startServer(server *http.Server) {
    go func() {
        if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("Server failed: %v", err)
        }
    }()
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := server.Shutdown(ctx); err != nil {
        log.Fatalf("Server shutdown failed: %v", err)
    }
}