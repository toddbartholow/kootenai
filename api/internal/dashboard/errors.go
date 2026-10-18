// Package dashboard provides the service layer for dashboard operations.
package dashboard

import "errors"

// Service errors for dashboard operations.
var (
	// ErrUserNotFound indicates the requested user doesn't exist.
	ErrUserNotFound = errors.New("user not found")

	// ErrRepoNotAvailable indicates a required repository is not configured.
	ErrRepoNotAvailable = errors.New("dashboard repository not available")
)
