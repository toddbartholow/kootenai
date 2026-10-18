// Package server provides the HTTP server and API routes.
//
// Validation types and functions are defined in serverutil and re-exported here
// as aliases for backward compatibility. New sub-packages should import
// serverutil directly.
package server

import (
	"net/http"

	"github.com/toddbartholow/kootenai/api/internal/server/serverutil"
)

// Constants (re-exported from serverutil).
const (
	MaxIDLength   = serverutil.MaxIDLength
	MaxNameLength = serverutil.MaxNameLength
)

// Package-level validator (re-exported from serverutil).
var validate = serverutil.Validate

// Type aliases.
type ValidationError = serverutil.ValidationError
type ValidationErrorResponse = serverutil.ValidationErrorResponse
type FieldValidationError = serverutil.FieldValidationError

// Function aliases.
var (
	ValidateID         = serverutil.ValidateID
	ValidateUUID       = serverutil.ValidateUUID
	ValidateName       = serverutil.ValidateName
	ValidateVMID       = serverutil.ValidateVMID
	ValidateOptionalID = serverutil.ValidateOptionalID
)

// decodeAndValidate delegates to serverutil.DecodeAndValidate.
// Generic functions cannot be assigned to variables, so this is a wrapper.
func decodeAndValidate[T any](r *http.Request) (*T, []ValidationError) {
	return serverutil.DecodeAndValidate[T](r)
}

// formatValidationErrors delegates to serverutil.FormatValidationErrors.
var formatValidationErrors = serverutil.FormatValidationErrors
