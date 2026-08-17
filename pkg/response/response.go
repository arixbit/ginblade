package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/arixbit/ginblade/internal/errcode"
	customvalidator "github.com/arixbit/ginblade/pkg/validator"
)

// Response is the standard API response structure.
type Response struct {
	Code     int            `json:"code"`
	Message  string         `json:"msg"`
	Reason   string         `json:"reason,omitempty"`
	Data     any            `json:"data,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// SuccessResponse creates a success response with code 0.
func SuccessResponse(data any) Response {
	return Response{Code: 0, Message: "success", Data: data}
}

// ErrorResponse creates an error response.
func ErrorResponse(c *gin.Context, errorCode errcode.Error) Response {
	reason := errorCode.Reason()
	return Response{
		Code:     errorCode.Code(),
		Reason:   reason,
		Message:  messageFor(reason),
		Metadata: buildMetadata(c),
	}
}

// BuildValidationErrorResponse creates a validation error response.
func BuildValidationErrorResponse(c *gin.Context, err error) Response {
	msg := err.Error()
	if errs, ok := err.(validator.ValidationErrors); ok {
		msg = customvalidator.HandleValidatorError(errs)
	}
	return Response{
		Code:     errcode.InvalidParams.Code(),
		Reason:   errcode.InvalidParams.Reason(),
		Message:  msg,
		Metadata: buildMetadata(c),
	}
}

// WriteSuccess writes a success response with HTTP 200.
func WriteSuccess(c *gin.Context, data any) {
	c.JSON(http.StatusOK, SuccessResponse(data))
}

// WriteError translates an errcode.Error into the API error envelope.
func WriteError(c *gin.Context, err error) {
	var ec errcode.Error
	if errors.As(err, &ec) {
		c.JSON(http.StatusOK, ErrorResponse(c, ec))
		return
	}
	c.JSON(http.StatusOK, ErrorResponse(c, errcode.InternalError))
}

// WriteValidationError writes a validation error response.
func WriteValidationError(c *gin.Context, err error) {
	c.JSON(http.StatusOK, BuildValidationErrorResponse(c, err))
}

func messageFor(reason string) string {
	switch reason {
	case "INVALID_PARAMS":
		return "invalid request parameters"
	case "UNAUTHORIZED":
		return "unauthorized"
	case "PERMISSION_DENIED":
		return "permission denied"
	case "TOO_MANY_REQUESTS":
		return "too many requests"
	case "REQUEST_TIMEOUT":
		return "request timeout"
	case "DATABASE_ERROR":
		return "database error"
	case "QUEUE_UNAVAILABLE":
		return "queue unavailable"
	case "QUEUE_ERROR":
		return "queue error"
	default:
		return "operation failed"
	}
}

func buildMetadata(c *gin.Context) map[string]any {
	if traceID := c.GetString("trace_id"); traceID != "" {
		return map[string]any{"trace_id": traceID}
	}
	return nil
}
