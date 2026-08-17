package validator

import (
	"testing"

	"github.com/go-playground/validator/v10"
)

func TestHandleValidatorErrorEmpty(t *testing.T) {
	if got := HandleValidatorError(nil); got != "invalid request parameters" {
		t.Fatalf("HandleValidatorError(nil) = %q, want default message", got)
	}
}

type requiredStruct struct {
	Name string `validate:"required"`
}

type minStruct struct {
	Age int `validate:"min=18"`
}

type maxStruct struct {
	Score int `validate:"max=100"`
}

type emailStruct struct {
	Email string `validate:"email"`
}

func validateTag(t *testing.T, value any) validator.ValidationErrors {
	t.Helper()
	err := validator.New().Struct(value)
	if err == nil {
		t.Fatalf("expected validation error for %+v", value)
	}
	errs, ok := err.(validator.ValidationErrors)
	if !ok {
		t.Fatalf("expected ValidationErrors, got %T", err)
	}
	return errs
}

func TestHandleValidatorErrorRequired(t *testing.T) {
	got := HandleValidatorError(validateTag(t, requiredStruct{}))
	if got != "name is required" {
		t.Fatalf("required message = %q, want 'name is required'", got)
	}
}

func TestHandleValidatorErrorMin(t *testing.T) {
	got := HandleValidatorError(validateTag(t, minStruct{Age: 10}))
	if got != "age must be at least 18" {
		t.Fatalf("min message = %q, want 'age must be at least 18'", got)
	}
}

func TestHandleValidatorErrorMax(t *testing.T) {
	got := HandleValidatorError(validateTag(t, maxStruct{Score: 200}))
	if got != "score must be at most 100" {
		t.Fatalf("max message = %q, want 'score must be at most 100'", got)
	}
}

func TestHandleValidatorErrorDefault(t *testing.T) {
	got := HandleValidatorError(validateTag(t, emailStruct{Email: "not-an-email"}))
	if got != "email is invalid" {
		t.Fatalf("default message = %q, want 'email is invalid'", got)
	}
}
