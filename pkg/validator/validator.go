package validator

import (
	"strings"

	"github.com/go-playground/validator/v10"

	"github.com/arixbit/ginblade/internal/errcode"
	"github.com/arixbit/ginblade/pkg/i18n"
)

// InitValidator is the hook for registering custom validation rules.
func InitValidator() {
}

// HandleValidatorError converts validation errors into a concise client message in l's locale.
func HandleValidatorError(l i18n.Localizer, errs validator.ValidationErrors) string {
	if len(errs) == 0 {
		return l.Error(errcode.InvalidParams.Reason())
	}

	err := errs[0]
	return l.Validation(err.Tag(), strings.ToLower(err.Field()), err.Param())
}
