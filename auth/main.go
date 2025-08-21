package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/chepyr/go-task-tracker/internal/db"
	"github.com/chepyr/go-task-tracker/internal/handlers"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		panic("Error loading .env file: " + err.Error())
	}

	user := os.Getenv("POSTGRES_USER")
	password := os.Getenv("POSTGRES_PASSWORD")
	dbname := os.Getenv("POSTGRES_DB")
	port := os.Getenv("POSTGRES_PORT")

	requiredEnvVars := []string{
		"POSTGRES_USER", "POSTGRES_PASSWORD", "POSTGRES_DB",
		"POSTGRES_HOST", "POSTGRES_PORT",
	}
	for _, env := range requiredEnvVars {
		if os.Getenv(env) == "" {
			log.Fatalf("Environment variable %s must be set", env)
		}
	}

	dsn := fmt.Sprintf("host=localhost user=%s password=%s dbname=%s port=%s sslmode=disable", user, password, dbname, port)

	dbConn, err := db.Connect(dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbConn.Close()

	handler := &handlers.Handler{UserRepo: db.NewUserRepository(dbConn)}
	http.HandleFunc("/register", handler.Register)

	serverPort := os.Getenv("SERVER_PORT")
	if serverPort == "" {
		log.Fatal("SERVER_PORT environment variable must be set")
	}
	log.Printf("Starting server on :%s", serverPort)
	if err := http.ListenAndServe(":"+serverPort, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
