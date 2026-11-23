package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	applog "github.com/arixbit/ginblade/pkg/log"
)

func TestTraceLoggerSanitizesRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	restore := applog.SetLogger(zap.NewNop())
	defer restore()

	router := gin.New()
	router.Use(TraceLogger(false, nil))
	router.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, applog.TraceIDFrom(c.Request.Context()))
	})

	t.Run("accepts valid trace id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.Header.Set("X-Request-ID", "  req_123-abc.xyz:1/2  ")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		const want = "req_123-abc.xyz:1/2"
		if got := rec.Header().Get("X-Request-ID"); got != want {
			t.Fatalf("header X-Request-ID = %q, want %q", got, want)
		}
		if got := rec.Body.String(); got != want {
			t.Fatalf("context trace_id = %q, want %q", got, want)
		}
	})

	invalidCases := []struct {
		name  string
		value string
	}{
		{"empty", ""},
		{"whitespace", "   "},
		{"crlf injection", "abc\r\nX-Injected: 1"},
		{"newline", "abc\ndef"},
		{"spaces", "abc def"},
		{"quotes", `abc"def`},
		{"too long", strings.Repeat("a", maxTraceIDLen+1)},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/ping", nil)
			if tc.value != "" {
				req.Header.Set("X-Request-ID", tc.value)
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			got := rec.Header().Get("X-Request-ID")
			if _, err := uuid.Parse(got); err != nil {
				t.Fatalf("expected generated UUID for %q, got %q (%v)", tc.name, got, err)
			}
			if rec.Body.String() != got {
				t.Fatalf("context trace_id = %q, want %q", rec.Body.String(), got)
			}
		})
	}
}
