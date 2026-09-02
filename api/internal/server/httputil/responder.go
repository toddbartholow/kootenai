// Package httputil provides shared HTTP response utilities for the server package.
// It decouples handler response writing from the Server struct, enabling managers
// to send HTTP responses without a *Server backpointer.
package httputil

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/lib/pq"

	"github.com/toddbartholow/kootenai/api/internal/apperrors"
	appi18n "github.com/toddbartholow/kootenai/api/internal/i18n"
	"github.com/toddbartholow/kootenai/api/internal/orchestrator"
)

// Common error messages for clients (generic, no sensitive details).
// These constants now double as English-catalog fallbacks; handlers that have
// a context.Context in scope should prefer LocalizedErrorResponse so the
// response respects the caller's Accept-Language.
const (
	ErrMsgInternalError    = "an internal error occurred"
	ErrMsgDatabaseError    = "failed to process request"
	ErrMsgNotFound         = "resource not found"
	ErrMsgUnauthorized     = "authentication required"
	ErrMsgForbidden        = "access denied"
	ErrMsgBadRequest       = "invalid request"
	ErrMsgConflict         = "resource already exists"
	ErrMsgValidationFailed = "validation failed"
)

// Message IDs for client-facing error messages. Pair with
// LocalizedErrorResponse to serve the caller's locale.
const (
	MsgIDInternalError    = "error.internal"
	MsgIDDatabaseError    = "error.database"
	MsgIDNotFound         = "error.notFound"
	MsgIDUnauthorized     = "error.unauthorized"
	MsgIDForbidden        = "error.forbidden"
	MsgIDBadRequest       = "error.badRequest"
	MsgIDConflict         = "error.conflict"
	MsgIDValidationFailed = "error.validationFailed"
	MsgIDNotImplemented   = "error.notImplemented"
)

// ValidationError represents a field-level validation error.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationErrorResponse is the response for validation errors.
type ValidationErrorResponse struct {
	Error   string            `json:"error"`
	Details []ValidationError `json:"details"`
}

// SanitizedError represents an error that's safe to return to clients.
type SanitizedError struct {
	ClientMessage string
	InternalError error
	StatusCode    int
}

func (e *SanitizedError) Error() string {
	return e.ClientMessage
}

func (e *SanitizedError) Unwrap() error {
	return e.InternalError
}

// Responder provides HTTP response helpers with structured logging.
// It satisfies the audit.ResponseWriter and features.ResponseWriter interfaces.
type Responder struct {
	Logger *slog.Logger
}

// NewResponder creates a new Responder.
func NewResponder(logger *slog.Logger) *Responder {
	if logger == nil {
		logger = slog.Default()
	}
	return &Responder{Logger: logger}
}

// JSONResponse writes a JSON response with the given status code.
func (r *Responder) JSONResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		r.Logger.Error("Failed to encode response", "error", err)
	}
}

// ErrorResponse writes a JSON error response with the given status code and message.
func (r *Responder) ErrorResponse(w http.ResponseWriter, status int, message string) {
	r.JSONResponse(w, status, map[string]string{
		"error": message,
	})
}

// LocalizedErrorResponse writes a JSON error response using the caller's
// request-scoped Localizer (attached by the locale middleware). It resolves
// messageID against the catalog, interpolating templateData into named
// parameters like `{{.Field}}`, and falls back to the English message when
// no localizer is on the context.
//
// Prefer this over ErrorResponse whenever a context.Context is in scope.
func (r *Responder) LocalizedErrorResponse(ctx context.Context, w http.ResponseWriter, status int, messageID string, templateData map[string]any) {
	message := appi18n.Localize(ctx, messageID, templateData)
	r.ErrorResponse(w, status, message)
}

// LocalizableError is implemented by errors that carry a catalog message ID
// plus template data. Handlers can pass any error to LocalizedErrorResponseFromErr;
// those that implement this interface render in the caller's locale, others
// fall back to their Error() string.
type LocalizableError interface {
	error
	MessageID() string
	TemplateData() map[string]any
}

// LocalizedErrorResponseFromErr writes a JSON error response localized from
// err. If err (or anything it wraps) implements LocalizableError, the message
// is resolved against the caller's localizer; otherwise err.Error() is used
// verbatim.
func (r *Responder) LocalizedErrorResponseFromErr(ctx context.Context, w http.ResponseWriter, status int, err error) {
	var le LocalizableError
	if errors.As(err, &le) {
		r.LocalizedErrorResponse(ctx, w, status, le.MessageID(), le.TemplateData())
		return
	}
	r.ErrorResponse(w, status, err.Error())
}

// LocalizedHTTPError writes a plaintext error via net/http.Error, localized
// from the catalog. Use this for browser-facing endpoints (LTI launches,
// OAuth2 callbacks) where the client expects text/plain rather than JSON.
// For API endpoints, prefer LocalizedErrorResponse.
func (r *Responder) LocalizedHTTPError(ctx context.Context, w http.ResponseWriter, status int, messageID string, templateData map[string]any) {
	http.Error(w, appi18n.Localize(ctx, messageID, templateData), status)
}

// SanitizeError converts an error to a client-safe message.
// It logs the internal error and returns a generic message.
func (r *Responder) SanitizeError(err error, operation string) *SanitizedError {
	if err == nil {
		return nil
	}

	statusCode := http.StatusInternalServerError
	clientMessage := ErrMsgInternalError

	switch {
	case errors.Is(err, apperrors.ErrNotFound):
		statusCode = http.StatusNotFound
		clientMessage = ErrMsgNotFound

	case errors.Is(err, apperrors.ErrUnauthorized):
		statusCode = http.StatusUnauthorized
		clientMessage = ErrMsgUnauthorized

	case errors.Is(err, apperrors.ErrForbidden):
		statusCode = http.StatusForbidden
		clientMessage = ErrMsgForbidden

	case isUniqueViolation(err):
		statusCode = http.StatusConflict
		clientMessage = ErrMsgConflict

	case isValidationError(err):
		statusCode = http.StatusBadRequest
		clientMessage = ErrMsgValidationFailed

	case isDatabaseError(err):
		statusCode = http.StatusInternalServerError
		clientMessage = ErrMsgDatabaseError

	case errors.Is(err, apperrors.ErrConflict):
		statusCode = http.StatusConflict
		clientMessage = ErrMsgConflict

	case errors.Is(err, orchestrator.ErrNotImplemented):
		statusCode = http.StatusNotImplemented
		clientMessage = "this operation is not yet supported"
	}

	r.Logger.Error("Operation failed",
		slog.String("operation", operation),
		slog.String("error", err.Error()),
		slog.Int("status_code", statusCode),
	)

	return &SanitizedError{
		ClientMessage: clientMessage,
		InternalError: err,
		StatusCode:    statusCode,
	}
}

// SafeErrorResponse sends a sanitized error response to the client.
func (r *Responder) SafeErrorResponse(w http.ResponseWriter, err error, operation string) {
	sanitized := r.SanitizeError(err, operation)
	if sanitized == nil {
		return
	}
	r.ErrorResponse(w, sanitized.StatusCode, sanitized.ClientMessage)
}

// SafeErrorResponseWithMessage sends a specific client message (when you know it's safe)
// while logging the actual error internally.
func (r *Responder) SafeErrorResponseWithMessage(w http.ResponseWriter, statusCode int, clientMsg string, err error, operation string) {
	r.Logger.Error("Operation failed",
		slog.String("operation", operation),
		slog.String("error", err.Error()),
		slog.Int("status_code", statusCode),
		slog.String("client_message", clientMsg),
	)
	r.ErrorResponse(w, statusCode, clientMsg)
}

// RespondWithValidationError sends a validation error response.
func (r *Responder) RespondWithValidationError(w http.ResponseWriter, errs []ValidationError) {
	r.JSONResponse(w, http.StatusBadRequest, ValidationErrorResponse{
		Error:   "validation failed",
		Details: errs,
	})
}

// isUniqueViolation checks if the error is a database unique constraint violation.
func isUniqueViolation(err error) bool {
	var pgErr *pq.Error
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

// isDatabaseError checks if the error is a database-related error.
func isDatabaseError(err error) bool {
	var pgErr *pq.Error
	return errors.As(err, &pgErr)
}

// ErrValidation is the sentinel for validation errors.
var ErrValidation = errors.New("validation error")

// isValidationError checks if the error wraps the validation sentinel.
func isValidationError(err error) bool {
	return errors.Is(err, ErrValidation)
}
