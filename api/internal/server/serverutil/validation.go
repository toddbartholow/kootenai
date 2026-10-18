package serverutil

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"

	appi18n "github.com/toddbartholow/kootenai/api/internal/i18n"
)

const (
	// MaxIDLength is the maximum length for ID parameters.
	MaxIDLength = 255

	// MaxNameLength is the maximum length for name parameters.
	MaxNameLength = 255
)

var (
	// UUIDRegex matches standard UUID format.
	UUIDRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

	// SafeNameRegex matches safe alphanumeric names with common separators.
	SafeNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_\-\.]*$`)

	// VMIDRegex matches Proxmox VMID format (positive integers).
	VMIDRegex = regexp.MustCompile(`^[1-9][0-9]*$`)
)

// Validate holds the validator instance.
var Validate = validator.New()

func init() {
	// Use json tag names in error messages.
	Validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})
}

// ValidationError represents a validation error with field details.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationErrorResponse is the response for validation errors.
type ValidationErrorResponse struct {
	Error   string            `json:"error"`
	Details []ValidationError `json:"details"`
}

// FieldValidationError is returned by the Validate* helpers. It implements
// httputil.LocalizableError so handlers can pass it straight to
// Responder.LocalizedErrorResponseFromErr for a locale-aware response.
type FieldValidationError struct {
	Field string
	MsgID string
	Max   int
}

func (e *FieldValidationError) Error() string {
	return e.Field + ": " + e.MsgID
}

func (e *FieldValidationError) MessageID() string { return e.MsgID }

func (e *FieldValidationError) TemplateData() map[string]any {
	data := map[string]any{"Field": e.Field}
	if e.Max > 0 {
		data["Max"] = e.Max
	}
	return data
}

func fieldErr(field, msgID string) *FieldValidationError {
	return &FieldValidationError{Field: field, MsgID: msgID}
}

func fieldErrMax(field, msgID string, max int) *FieldValidationError {
	return &FieldValidationError{Field: field, MsgID: msgID, Max: max}
}

// DecodeAndValidate decodes JSON body and validates the struct, producing
// localized error messages using the Localizer attached to r.Context().
func DecodeAndValidate[T any](r *http.Request) (*T, []ValidationError) {
	ctx := r.Context()
	var req T
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, []ValidationError{{
			Field:   "body",
			Message: appi18n.Localize(ctx, "validation.bodyInvalidJSON", nil),
		}}
	}

	if err := Validate.Struct(req); err != nil {
		return nil, FormatValidationErrors(ctx, err)
	}

	return &req, nil
}

// FormatValidationErrors converts validator errors to ValidationError slice
// using the request's localizer.
func FormatValidationErrors(ctx context.Context, err error) []ValidationError {
	var errors []ValidationError

	if validationErrs, ok := err.(validator.ValidationErrors); ok {
		for _, e := range validationErrs {
			errors = append(errors, ValidationError{
				Field:   e.Field(),
				Message: FormatErrorMessage(ctx, e),
			})
		}
	}

	return errors
}

// FormatErrorMessage creates a localized, human-readable error message for a
// single validator.FieldError.
func FormatErrorMessage(ctx context.Context, e validator.FieldError) string {
	data := map[string]any{
		"Field": e.Field(),
		"Param": e.Param(),
		"Tag":   e.Tag(),
	}
	switch e.Tag() {
	case "required":
		return appi18n.Localize(ctx, "validation.required", data)
	case "email":
		return appi18n.Localize(ctx, "validation.email", data)
	case "min":
		return appi18n.Localize(ctx, "validation.min", data)
	case "max":
		return appi18n.Localize(ctx, "validation.max", data)
	case "uuid":
		return appi18n.Localize(ctx, "validation.uuid", data)
	case "oneof":
		return appi18n.Localize(ctx, "validation.oneof", data)
	case "gte":
		return appi18n.Localize(ctx, "validation.gte", data)
	case "lte":
		return appi18n.Localize(ctx, "validation.lte", data)
	case "url":
		return appi18n.Localize(ctx, "validation.url", data)
	default:
		return appi18n.Localize(ctx, "validation.generic", data)
	}
}

// ValidateID validates an ID parameter (UUID or safe name).
func ValidateID(field, value string) (string, error) {
	if value == "" {
		return "", fieldErr(field, "validation.empty")
	}

	value = strings.TrimSpace(value)

	if len(value) > MaxIDLength {
		return "", fieldErrMax(field, "validation.maxLength", MaxIDLength)
	}

	if strings.Contains(value, "..") || strings.Contains(value, "//") || strings.Contains(value, "\\") {
		return "", fieldErr(field, "validation.invalidPath")
	}

	if UUIDRegex.MatchString(value) {
		return value, nil
	}

	if SafeNameRegex.MatchString(value) {
		return value, nil
	}

	return "", fieldErr(field, "validation.invalidIDFormat")
}

// ValidateUUID validates a UUID parameter.
func ValidateUUID(field, value string) (string, error) {
	if value == "" {
		return "", fieldErr(field, "validation.empty")
	}

	value = strings.TrimSpace(value)

	if !UUIDRegex.MatchString(value) {
		return "", fieldErr(field, "validation.uuid")
	}

	return value, nil
}

// ValidateName validates a name parameter.
func ValidateName(field, value string) (string, error) {
	if value == "" {
		return "", fieldErr(field, "validation.empty")
	}

	value = strings.TrimSpace(value)

	if len(value) > MaxNameLength {
		return "", fieldErrMax(field, "validation.maxLength", MaxNameLength)
	}

	if strings.Contains(value, "..") || strings.Contains(value, "//") || strings.Contains(value, "\\") {
		return "", fieldErr(field, "validation.invalidPath")
	}

	if !SafeNameRegex.MatchString(value) {
		return "", fieldErr(field, "validation.invalidNameFormat")
	}

	return value, nil
}

// ValidateVMID validates a Proxmox VM ID.
func ValidateVMID(field, value string) (string, error) {
	if value == "" {
		return "", fieldErr(field, "validation.empty")
	}

	value = strings.TrimSpace(value)

	if !VMIDRegex.MatchString(value) {
		return "", fieldErr(field, "validation.invalidVMID")
	}

	return value, nil
}

// ValidateOptionalID validates an optional ID parameter (can be empty).
func ValidateOptionalID(field, value string) (string, error) {
	if value == "" {
		return "", nil
	}
	return ValidateID(field, value)
}
