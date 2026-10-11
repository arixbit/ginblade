package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/arixbit/ginblade/internal/errcode"
	"github.com/arixbit/ginblade/internal/locales"
	"github.com/arixbit/ginblade/pkg/i18n"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	return c, w
}

// setLocalizer does what the Locale middleware does for a request in locale.
func setLocalizer(t *testing.T, c *gin.Context, locale string) {
	t.Helper()
	tr, err := i18n.New(locales.FS, "en")
	if err != nil {
		t.Fatalf("i18n.New: %v", err)
	}
	c.Set("localizer", tr.Localizer(locale))
}

func TestSuccessResponse(t *testing.T) {
	res := SuccessResponse(map[string]string{"k": "v"})
	if res.Code != 0 {
		t.Fatalf("Code = %d, want 0", res.Code)
	}
	if res.Message != "success" {
		t.Fatalf("Message = %q, want success", res.Message)
	}
}

func TestErrorResponse(t *testing.T) {
	c, _ := newTestContext()
	c.Set("trace_id", "trace-1")
	setLocalizer(t, c, "en")

	res := ErrorResponse(c, errcode.InvalidParams)
	if res.Code != errcode.InvalidParams.Code() {
		t.Fatalf("Code = %d, want %d", res.Code, errcode.InvalidParams.Code())
	}
	if res.Reason != "INVALID_PARAMS" {
		t.Fatalf("Reason = %q, want INVALID_PARAMS", res.Reason)
	}
	if res.Message != "invalid request parameters" {
		t.Fatalf("Message = %q, want invalid request parameters", res.Message)
	}
	if res.Metadata["trace_id"] != "trace-1" {
		t.Fatalf("Metadata = %v, want trace_id trace-1", res.Metadata)
	}
}

func TestErrorResponseWithoutTrace(t *testing.T) {
	c, _ := newTestContext()
	res := ErrorResponse(c, errcode.Unauthorized)
	if res.Metadata != nil {
		t.Fatalf("Metadata = %v, want nil when no trace id", res.Metadata)
	}
}

func TestBuildValidationErrorResponse(t *testing.T) {
	c, _ := newTestContext()
	res := BuildValidationErrorResponse(c, errors.New("bad request"))
	if res.Code != errcode.InvalidParams.Code() {
		t.Fatalf("Code = %d, want %d", res.Code, errcode.InvalidParams.Code())
	}
	if res.Message != "bad request" {
		t.Fatalf("Message = %q, want bad request", res.Message)
	}
}

func TestWriteSuccess(t *testing.T) {
	c, w := newTestContext()
	WriteSuccess(c, map[string]string{"k": "v"})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var body Response
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Code != 0 {
		t.Fatalf("Code = %d, want 0", body.Code)
	}
}

func TestWriteErrorKnownCode(t *testing.T) {
	c, w := newTestContext()
	WriteError(c, errcode.DatabaseError)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (business errors use 200 by convention)", w.Code)
	}
	var body Response
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Code != errcode.DatabaseError.Code() {
		t.Fatalf("Code = %d, want %d", body.Code, errcode.DatabaseError.Code())
	}
}

func TestWriteErrorUnknownError(t *testing.T) {
	c, w := newTestContext()
	WriteError(c, errors.New("unexpected"))

	var body Response
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Code != errcode.InternalError.Code() {
		t.Fatalf("Code = %d, want %d (internal)", body.Code, errcode.InternalError.Code())
	}
}

func TestWriteValidationError(t *testing.T) {
	c, w := newTestContext()
	WriteValidationError(c, errors.New("bad"))

	var body Response
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Code != errcode.InvalidParams.Code() {
		t.Fatalf("Code = %d, want %d", body.Code, errcode.InvalidParams.Code())
	}
}

func TestEnglishMessages(t *testing.T) {
	c, _ := newTestContext()
	setLocalizer(t, c, "en")
	cases := map[string]string{
		"INVALID_PARAMS":    "invalid request parameters",
		"UNAUTHORIZED":      "unauthorized",
		"PERMISSION_DENIED": "permission denied",
		"TOO_MANY_REQUESTS": "too many requests",
		"REQUEST_TIMEOUT":   "request timeout",
		"DATABASE_ERROR":    "database error",
		"QUEUE_UNAVAILABLE": "queue unavailable",
		"QUEUE_ERROR":       "queue error",
		"UNKNOWN_REASON":    "operation failed",
	}
	for reason, want := range cases {
		if got := localizer(c).Error(reason); got != want {
			t.Errorf("Error(%q) = %q, want %q", reason, got, want)
		}
	}
}

func TestErrorResponseLocalized(t *testing.T) {
	c, _ := newTestContext()
	setLocalizer(t, c, "zh-CN")

	res := ErrorResponse(c, errcode.InvalidParams)
	if res.Message != "请求参数错误" {
		t.Fatalf("Message = %q, want 请求参数错误", res.Message)
	}
	if res.Reason != "INVALID_PARAMS" {
		t.Fatalf("Reason = %q, want INVALID_PARAMS regardless of locale", res.Reason)
	}
}

func TestErrorResponseWithoutLocalizer(t *testing.T) {
	c, _ := newTestContext()
	res := ErrorResponse(c, errcode.Unauthorized)
	if res.Message != "UNAUTHORIZED" {
		t.Fatalf("Message = %q, want the reason when no Locale middleware ran", res.Message)
	}
}

func TestBuildMetadata(t *testing.T) {
	c, _ := newTestContext()
	if got := buildMetadata(c); got != nil {
		t.Fatalf("buildMetadata without trace = %v, want nil", got)
	}
	c.Set("trace_id", "t-1")
	got := buildMetadata(c)
	if got["trace_id"] != "t-1" {
		t.Fatalf("buildMetadata = %v, want trace_id t-1", got)
	}
}

func TestJSONEnvelopeShape(t *testing.T) {
	c, _ := newTestContext()
	res := ErrorResponse(c, errcode.QueueUnavailable)
	data, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	raw := string(data)
	for _, field := range []string{`"code"`, `"msg"`, `"reason"`} {
		if !strings.Contains(raw, field) {
			t.Errorf("envelope missing %s: %s", field, raw)
		}
	}
}
