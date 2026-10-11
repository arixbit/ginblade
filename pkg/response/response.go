package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/arixbit/ginblade/internal/errcode"
	"github.com/arixbit/ginblade/pkg/i18n"
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
		Message:  localizer(c).Error(reason),
		Metadata: buildMetadata(c),
	}
}

// BuildValidationErrorResponse creates a validation error response.
func BuildValidationErrorResponse(c *gin.Context, err error) Response {
	msg := err.Error()
	if errs, ok := err.(validator.ValidationErrors); ok {
		msg = customvalidator.HandleValidatorError(localizer(c), errs)
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

// localizer returns the request's localizer stored by the Locale middleware.
// Without one, the zero Localizer returns message keys unchanged.
func localizer(c *gin.Context) i18n.Localizer {
	v, _ := c.Get("localizer")
	l, _ := v.(i18n.Localizer)
	return l
}

func buildMetadata(c *gin.Context) map[string]any {
	if traceID := c.GetString("trace_id"); traceID != "" {
		return map[string]any{"trace_id": traceID}
	}
	return nil
}
