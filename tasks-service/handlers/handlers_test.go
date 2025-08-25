package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestClientIP_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
	req.RemoteAddr = "10.0.0.1:1234"

	if got := clientIP(req); got != "1.2.3.4" {
		t.Fatalf("clientIP = %q, want %q", got, "1.2.3.4")
	}
}

func TestClientIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:5555"
	if got := clientIP(req); got != "127.0.0.1" {
		t.Fatalf("clientIP = %q, want %q", got, "127.0.0.1")
	}
}

func TestCheckOrigin_EmptyAllowsAll(t *testing.T) {
	_ = os.Setenv("ALLOWED_ORIGINS", "")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://any.example")
	if !checkOrigin(req) {
		t.Fatalf("checkOrigin should allow when ALLOWED_ORIGINS is empty")
	}
}

func TestCheckOrigin_ListAllowAndDeny(t *testing.T) {
	_ = os.Setenv("ALLOWED_ORIGINS", "https://a.example, https://b.example")
	allowReq := httptest.NewRequest(http.MethodGet, "/", nil)
	allowReq.Header.Set("Origin", "https://b.example")
	denyReq := httptest.NewRequest(http.MethodGet, "/", nil)
	denyReq.Header.Set("Origin", "https://c.example")

	if !checkOrigin(allowReq) {
		t.Fatalf("expected allow for https://b.example")
	}
	if checkOrigin(denyReq) {
		t.Fatalf("expected deny for https://c.example")
	}
}

func TestRateLimiter_AllowBlocksAndResets(t *testing.T) {
	rl := NewRateLimiter(2, 50*time.Millisecond)

	ip := "1.2.3.4"
	if !rl.Allow(ip) || !rl.Allow(ip) {
		t.Fatalf("first two attempts should be allowed")
	}
	if rl.Allow(ip) {
		t.Fatalf("third attempt should be blocked")
	}

	time.Sleep(120 * time.Millisecond) // wait for cleanup to run
	if !rl.Allow(ip) {
		t.Fatalf("after window cleanup attempt should be allowed again")
	}
}
