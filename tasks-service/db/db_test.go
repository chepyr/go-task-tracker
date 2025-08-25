package db

import (
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestConnect(t *testing.T) {
	tests := []struct {
		name          string
		driverName    string
		dsn           string
		expectedError bool
	}{
		{
			name:          "Successful connection with SQLite",
			driverName:    "sqlite3",
			dsn:           ":memory:",
			expectedError: false,
		},
		{
			name:          "Failed connection with invalid DSN",
			driverName:    "sqlite3",
			dsn:           "file::memory:?mode=invalid",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn, err := Connect(tt.driverName, tt.dsn)

			if tt.expectedError {
				if err == nil {
					t.Error("Expected error, got none")
				}
				if conn != nil {
					t.Error("Expected nil connection on error")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if conn == nil {
					t.Error("Expected non-nil connection")
				} else {
					if conn.Stats().MaxOpenConnections != 10 {
						t.Errorf("Expected MaxOpenConnections to be 10, got %d", conn.Stats().MaxOpenConnections)
					}

					// Simulate usage to check idle connections
					for i := 0; i < 10; i++ {
						_, err := conn.Query("SELECT 1")
						if err != nil {
							t.Errorf("Query failed: %v", err)
						}
					}
					time.Sleep(100 * time.Millisecond)
					if conn.Stats().Idle > 5 {
						t.Errorf("Expected at most 5 idle connections, got %d", conn.Stats().Idle)
					}
				}
			}

			if conn != nil {
				conn.Close()
			}
		})
	}
}
