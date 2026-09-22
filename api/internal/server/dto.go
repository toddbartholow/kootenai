// Package server provides DTOs (Data Transfer Objects) for API responses.
// These types provide type safety and consistent API response structures.
//
// Common response types (StatusResponse, MessageResponse, PaginationMeta, etc.)
// are defined in serverutil and re-exported here as type aliases for backward
// compatibility. New sub-packages should import serverutil directly.
package server

import (
	"github.com/toddbartholow/kootenai/api/internal/server/pods"
	"github.com/toddbartholow/kootenai/api/internal/server/serverutil"
	"github.com/toddbartholow/kootenai/api/internal/server/sessions"
)

// -----------------------------------------------------------------------------
// Common Response Types (aliases — canonical definitions in serverutil)
// -----------------------------------------------------------------------------

type StatusResponse = serverutil.StatusResponse
type MessageResponse = serverutil.MessageResponse
type PaginationMeta = serverutil.PaginationMeta

// -----------------------------------------------------------------------------
// Pod DTOs (aliases — canonical definitions in pods sub-package)
// -----------------------------------------------------------------------------

type ListPodsResponse = pods.ListPodsResponse
type ListSnapshotsResponse = pods.ListSnapshotsResponse
type VMActionResponse = pods.VMActionResponse

// -----------------------------------------------------------------------------
// Session DTOs (aliases — canonical definitions in sessions sub-package)
// -----------------------------------------------------------------------------

type ListSessionsResponse = sessions.ListSessionsResponse
type SessionResponse = sessions.SessionResponse
type CreateSessionResponse = sessions.CreateSessionResponse
type SessionSubmitResponse = sessions.SessionSubmitResponse
type SessionStartedResponse = sessions.SessionStartedResponse
type SessionProgressResponse = sessions.SessionProgressResponse
type CheckpointProgressDTO = sessions.CheckpointProgressDTO
type SessionSubmitDetailResponse = sessions.SessionSubmitDetailResponse
type CheckpointsResponse = sessions.CheckpointsResponse
type GradeResponse = sessions.GradeResponse

// Lab template DTOs live in internal/server/labs, next to the swagger
// annotations that reference them.

// -----------------------------------------------------------------------------
// Auth DTOs (referenced by swagger annotations)
// -----------------------------------------------------------------------------

// LogoutResponse represents a logout response
type LogoutResponse struct {
	Message string `json:"message"`
}

// -----------------------------------------------------------------------------
// Monitoring/Health DTOs (used by server health check handlers)
// -----------------------------------------------------------------------------

// HealthCheck represents the result of a single dependency health check
type HealthCheck struct {
	Status    string `json:"status"`              // healthy, unhealthy, not_configured
	LatencyMs int64  `json:"latencyMs,omitempty"` // Check latency in milliseconds
	Error     string `json:"error,omitempty"`     // Error message if unhealthy
	// Additional fields for circuit breaker status
	CircuitState string `json:"circuitState,omitempty"` // closed, half-open, open
	Failures     int    `json:"failures,omitempty"`     // Failure count
	LastFailure  string `json:"lastFailure,omitempty"`  // Last failure timestamp
}

// HealthResponse represents the enhanced health check response
type HealthResponse struct {
	Status  string                  `json:"status"`            // healthy, degraded, unhealthy
	Version string                  `json:"version,omitempty"` // API version
	Uptime  string                  `json:"uptime,omitempty"`  // Server uptime
	Checks  map[string]*HealthCheck `json:"checks,omitempty"`  // Individual dependency checks
}

// -----------------------------------------------------------------------------
// Helper Functions (forwarding to serverutil)
// -----------------------------------------------------------------------------

var (
	NewStatusResponse  = serverutil.NewStatusResponse
	NewMessageResponse = serverutil.NewMessageResponse
	NewErrorResponse   = serverutil.NewErrorResponse
	NewPaginationMeta  = serverutil.NewPaginationMeta
	ParsePagination    = serverutil.ParsePagination
)

// PaginationParams is an alias for serverutil.PaginationParams.
type PaginationParams = serverutil.PaginationParams
