package handlers

import (
	"sync"
	"testing"
	"time"
)

// TestNewRateLimiter verifies the initialization of RateLimiter.
func TestNewRateLimiter(t *testing.T) {
	limit := 5
	window := 1 * time.Second
	rl := NewRateLimiter(limit, window)

	if rl.limit != limit {
		t.Errorf("Expected limit %d, got %d", limit, rl.limit)
	}
	if rl.window != window {
		t.Errorf("Expected window %v, got %v", window, rl.window)
	}
	if rl.attempts == nil {
		t.Error("Expected attempts map to be initialized, got nil")
	}
}

// TestRateLimiter_Allow tests the Allow method for rate limiting logic.
func TestRateLimiter_Allow(t *testing.T) {
	tests := []struct {
		name     string
		limit    int
		attempts []string // IPs to attempt
		expected []bool   // Expected results
	}{
		{
			name:     "Within limit",
			limit:    2,
			attempts: []string{"192.168.1.1", "192.168.1.1"},
			expected: []bool{true, true},
		},
		{
			name:     "Exceed limit",
			limit:    1,
			attempts: []string{"192.168.1.1", "192.168.1.1"},
			expected: []bool{true, false},
		},
		{
			name:     "Multiple IPs",
			limit:    1,
			attempts: []string{"192.168.1.1", "192.168.1.2"},
			expected: []bool{true, true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := NewRateLimiter(tt.limit, 1*time.Second)
			for i, ip := range tt.attempts {
				got := rl.Allow(ip)
				if got != tt.expected[i] {
					t.Errorf("Attempt %d for IP %s: expected %v, got %v", i+1, ip, tt.expected[i], got)
				}
			}
		})
	}
}

// TestRateLimiter_Cleanup tests the cleanup method.
func TestRateLimiter_Cleanup(t *testing.T) {
	rl := NewRateLimiter(5, 100*time.Millisecond)

	// Add attempts
	rl.Allow("192.168.1.1")
	rl.Allow("192.168.1.2")

	rl.mutex.Lock()
	if len(rl.attempts) != 2 {
		t.Errorf("Expected 2 IPs in attempts, got %d", len(rl.attempts))
	}
	rl.mutex.Unlock()

	// Wait for cleanup
	time.Sleep(150 * time.Millisecond)

	rl.mutex.Lock()
	defer rl.mutex.Unlock()
	if len(rl.attempts) != 0 {
		t.Errorf("Expected attempts map to be empty after cleanup, got %d", len(rl.attempts))
	}
}

// TestRateLimiter_Concurrent tests concurrent access to Allow.
func TestRateLimiter_Concurrent(t *testing.T) {
	rl := NewRateLimiter(3, 1*time.Second)
	ip := "192.168.1.1"
	var wg sync.WaitGroup
	results := make([]bool, 5)

	for i := range 5 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = rl.Allow(ip)
		}(i)
	}
	wg.Wait()

	allowedCount := 0
	for _, result := range results {
		if result {
			allowedCount++
		}
	}
	if allowedCount > rl.limit {
		t.Errorf("Expected at most %d allowed attempts, got %d", rl.limit, allowedCount)
	}
}
