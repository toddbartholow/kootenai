// Package serverutil provides shared types and helpers used across server
// sub-packages. Extracting them here breaks the dependency on the monolithic
// server package, allowing sub-packages to import these types directly.
package serverutil

import (
	"net/http"
	"strconv"
)

// -----------------------------------------------------------------------------
// Common Response Types
// -----------------------------------------------------------------------------

// StatusResponse represents a simple status message response.
type StatusResponse struct {
	Status string `json:"status"`
}

// MessageResponse represents a simple message response.
type MessageResponse struct {
	Message string `json:"message"`
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error string `json:"error" example:"invalid credentials"`
}

// PaginationMeta contains pagination metadata.
type PaginationMeta struct {
	Total      int  `json:"total"`
	Limit      int  `json:"limit"`
	Offset     int  `json:"offset"`
	HasMore    bool `json:"hasMore"`
	TotalPages int  `json:"totalPages,omitempty"`
	Page       int  `json:"page,omitempty"`
}

// PaginationParams holds parsed pagination parameters.
type PaginationParams struct {
	Limit  int
	Offset int
}

// -----------------------------------------------------------------------------
// Constructors
// -----------------------------------------------------------------------------

// NewStatusResponse creates a new status response.
func NewStatusResponse(status string) StatusResponse {
	return StatusResponse{Status: status}
}

// NewMessageResponse creates a new message response.
func NewMessageResponse(message string) MessageResponse {
	return MessageResponse{Message: message}
}

// NewErrorResponse creates a new error response.
func NewErrorResponse(err string) ErrorResponse {
	return ErrorResponse{Error: err}
}

// NewPaginationMeta creates pagination metadata from count values.
func NewPaginationMeta(total, limit, offset int) PaginationMeta {
	hasMore := offset+limit < total
	totalPages := 0
	page := 0
	if limit > 0 {
		totalPages = (total + limit - 1) / limit
		page = (offset / limit) + 1
	}

	return PaginationMeta{
		Total:      total,
		Limit:      limit,
		Offset:     offset,
		HasMore:    hasMore,
		TotalPages: totalPages,
		Page:       page,
	}
}

// DefaultPaginationParams returns default pagination params.
func DefaultPaginationParams() PaginationParams {
	return PaginationParams{Limit: 50, Offset: 0}
}

// ParsePagination extracts limit and offset from query parameters.
// Returns default values if not specified. Limits are capped at maxLimit (default 500).
func ParsePagination(r *http.Request, defaultLimit, maxLimit int) PaginationParams {
	if defaultLimit <= 0 {
		defaultLimit = 50
	}
	if maxLimit <= 0 {
		maxLimit = 500
	}

	params := PaginationParams{
		Limit:  defaultLimit,
		Offset: 0,
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			if limit > maxLimit {
				limit = maxLimit
			}
			params.Limit = limit
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			params.Offset = offset
		}
	}

	return params
}
