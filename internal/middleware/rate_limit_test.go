package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/arixbit/ginblade/internal/errcode"
	"github.com/arixbit/ginblade/pkg/response"
)

func TestIPRateLimiterConcurrentStop(t *testing.T) {
	limiter := NewIPRateLimiterPerMinute(60)
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			limiter.Stop()
		}()
	}
	wg.Wait()
}

func TestIPRateLimiterBlocksExcessRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := NewIPRateLimiterPerMinute(1)
	defer limiter.Stop()

	router := gin.New()
	router.Use(limiter.Middleware())
	router.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req1 := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req1.RemoteAddr = "203.0.113.1:1234"
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK || rec1.Body.Len() != 0 {
		t.Fatalf("first request unexpected response: status=%d body=%q", rec1.Code, rec1.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req2.RemoteAddr = "203.0.113.1:1234"
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)

	var body response.Response
	if err := json.Unmarshal(rec2.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal limited response: %v", err)
	}
	if body.Code != errcode.TooManyRequests.Code() {
		t.Fatalf("Code = %d, want %d", body.Code, errcode.TooManyRequests.Code())
	}
}

func TestIPRateLimiterCapsVisitorsAndEvictsStale(t *testing.T) {
	limiter := NewIPRateLimiterPerMinute(60)
	defer limiter.Stop()
	limiter.maxVisitors = 2

	if !limiter.allow("198.51.100.1") {
		t.Fatal("expected ip1 allowed")
	}
	if !limiter.allow("198.51.100.2") {
		t.Fatal("expected ip2 allowed")
	}
	if limiter.allow("198.51.100.3") {
		t.Fatal("expected new ip3 rejected when visitor table is full of active entries")
	}
	if !limiter.allow("198.51.100.1") {
		t.Fatal("expected existing ip1 still allowed when table is full")
	}

	// Mark ip1 as stale so the next new IP triggers eviction and succeeds.
	limiter.mu.Lock()
	limiter.visitors["198.51.100.1"].lastSeen = time.Now().Add(-visitorTTL - time.Second)
	limiter.mu.Unlock()

	if !limiter.allow("198.51.100.3") {
		t.Fatal("expected ip3 allowed after stale visitor eviction")
	}
	if got := len(limiter.visitors); got != 2 {
		t.Fatalf("visitors count = %d, want 2 (%s)", got, fmt.Sprintf("%v", limiter.visitors))
	}
}
