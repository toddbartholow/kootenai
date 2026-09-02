// Package session provides session business logic
package session

import (
	"errors"

	"github.com/toddbartholow/kootenai/api/internal/apperrors"
)

// Service-specific errors
var (
	// ErrSessionNotFound indicates the requested session does not exist
	ErrSessionNotFound = errors.New("session not found")

	// ErrSessionAlreadySubmitted indicates the session was already submitted
	ErrSessionAlreadySubmitted = errors.New("session already submitted")

	// ErrSessionAlreadyEnded indicates the session was already ended
	ErrSessionAlreadyEnded = errors.New("session already ended")

	// ErrForbidden is the canonical forbidden error from apperrors.
	ErrForbidden = apperrors.ErrForbidden

	// ErrProgressUnavailable indicates checkpoint progress data is not available
	ErrProgressUnavailable = errors.New("progress data unavailable")

	// ErrInvalidRequest indicates invalid request parameters
	ErrInvalidRequest = errors.New("invalid request parameters")

	// ErrSessionRepoNotAvailable indicates the session repository is not configured
	ErrSessionRepoNotAvailable = errors.New("session repository not available")
)
