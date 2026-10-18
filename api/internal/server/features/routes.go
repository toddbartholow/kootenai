package features

import (
	"github.com/go-chi/chi/v5"
)

// SetupGlobalRoutes configures global feature list routes (outside org context)
func SetupGlobalRoutes(r chi.Router, svc *Service, rw ResponseWriter) {
	// Global features list (public, for informational purposes)
	r.Get("/features", svc.HandleListFeatures(rw))
}

// SetupOrgRoutes configures organization-scoped feature routes
// This should be called within an organization route group that has tenant middleware applied
func SetupOrgRoutes(r chi.Router, svc *Service, rw ResponseWriter) {
	r.Get("/features", svc.HandleGetOrganizationFeatures(rw))
	r.Post("/features/{featureID}", svc.HandleSetOrganizationFeature(rw))
	r.Delete("/features/{featureID}", svc.HandleRemoveOrganizationFeature(rw))
}
