package server

import (
	"errors"
	"net/http"
	"testing"
)

// -----------------------------------------------------------------------------
// SanitizedError Tests
// -----------------------------------------------------------------------------

func TestSanitizedError_Error(t *testing.T) {
	se := &SanitizedError{
		ClientMessage: "client message",
		InternalError: errors.New("internal error"),
		StatusCode:    http.StatusBadRequest,
	}

	if se.Error() != "client message" {
		t.Errorf("Error() = %s, want 'client message'", se.Error())
	}
}

func TestSanitizedError_Unwrap(t *testing.T) {
	internalErr := errors.New("internal error")
	se := &SanitizedError{
		ClientMessage: "client message",
		InternalError: internalErr,
		StatusCode:    http.StatusBadRequest,
	}

	if se.Unwrap() != internalErr {
		t.Error("Unwrap() should return internal error")
	}
}

// -----------------------------------------------------------------------------
// Error Type Detection Tests
// -----------------------------------------------------------------------------
//
// TestIsUniqueViolation and TestIsDatabaseError were removed when their
// production counterparts moved to server/httputil as package-private
// helpers. The httputil package has its own tests covering the same
// detection logic; duplicating them here with an import cycle through the
// server package offered no additional coverage.

// -----------------------------------------------------------------------------
// DTO Helper Tests
// -----------------------------------------------------------------------------

func TestNewStatusResponse(t *testing.T) {
	resp := NewStatusResponse("ok")
	if resp.Status != "ok" {
		t.Errorf("Status = %s, want 'ok'", resp.Status)
	}
}

func TestNewMessageResponse(t *testing.T) {
	resp := NewMessageResponse("hello world")
	if resp.Message != "hello world" {
		t.Errorf("Message = %s, want 'hello world'", resp.Message)
	}
}

func TestNewErrorResponse(t *testing.T) {
	resp := NewErrorResponse("something went wrong")
	if resp.Error != "something went wrong" {
		t.Errorf("Error = %s, want 'something went wrong'", resp.Error)
	}
}

func TestNewPaginationMeta(t *testing.T) {
	tests := []struct {
		name            string
		total           int
		limit           int
		offset          int
		expectedHasMore bool
		expectedPages   int
		expectedPage    int
	}{
		{
			name:            "first page with more",
			total:           100,
			limit:           10,
			offset:          0,
			expectedHasMore: true,
			expectedPages:   10,
			expectedPage:    1,
		},
		{
			name:            "middle page",
			total:           100,
			limit:           10,
			offset:          50,
			expectedHasMore: true,
			expectedPages:   10,
			expectedPage:    6,
		},
		{
			name:            "last page",
			total:           100,
			limit:           10,
			offset:          90,
			expectedHasMore: false,
			expectedPages:   10,
			expectedPage:    10,
		},
		{
			name:            "zero limit",
			total:           100,
			limit:           0,
			offset:          0,
			expectedHasMore: true, // 0 + 0 < 100 = true
			expectedPages:   0,
			expectedPage:    0,
		},
		{
			name:            "total less than limit",
			total:           5,
			limit:           10,
			offset:          0,
			expectedHasMore: false,
			expectedPages:   1,
			expectedPage:    1,
		},
		{
			name:            "partial page",
			total:           25,
			limit:           10,
			offset:          20,
			expectedHasMore: false,
			expectedPages:   3,
			expectedPage:    3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := NewPaginationMeta(tt.total, tt.limit, tt.offset)

			if meta.Total != tt.total {
				t.Errorf("Total = %d, want %d", meta.Total, tt.total)
			}
			if meta.Limit != tt.limit {
				t.Errorf("Limit = %d, want %d", meta.Limit, tt.limit)
			}
			if meta.Offset != tt.offset {
				t.Errorf("Offset = %d, want %d", meta.Offset, tt.offset)
			}
			if meta.HasMore != tt.expectedHasMore {
				t.Errorf("HasMore = %v, want %v", meta.HasMore, tt.expectedHasMore)
			}
			if meta.TotalPages != tt.expectedPages {
				t.Errorf("TotalPages = %d, want %d", meta.TotalPages, tt.expectedPages)
			}
			if meta.Page != tt.expectedPage {
				t.Errorf("Page = %d, want %d", meta.Page, tt.expectedPage)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Sentinel Errors Tests
// -----------------------------------------------------------------------------

func TestSentinelErrors(t *testing.T) {
	// Test that sentinel errors exist and can be used with errors.Is
	tests := []struct {
		name string
		err  error
	}{
		{"ErrNotFound", ErrNotFound},
		{"ErrUnauthorized", ErrUnauthorized},
		{"ErrForbidden", ErrForbidden},
		{"ErrConflict", ErrConflict},
		{"ErrValidation", ErrValidation},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Errorf("%s should not be nil", tt.name)
			}
			if tt.err.Error() == "" {
				t.Errorf("%s.Error() should not be empty", tt.name)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Error Message Constants Tests
// -----------------------------------------------------------------------------

func TestErrorMessageConstants(t *testing.T) {
	constants := []struct {
		name  string
		value string
	}{
		{"ErrMsgInternalError", ErrMsgInternalError},
		{"ErrMsgDatabaseError", ErrMsgDatabaseError},
		{"ErrMsgNotFound", ErrMsgNotFound},
		{"ErrMsgUnauthorized", ErrMsgUnauthorized},
		{"ErrMsgForbidden", ErrMsgForbidden},
		{"ErrMsgBadRequest", ErrMsgBadRequest},
		{"ErrMsgConflict", ErrMsgConflict},
		{"ErrMsgValidationFailed", ErrMsgValidationFailed},
	}

	for _, c := range constants {
		if c.value == "" {
			t.Errorf("%s should not be empty", c.name)
		}
	}
}
