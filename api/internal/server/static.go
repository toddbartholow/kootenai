// Package server provides static file serving
package server

import (
	"embed"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
)

//go:embed static/*
var staticFiles embed.FS

// mimeTypes maps file extensions to MIME types
var mimeTypes = map[string]string{
	".js":    "application/javascript",
	".mjs":   "application/javascript",
	".css":   "text/css",
	".html":  "text/html",
	".json":  "application/json",
	".png":   "image/png",
	".jpg":   "image/jpeg",
	".jpeg":  "image/jpeg",
	".gif":   "image/gif",
	".svg":   "image/svg+xml",
	".woff":  "font/woff",
	".woff2": "font/woff2",
	".ttf":   "font/ttf",
}

// mimeResponseWriter wraps http.ResponseWriter to override Content-Type
type mimeResponseWriter struct {
	http.ResponseWriter
	mimeType    string
	wroteHeader bool
}

func (w *mimeResponseWriter) WriteHeader(code int) {
	if !w.wroteHeader && w.mimeType != "" {
		w.Header().Set("Content-Type", w.mimeType)
	}
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *mimeResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// StaticHandler returns an http.Handler that serves static files from the embedded filesystem
func (s *Server) StaticHandler() http.Handler {
	// Get the static subdirectory from the embedded filesystem
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		s.logger.Error("Failed to create static filesystem", "error", err)
		return http.NotFoundHandler()
	}

	fileServer := http.FileServer(http.FS(staticFS))

	// Wrap with MIME type handler
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Determine correct Content-Type based on file extension
		ext := strings.ToLower(filepath.Ext(r.URL.Path))
		mimeType := mimeTypes[ext]

		// Use wrapper to ensure our MIME type is set
		wrappedWriter := &mimeResponseWriter{
			ResponseWriter: w,
			mimeType:       mimeType,
		}
		fileServer.ServeHTTP(wrappedWriter, r)
	})
}
