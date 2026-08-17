package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	applog "github.com/arixbit/ginblade/pkg/log"
)

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
		traceID := c.GetHeader("X-Request-ID")
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
