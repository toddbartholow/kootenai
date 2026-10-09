package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecureHeaders(t *testing.T) {
	handler := SecureHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	expectedHeaders := map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"X-XSS-Protection":        "1; mode=block",
		"Referrer-Policy":         "strict-origin-when-cross-origin",
		"Content-Security-Policy": "default-src 'self'",
	}

	for header, expected := range expectedHeaders {
		if got := rr.Header().Get(header); got != expected {
			t.Errorf("expected %s header to be %s, got %s", header, expected, got)
		}
	}
}

func TestMaxBodySize(t *testing.T) {
	tests := []struct {
		name           string
		bodySize       int
		maxSize        int64
		expectedStatus int
	}{
		{
			name:           "body within limit",
			bodySize:       100,
			maxSize:        1024,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "body exceeds limit",
			bodySize:       2000,
			maxSize:        1024,
			expectedStatus: http.StatusRequestEntityTooLarge,
		},
		{
			name:           "body at exact limit",
			bodySize:       1024,
			maxSize:        1024,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := MaxBodySize(tt.maxSize)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			body := bytes.Repeat([]byte("a"), tt.bodySize)
			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
			req.ContentLength = int64(tt.bodySize)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}
		})
	}
}

func TestRequireJSON(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		contentType    string
		expectedStatus int
	}{
		{
			name:           "GET request - no content type required",
			method:         http.MethodGet,
			contentType:    "",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "POST with JSON content type",
			method:         http.MethodPost,
			contentType:    "application/json",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "POST with JSON charset content type",
			method:         http.MethodPost,
			contentType:    "application/json; charset=utf-8",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "POST with wrong content type",
			method:         http.MethodPost,
			contentType:    "text/plain",
			expectedStatus: http.StatusUnsupportedMediaType,
		},
		{
			name:           "POST with no content type",
			method:         http.MethodPost,
			contentType:    "",
			expectedStatus: http.StatusUnsupportedMediaType,
		},
		{
			name:           "DELETE - no content type required",
			method:         http.MethodDelete,
			contentType:    "",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := RequireJSON(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(tt.method, "/", nil)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}
		})
	}
}

func TestValidateJSON(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		contentType    string
		body           string
		expectedStatus int
	}{
		{
			name:           "GET request - skipped",
			method:         http.MethodGet,
			contentType:    "",
			body:           "",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "POST with valid JSON",
			method:         http.MethodPost,
			contentType:    "application/json",
			body:           `{"key": "value"}`,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "POST with invalid JSON",
			method:         http.MethodPost,
			contentType:    "application/json",
			body:           `{"key": }`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "POST with empty body",
			method:         http.MethodPost,
			contentType:    "application/json",
			body:           "",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "POST with non-JSON content type - skipped",
			method:         http.MethodPost,
			contentType:    "text/plain",
			body:           "not json",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := ValidateJSON(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			var body *strings.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			} else {
				body = strings.NewReader("")
			}

			req := httptest.NewRequest(tt.method, "/", body)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}
		})
	}
}

func TestSanitizeInput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal string",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "string with null bytes",
			input:    "hello\x00world",
			expected: "helloworld",
		},
		{
			name:     "string with leading/trailing whitespace",
			input:    "  hello  ",
			expected: "hello",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only whitespace",
			input:    "   ",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeInput(tt.input)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestValidateNonEmpty(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "non-empty string",
			input:    "hello",
			expected: true,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "only whitespace",
			input:    "   ",
			expected: false,
		},
		{
			name:     "only null bytes",
			input:    "\x00\x00",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateNonEmpty(tt.input)
			if got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestValidateMaxLength(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected bool
	}{
		{
			name:     "within limit",
			input:    "hello",
			maxLen:   10,
			expected: true,
		},
		{
			name:     "at limit",
			input:    "hello",
			maxLen:   5,
			expected: true,
		},
		{
			name:     "exceeds limit",
			input:    "hello world",
			maxLen:   5,
			expected: false,
		},
		{
			name:     "empty string",
			input:    "",
			maxLen:   5,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateMaxLength(tt.input, tt.maxLen)
			if got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestValidateMinLength(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		minLen   int
		expected bool
	}{
		{
			name:     "exceeds min",
			input:    "hello",
			minLen:   3,
			expected: true,
		},
		{
			name:     "at min",
			input:    "hello",
			minLen:   5,
			expected: true,
		},
		{
			name:     "below min",
			input:    "hi",
			minLen:   5,
			expected: false,
		},
		{
			name:     "empty string with zero min",
			input:    "",
			minLen:   0,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateMinLength(tt.input, tt.minLen)
			if got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestValidateRequestBody(t *testing.T) {
	cfg := ValidationConfig{
		MaxBodySize:    1024,
		RequireJSON:    true,
		AllowEmptyBody: true,
	}

	handler := ValidateRequestBody(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("GET request bypasses validation", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
	})

	t.Run("POST with valid JSON", func(t *testing.T) {
		body := strings.NewReader(`{"test": true}`)
		req := httptest.NewRequest(http.MethodPost, "/", body)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
	})

	t.Run("POST without JSON content type when required", func(t *testing.T) {
		body := strings.NewReader(`{"test": true}`)
		req := httptest.NewRequest(http.MethodPost, "/", body)
		req.Header.Set("Content-Type", "text/plain")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnsupportedMediaType {
			t.Errorf("expected status %d, got %d", http.StatusUnsupportedMediaType, rr.Code)
		}
	})
}
