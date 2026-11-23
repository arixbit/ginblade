package response

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// BuildValidationErrorResponse creates a validation error response without leaking internal Go errors.
func BuildValidationErrorResponse(c *gin.Context, err error) Response {
	return Response{
		Code:     errcode.InvalidParams.Code(),
		Reason:   errcode.InvalidParams.Reason(),
		Message:  validationErrorMessage(err),
		Metadata: buildMetadata(c),
	}
}

func validationErrorMessage(err error) string {
	if err == nil {
		return messageFor(errcode.InvalidParams.Reason())
	}

	var errs validator.ValidationErrors
	if errors.As(err, &errs) {
		return customvalidator.HandleValidatorError(errs)
	}

	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return "malformed request body"
	}

	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		if typeErr.Field != "" {
			return fmt.Sprintf("invalid value for %s", typeErr.Field)
		}
		return "invalid request body"
	}

	return messageFor(errcode.InvalidParams.Reason())
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
	if errors.Is(err, context.DeadlineExceeded) {
		c.JSON(http.StatusOK, ErrorResponse(c, errcode.RequestTimeout))
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
