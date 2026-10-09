// Package apperrors provides canonical error sentinels shared across the application.
// This package must not import any other internal packages to prevent import cycles.
package apperrors

import "errors"

var (
	// ErrNotFound is returned when a requested resource does not exist.
	ErrNotFound = errors.New("not found")

	// ErrForbidden is returned when a user lacks permission for an operation.
	ErrForbidden = errors.New("forbidden")

	// ErrConflict is returned when an operation conflicts with existing state.
	ErrConflict = errors.New("conflict")

	// ErrUnauthorized is returned when authentication is required but missing.
	ErrUnauthorized = errors.New("unauthorized")
)
