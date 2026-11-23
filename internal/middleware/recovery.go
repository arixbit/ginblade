package middleware

import (
	"net/http"
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
				if err == http.ErrAbortHandler {
					panic(err)
				}
				applog.FromContext(c.Request.Context()).Error("panic recovered",
					zap.String("method", c.Request.Method),
					zap.String("path", c.Request.URL.Path),
					zap.Any("error", err),
					zap.ByteString("stacktrace", debug.Stack()),
				)
				if c.Writer.Written() {
					c.Abort()
					return
				}
				c.AbortWithStatusJSON(http.StatusOK, response.ErrorResponse(c, errcode.InternalError))
			}
		}()
		c.Next()
	}
}
