package repositories

import "errors"

// Repository-level sentinel errors. Callers should use errors.Is to detect
// these conditions rather than substring-matching error strings. Strings are
// kept stable so existing tests that assert on Error() output keep passing.

var (
	// ErrLicenseNotFound is returned when a license lookup misses or an
	// update/delete affects zero rows.
	ErrLicenseNotFound = errors.New("license not found")

	// ErrRoleNotFoundOrSystem is returned when the targeted role doesn't
	// exist or is a built-in system role that callers can't mutate.
	ErrRoleNotFoundOrSystem = errors.New("role not found or is a system role")

	// ErrRoleAssignmentNotFound is returned when revoking a role assignment
	// that doesn't exist for the (user, role) pair.
	ErrRoleAssignmentNotFound = errors.New("role assignment not found")

	// ErrInvitationInvalid is returned when an invitation token is either
	// unknown, already accepted, or past its expiry.
	ErrInvitationInvalid = errors.New("invalid or expired invitation token")
)
