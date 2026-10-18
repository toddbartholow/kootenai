package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// -----------------------------------------------------------------------------
// mimeResponseWriter Tests
// -----------------------------------------------------------------------------

func TestMimeResponseWriter_WriteHeader(t *testing.T) {
	t.Run("sets Content-Type when mime type is set", func(t *testing.T) {
		rr := httptest.NewRecorder()
		w := &mimeResponseWriter{
			ResponseWriter: rr,
			mimeType:       "text/css",
		}

		w.WriteHeader(http.StatusOK)

		contentType := rr.Header().Get("Content-Type")
		if contentType != "text/css" {
			t.Errorf("expected Content-Type 'text/css', got '%s'", contentType)
		}
		if !w.wroteHeader {
			t.Error("expected wroteHeader to be true")
		}
	})

	t.Run("does not set Content-Type when empty", func(t *testing.T) {
		rr := httptest.NewRecorder()
		w := &mimeResponseWriter{
			ResponseWriter: rr,
			mimeType:       "",
		}

		w.WriteHeader(http.StatusOK)

		contentType := rr.Header().Get("Content-Type")
		if contentType != "" {
			t.Errorf("expected empty Content-Type, got '%s'", contentType)
		}
	})

	t.Run("only sets header once", func(t *testing.T) {
		rr := httptest.NewRecorder()
		w := &mimeResponseWriter{
			ResponseWriter: rr,
			mimeType:       "text/css",
		}

		w.WriteHeader(http.StatusOK)
		// Change mime type after first WriteHeader
		w.mimeType = "text/html"
		w.WriteHeader(http.StatusOK)

		// Should still be the first value
		contentType := rr.Header().Get("Content-Type")
		if contentType != "text/css" {
			t.Errorf("expected Content-Type 'text/css', got '%s'", contentType)
		}
	})
}

func TestMimeResponseWriter_Write(t *testing.T) {
	t.Run("writes content and sets header implicitly", func(t *testing.T) {
		rr := httptest.NewRecorder()
		w := &mimeResponseWriter{
			ResponseWriter: rr,
			mimeType:       "application/javascript",
		}

		content := []byte("console.log('hello');")
		n, err := w.Write(content)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != len(content) {
			t.Errorf("expected to write %d bytes, wrote %d", len(content), n)
		}
		if rr.Body.String() != string(content) {
			t.Errorf("expected body '%s', got '%s'", string(content), rr.Body.String())
		}
		contentType := rr.Header().Get("Content-Type")
		if contentType != "application/javascript" {
			t.Errorf("expected Content-Type 'application/javascript', got '%s'", contentType)
		}
	})

	t.Run("uses existing header if already written", func(t *testing.T) {
		rr := httptest.NewRecorder()
		w := &mimeResponseWriter{
			ResponseWriter: rr,
			mimeType:       "text/css",
		}

		// Write header first
		w.WriteHeader(http.StatusNotFound)

		content := []byte("body{}")
		_, _ = w.Write(content)

		// Should have recorded 404, not 200
		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// MIME Types Tests
// -----------------------------------------------------------------------------

func TestMimeTypes(t *testing.T) {
	expectedTypes := map[string]string{
		".js":    "application/javascript",
		".mjs":   "application/javascript",
		".css":   "text/css",
		".html":  "text/html",
		".json":  "application/json",
		".png":   "image/png",
		".svg":   "image/svg+xml",
		".woff2": "font/woff2",
	}

	for ext, expected := range expectedTypes {
		t.Run(ext, func(t *testing.T) {
			actual := mimeTypes[ext]
			if actual != expected {
				t.Errorf("expected mime type for %s to be '%s', got '%s'", ext, expected, actual)
			}
		})
	}
}

func TestStaticHandler(t *testing.T) {
	t.Run("returns handler without panic", func(t *testing.T) {
		srv := newTestServer(t)
		handler := srv.StaticHandler()

		if handler == nil {
			t.Fatal("expected handler to be non-nil")
		}
	})
}
