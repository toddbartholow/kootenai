package httputil

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/toddbartholow/kootenai/api/internal/apperrors"
)

func newTestResponder() *Responder {
	return NewResponder(slog.Default())
}

func TestJSONResponse(t *testing.T) {
	r := newTestResponder()
	rr := httptest.NewRecorder()

	data := map[string]string{"key": "value"}
	r.JSONResponse(rr, http.StatusOK, data)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}

	var result map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("expected key=value, got %s", result["key"])
	}
}

func TestErrorResponse(t *testing.T) {
	r := newTestResponder()
	rr := httptest.NewRecorder()

	r.ErrorResponse(rr, http.StatusBadRequest, "bad input")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	var result map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result["error"] != "bad input" {
		t.Errorf("expected error=bad input, got %s", result["error"])
	}
}

func TestSanitizeError(t *testing.T) {
	r := newTestResponder()

	tests := []struct {
		name           string
		err            error
		expectedStatus int
		expectedMsg    string
	}{
		{"nil error", nil, 0, ""},
		{"not found", fmt.Errorf("wrap: %w", apperrors.ErrNotFound), http.StatusNotFound, ErrMsgNotFound},
		{"forbidden", fmt.Errorf("wrap: %w", apperrors.ErrForbidden), http.StatusForbidden, ErrMsgForbidden},
		{"unauthorized", fmt.Errorf("wrap: %w", apperrors.ErrUnauthorized), http.StatusUnauthorized, ErrMsgUnauthorized},
		{"conflict", fmt.Errorf("wrap: %w", apperrors.ErrConflict), http.StatusConflict, ErrMsgConflict},
		{"generic error", errors.New("something broke"), http.StatusInternalServerError, ErrMsgInternalError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.SanitizeError(tt.err, "test-op")
			if tt.err == nil {
				if result != nil {
					t.Error("expected nil result for nil error")
				}
				return
			}
			if result.StatusCode != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, result.StatusCode)
			}
			if result.ClientMessage != tt.expectedMsg {
				t.Errorf("expected message %q, got %q", tt.expectedMsg, result.ClientMessage)
			}
		})
	}
}

func TestSafeErrorResponse(t *testing.T) {
	r := newTestResponder()
	rr := httptest.NewRecorder()

	r.SafeErrorResponse(rr, fmt.Errorf("wrap: %w", apperrors.ErrNotFound), "test-op")

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}

	var result map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result["error"] != ErrMsgNotFound {
		t.Errorf("expected error=%s, got %s", ErrMsgNotFound, result["error"])
	}
}

func TestRespondWithValidationError(t *testing.T) {
	r := newTestResponder()
	rr := httptest.NewRecorder()

	errs := []ValidationError{
		{Field: "name", Message: "name is required"},
		{Field: "email", Message: "invalid email format"},
	}
	r.RespondWithValidationError(rr, errs)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	var result ValidationErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result.Error != "validation failed" {
		t.Errorf("expected error=validation failed, got %s", result.Error)
	}
	if len(result.Details) != 2 {
		t.Errorf("expected 2 validation errors, got %d", len(result.Details))
	}
}

func TestNewResponder_NilLogger(t *testing.T) {
	r := NewResponder(nil)
	if r.Logger == nil {
		t.Error("expected default logger when nil is passed")
	}
}
