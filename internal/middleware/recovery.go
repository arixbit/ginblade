package middleware

import (
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/arixbit/ginblade/internal/errcode"
	applog "github.com/arixbit/ginblade/pkg/log"
	"github.com/arixbit/ginblade/pkg/response"
)

// Recovery catches panics and returns the standard API error envelope.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				applog.FromContext(c.Request.Context()).Error("panic recovered",
					zap.String("method", c.Request.Method),
					zap.String("path", c.Request.URL.Path),
					zap.Any("error", err),
					zap.ByteString("stacktrace", debug.Stack()),
				)
				c.AbortWithStatusJSON(200, response.ErrorResponse(c, errcode.InternalError))
			}
		}()
		c.Next()
	}
}
