package server

import (
	"github.com/toddbartholow/kootenai/api/internal/apperrors"
	"github.com/toddbartholow/kootenai/api/internal/server/httputil"
)

// Common error messages for clients (aliased to httputil constants).
const (
	ErrMsgInternalError    = httputil.ErrMsgInternalError
	ErrMsgDatabaseError    = httputil.ErrMsgDatabaseError
	ErrMsgNotFound         = httputil.ErrMsgNotFound
	ErrMsgUnauthorized     = httputil.ErrMsgUnauthorized
	ErrMsgForbidden        = httputil.ErrMsgForbidden
	ErrMsgBadRequest       = httputil.ErrMsgBadRequest
	ErrMsgConflict         = httputil.ErrMsgConflict
	ErrMsgValidationFailed = httputil.ErrMsgValidationFailed
)

// SanitizedError is re-exported so existing callers don't need to switch
// imports in lockstep.
type SanitizedError = httputil.SanitizedError

// Common sentinel errors for the application (aliases for apperrors).
var (
	ErrNotFound     = apperrors.ErrNotFound
	ErrUnauthorized = apperrors.ErrUnauthorized
	ErrForbidden    = apperrors.ErrForbidden
	ErrConflict     = apperrors.ErrConflict
	// ErrValidation is the sentinel for validation errors. It's the same
	// value as httputil.ErrValidation so `errors.Is` works across both.
	ErrValidation = httputil.ErrValidation
)
