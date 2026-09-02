// Package pathway provides the service layer for pathway operations.
package pathway

import (
	"errors"

	"github.com/toddbartholow/kootenai/api/internal/apperrors"
)

// Service errors for pathway operations.
var (
	// ErrPathwayNotFound indicates the requested pathway doesn't exist.
	ErrPathwayNotFound = errors.New("pathway not found")

	// ErrEnrollmentNotFound indicates the requested enrollment doesn't exist.
	ErrEnrollmentNotFound = errors.New("enrollment not found")

	// ErrModuleNotFound indicates the requested module doesn't exist.
	ErrModuleNotFound = errors.New("module not found")

	// ErrAlreadyEnrolled indicates the user is already enrolled in the pathway.
	ErrAlreadyEnrolled = errors.New("already enrolled in pathway")

	// ErrNotEnrolled indicates the user is not enrolled in the pathway.
	ErrNotEnrolled = errors.New("not enrolled in pathway")

	// ErrModuleNotInPathway indicates the module doesn't belong to the pathway.
	ErrModuleNotInPathway = errors.New("module does not belong to pathway")

	// ErrForbidden is the canonical forbidden error from apperrors.
	ErrForbidden = apperrors.ErrForbidden

	// ErrRepoNotAvailable indicates a required repository is not configured.
	ErrRepoNotAvailable = errors.New("pathway repository not available")
)
