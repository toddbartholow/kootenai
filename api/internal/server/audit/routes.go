package audit

import (
	"github.com/go-chi/chi/v5"

	custommiddleware "github.com/toddbartholow/kootenai/api/internal/middleware"
)

// SetupRoutes configures audit log API routes
func SetupRoutes(r chi.Router, svc *Service, rw ResponseWriter) {
	r.Route("/audit", func(r chi.Router) {
		// All audit routes require admin role
		r.Use(custommiddleware.RequireOrgAdmin)

		r.Get("/", svc.HandleList(rw))
		r.Post("/export", svc.HandleExport(rw))
		r.Get("/{entryID}", svc.HandleGet(rw))
	})
}
