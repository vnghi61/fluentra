package validation

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

// FieldError is a localized validation failure suitable for API rendering.
type FieldError struct {
	Field   string
	Code    string
	Message string
}

// Validator validates transport shapes using struct tags.
type Validator struct{ validate *validator.Validate }

// New creates the standard Fluentra validator.
func New() *Validator { return &Validator{validate: validator.New()} }

// Check validates value and returns localized field failures.
func (v *Validator) Check(value any, locale string) []FieldError {
	err := v.validate.Struct(value)
	if err == nil {
		return nil
	}
	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		return []FieldError{{Code: "invalid", Message: "Invalid value."}}
	}
	result := make([]FieldError, 0, len(validationErrors))
	for _, fieldErr := range validationErrors {
		field := strings.ToLower(fieldErr.Field())
		result = append(result, FieldError{Field: field, Code: fieldErr.Tag(), Message: message(locale, field, fieldErr)})
	}
	return result
}

func message(locale, field string, fieldErr validator.FieldError) string {
	if locale == "vi" {
		switch fieldErr.Tag() {
		case "required":
			return fmt.Sprintf("Trường %s là bắt buộc.", field)
		case "email":
			return fmt.Sprintf("Trường %s phải là email hợp lệ.", field)
		}
	}
	switch fieldErr.Tag() {
	case "required":
		return fmt.Sprintf("%s is required.", field)
	case "email":
		return fmt.Sprintf("%s must be a valid email address.", field)
	case "min":
		return fmt.Sprintf("%s must be at least %s characters.", field, fieldErr.Param())
	}
	return fmt.Sprintf("%s is invalid.", field)
}
