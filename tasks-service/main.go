package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chepyr/go-task-tracker/tasks-service/db"
	"github.com/chepyr/go-task-tracker/tasks-service/handlers"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	validateEnv()
	dbConn := initDB()
	defer dbConn.Close()

	initHandlers(dbConn)
	server := initServer()
	startServer(server)
}

func validateEnv() {
	requiredEnvVars := []string{
		"POSTGRES_USER", "POSTGRES_PASSWORD", "POSTGRES_DB",
		"POSTGRES_HOST", "POSTGRES_PORT", "SERVER_PORT_TASKS",
		"JWT_SECRET", "AUTH_SERVICE_URL",
	}
	for _, env := range requiredEnvVars {
		if os.Getenv(env) == "" {
			log.Fatalf("Environment variable %s must be set", env)
		}
	}
}

func initDB() *sql.DB {
	user := os.Getenv("POSTGRES_USER")
	password := os.Getenv("POSTGRES_PASSWORD")
	dbname := os.Getenv("POSTGRES_DB")
	port := os.Getenv("POSTGRES_PORT")
	host := os.Getenv("POSTGRES_HOST")

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		host, user, password, dbname, port)

	dbConn, err := db.Connect("postgres", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	return dbConn
}

func initHandlers(dbConn *sql.DB) *handlers.Handler {
	handler := &handlers.Handler{
		BoardRepo:   db.NewBoardRepository(dbConn),
		TaskRepo:    db.NewTaskRepository(dbConn),
		RateLimiter: handlers.NewRateLimiter(5, time.Second),
		WSHub:       handlers.NewWSHub(),
	}
	http.HandleFunc("/boards", handler.AuthMiddleware(handler.HandleBoards))
	http.HandleFunc("/boards/", handler.AuthMiddleware(handler.HandleBoardByID))
	http.HandleFunc("/boards/tasks", handler.AuthMiddleware(handler.HandleTasks))
	http.HandleFunc("/ws", handler.AuthMiddleware(handler.HandleWebSocket))
	return handler
}

func initServer() *http.Server {
	return &http.Server{
		Addr: ":" + os.Getenv("SERVER_PORT_TASKS"),
	}
}

func startServer(server *http.Server) {
	log.Printf("Starting tasks server on :%s", os.Getenv("SERVER_PORT_TASKS"))

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}
	log.Println("Server stopped")
}
