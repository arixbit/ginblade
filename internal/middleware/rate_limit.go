package middleware

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"github.com/arixbit/ginblade/internal/errcode"
	"github.com/arixbit/ginblade/pkg/response"
)

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// IPRateLimiter applies an in-memory per-IP token bucket.
type IPRateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rate     rate.Limit
	burst    int
	stop     chan struct{}
}

// NewIPRateLimiterPerMinute creates a per-IP limiter with the given request budget.
func NewIPRateLimiterPerMinute(requestsPerMinute int) *IPRateLimiter {
	if requestsPerMinute <= 0 {
		return nil
	}
	limiter := &IPRateLimiter{
		visitors: make(map[string]*visitor),
		rate:     rate.Every(time.Minute / time.Duration(requestsPerMinute)),
		burst:    requestsPerMinute,
		stop:     make(chan struct{}),
	}
	go limiter.cleanupLoop()
	return limiter
}

// Middleware blocks requests that exceed the configured per-IP limit.
func (l *IPRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if l == nil || l.allow(c.ClientIP()) {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(200, response.ErrorResponse(c, errcode.TooManyRequests))
	}
}

// Stop stops the cleanup goroutine.
func (l *IPRateLimiter) Stop() {
	if l == nil {
		return
	}
	select {
	case <-l.stop:
	default:
		close(l.stop)
	}
}

func (l *IPRateLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	v, ok := l.visitors[ip]
	if !ok {
		v = &visitor{limiter: rate.NewLimiter(l.rate, l.burst)}
		l.visitors[ip] = v
	}
	v.lastSeen = time.Now()
	return v.limiter.Allow()
}

func (l *IPRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			l.cleanup(time.Now().Add(-3 * time.Minute))
		case <-l.stop:
			return
		}
	}
}

func (l *IPRateLimiter) cleanup(cutoff time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for ip, v := range l.visitors {
		if v.lastSeen.Before(cutoff) {
			delete(l.visitors, ip)
		}
	}
}
