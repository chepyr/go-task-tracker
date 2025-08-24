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

	"github.com/chepyr/go-task-tracker/auth-service/db"
	"github.com/chepyr/go-task-tracker/auth-service/handlers"
	_ "github.com/lib/pq"
)

func main() {
	// if err := godotenv.Load(); err != nil {
	// 	log.Fatalf("Error loading .env file: %v", err)
	// }
	// if _, err := os.Stat(".env"); err == nil {
	// 	// файл есть → загружаем
	// 	if err := godotenv.Load(); err != nil {
	// 		log.Fatalf("Error loading .env file: %v", err)
	// 	}
	// } else {
	// 	log.Println(".env file not found, skipping — relying on environment variables")
	// }

	validateEnv()
	dbConn := initDB()

	defer func() {
		if err := dbConn.Close(); err != nil {
			log.Printf("Error closing database connection: %v", err)
		}
	}()

	initHandlers(dbConn)

	server := initServer()
	startServer(server)
}

func validateEnv() {
	requiredEnvVars := []string{
		"POSTGRES_USER", "POSTGRES_PASSWORD", "POSTGRES_DB",
		"POSTGRES_HOST", "POSTGRES_PORT", "SERVER_PORT",
	}
	for _, env := range requiredEnvVars {
		if os.Getenv(env) == "" {
			log.Fatalf("Environment variable %s must be set", env)
		}
	}
	if len(os.Getenv("JWT_SECRET")) < 32 {
		log.Fatal("JWT_SECRET must be at least 32 characters")
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

func initHandlers(dbConn *sql.DB) {
	handler := &handlers.Handler{
		UserRepo: db.NewUserRepository(dbConn),
		// allow max 5 login attempts per 15 minutes from the same IP
		RateLimiter: handlers.NewRateLimiter(5, 15*time.Minute),
	}
	http.HandleFunc("/register", handler.Register)
	http.HandleFunc("/login", handler.Login)
}

func initServer() *http.Server {
	return &http.Server{
		Addr: ":" + os.Getenv("SERVER_PORT"),
	}
}

func startServer(server *http.Server) {
	log.Printf("Starting server on :%s", os.Getenv("SERVER_PORT"))

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Graceful shutdown
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
