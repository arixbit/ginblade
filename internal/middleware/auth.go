package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/arixbit/ginblade/internal/errcode"
	"github.com/arixbit/ginblade/pkg/auth"
	"github.com/arixbit/ginblade/pkg/response"
)

const authSubjectKey = "auth_subject"

// AuthSubject returns the authenticated subject stored by BearerAuth.
func AuthSubject(c *gin.Context) string {
	return c.GetString(authSubjectKey)
}

// BearerAuth validates Authorization Bearer JWT tokens and stores the subject.
func BearerAuth(manager *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if manager == nil {
			c.AbortWithStatusJSON(http.StatusOK, response.ErrorResponse(c, errcode.Unauthorized))
			return
		}

		claims, err := manager.ParseToken(c.GetHeader("Authorization"))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusOK, response.ErrorResponse(c, errcode.Unauthorized))
			return
		}

		c.Set(authSubjectKey, claims.Subject)
		c.Next()
	}
}
