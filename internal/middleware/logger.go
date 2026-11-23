package middleware

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	applog "github.com/arixbit/ginblade/pkg/log"
)

const maxTraceIDLen = 128

// TraceLogger attaches a trace ID to each request and optionally logs request lifecycle fields.
func TraceLogger(auditEnabled bool, auditExcludes []string) gin.HandlerFunc {
	excludes := make(map[string]struct{}, len(auditExcludes))
	for _, path := range auditExcludes {
		if path != "" {
			excludes[path] = struct{}{}
		}
	}

	return func(c *gin.Context) {
		start := time.Now()
		traceID := sanitizeTraceID(c.GetHeader("X-Request-ID"))
		if traceID == "" {
			traceID = uuid.NewString()
		}
		c.Set("trace_id", traceID)
		c.Header("X-Request-ID", traceID)
		c.Request = c.Request.WithContext(applog.WithTraceID(c.Request.Context(), traceID))

		c.Next()

		if !auditEnabled {
			return
		}
		if _, skip := excludes[c.Request.URL.Path]; skip {
			return
		}
		applog.FromContext(c.Request.Context()).Info("http request completed",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("client_ip", c.ClientIP()),
		)
	}
}

// sanitizeTraceID validates client-supplied request IDs to prevent header/log injection
// and unbounded payload growth. Invalid values return empty so a new UUID is generated.
func sanitizeTraceID(raw string) string {
	id := strings.TrimSpace(raw)
	if id == "" || len(id) > maxTraceIDLen {
		return ""
	}
	for i := 0; i < len(id); i++ {
		ch := id[i]
		isAlphaNum := (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')
		isAllowedPunct := ch == '-' || ch == '_' || ch == '.' || ch == ':' || ch == '/'
		if !isAlphaNum && !isAllowedPunct {
			return ""
		}
	}
	return id
}
