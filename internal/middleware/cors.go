package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS returns a simple allow-list based CORS middleware.
func CORS(allowOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowOrigins))
	for _, origin := range allowOrigins {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			allowed[origin] = struct{}{}
		}
	}

	return func(c *gin.Context) {
		if len(allowed) > 0 {
			c.Writer.Header().Add("Vary", "Origin")
		}

		origin := strings.TrimSpace(c.GetHeader("Origin"))
		originAllowed := origin != "" && containsOrigin(allowed, origin)
		isPreflight := c.Request.Method == http.MethodOptions &&
			origin != "" &&
			strings.TrimSpace(c.GetHeader("Access-Control-Request-Method")) != ""

		if isPreflight {
			c.Writer.Header().Add("Vary", "Access-Control-Request-Method")
			c.Writer.Header().Add("Vary", "Access-Control-Request-Headers")
		}

		if originAllowed {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Origin,Content-Type,Accept,Authorization,X-Request-ID")
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		if isPreflight {
			if originAllowed {
				c.AbortWithStatus(http.StatusNoContent)
			} else {
				c.AbortWithStatus(http.StatusForbidden)
			}
			return
		}
		c.Next()
	}
}

func containsOrigin(allowed map[string]struct{}, origin string) bool {
	_, ok := allowed[origin]
	return ok
}
