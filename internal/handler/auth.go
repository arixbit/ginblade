package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/arixbit/ginblade/internal/errcode"
	"github.com/arixbit/ginblade/internal/middleware"
	"github.com/arixbit/ginblade/pkg/auth"
	"github.com/arixbit/ginblade/pkg/response"
)

// AuthHandler handles the minimal JWT example flow.
type AuthHandler struct {
	manager *auth.JWTManager
}

// NewAuthHandler creates an AuthHandler. It returns nil when auth is not configured.
func NewAuthHandler(manager *auth.JWTManager) *AuthHandler {
	if manager == nil {
		return nil
	}
	return &AuthHandler{manager: manager}
}

// CreateTokenReq is the request body for issuing a sample JWT.
type CreateTokenReq struct {
	Subject string `json:"subject" binding:"required"`
}

// CreateTokenRes is the response body for a sample JWT.
type CreateTokenRes struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

// MeRes is the response body for the protected auth example.
type MeRes struct {
	Subject string `json:"subject"`
}

// CreateToken issues a sample JWT for the given subject.
func (h *AuthHandler) CreateToken(c *gin.Context) {
	if h == nil || h.manager == nil {
		response.WriteError(c, errcode.Unauthorized)
		return
	}

	var req CreateTokenReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(200, response.BuildValidationErrorResponse(c, err))
		return
	}

	token, err := h.manager.GenerateToken(req.Subject)
	if err != nil {
		response.WriteError(c, errcode.InvalidParams)
		return
	}

	response.WriteSuccess(c, CreateTokenRes{
		AccessToken: token,
		TokenType:   "Bearer",
	})
}

// Me returns the subject from a valid Bearer token.
func (h *AuthHandler) Me(c *gin.Context) {
	response.WriteSuccess(c, MeRes{Subject: middleware.AuthSubject(c)})
}
