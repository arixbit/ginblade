package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/arixbit/ginblade/internal/errcode"
	applog "github.com/arixbit/ginblade/pkg/log"
	"github.com/arixbit/ginblade/pkg/response"
)

func TestRecoveryReturnsInternalErrorEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	restore := applog.SetLogger(zap.NewNop())
	defer restore()

	router := gin.New()
	router.Use(Recovery())
	router.GET("/panic", func(c *gin.Context) {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Code != errcode.InternalError.Code() {
		t.Fatalf("Code = %d, want %d", body.Code, errcode.InternalError.Code())
	}
}

func TestRecoveryRepanicsErrAbortHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	restore := applog.SetLogger(zap.NewNop())
	defer restore()

	router := gin.New()
	router.Use(Recovery())
	router.GET("/abort", func(c *gin.Context) {
		panic(http.ErrAbortHandler)
	})

	defer func() {
		recovered := recover()
		if recovered != http.ErrAbortHandler {
			t.Fatalf("recovered = %v, want http.ErrAbortHandler", recovered)
		}
	}()

	req := httptest.NewRequest(http.MethodGet, "/abort", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
}

func TestRecoveryDoesNotCorruptWrittenResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	restore := applog.SetLogger(zap.NewNop())
	defer restore()

	router := gin.New()
	router.Use(Recovery())
	router.GET("/partial", func(c *gin.Context) {
		c.String(http.StatusOK, "partial-output")
		panic("late boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/partial", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if got := rec.Body.String(); got != "partial-output" {
		t.Fatalf("body = %q, want partial-output without appended JSON", got)
	}
}
