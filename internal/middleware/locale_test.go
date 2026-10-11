package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/arixbit/ginblade/internal/errcode"
	"github.com/arixbit/ginblade/internal/locales"
	"github.com/arixbit/ginblade/pkg/i18n"
	"github.com/arixbit/ginblade/pkg/response"
)

func TestLocaleTranslatesErrorMessages(t *testing.T) {
	tr, err := i18n.New(locales.FS, "en")
	if err != nil {
		t.Fatalf("i18n.New: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Locale(tr))
	router.GET("/fail", func(c *gin.Context) {
		response.WriteError(c, errcode.Unauthorized)
	})

	tests := []struct {
		name           string
		target         string
		acceptLanguage string
		wantMsg        string
	}{
		{"default locale", "/fail", "", "unauthorized"},
		{"accept-language header", "/fail", "zh-CN,zh;q=0.9", "未认证"},
		{"query overrides header", "/fail?lang=en", "zh-CN", "unauthorized"},
		{"query alone", "/fail?lang=zh-CN", "", "未认证"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.target, nil)
			if tt.acceptLanguage != "" {
				req.Header.Set("Accept-Language", tt.acceptLanguage)
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			var body response.Response
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if body.Message != tt.wantMsg {
				t.Errorf("msg = %q, want %q", body.Message, tt.wantMsg)
			}
			if body.Reason != "UNAUTHORIZED" {
				t.Errorf("reason = %q, want UNAUTHORIZED in every locale", body.Reason)
			}
		})
	}
}
