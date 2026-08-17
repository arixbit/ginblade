package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/arixbit/ginblade/pkg/auth"
	"github.com/arixbit/ginblade/pkg/response"
	"github.com/arixbit/ginblade/pkg/validator"
)

func TestAuthHandlerCreateToken(t *testing.T) {
	validator.InitValidator()
	manager, err := auth.NewJWTManager(auth.JWTConfig{
		Secret: "test-secret",
		Issuer: "ginblade-test",
		TTL:    time.Hour,
	})
	if err != nil {
		t.Fatalf("NewJWTManager: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/auth/token", NewAuthHandler(manager).CreateToken)

	req := httptest.NewRequest(http.MethodPost, "/auth/token", strings.NewReader(`{"subject":"subject-1"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	var body response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body.Code != 0 {
		t.Fatalf("expected success code, got %d", body.Code)
	}

	data, ok := body.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected response data map, got %T", body.Data)
	}
	token, ok := data["access_token"].(string)
	if !ok || token == "" {
		t.Fatalf("expected access token, got %#v", data["access_token"])
	}
	if _, err := manager.ParseToken(token); err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
}
