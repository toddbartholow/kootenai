// Package pagination provides cursor-based pagination utilities for API endpoints.
package pagination

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// CursorType defines the type of cursor value being encoded
type CursorType string

const (
	CursorTypeTime CursorType = "time"
	CursorTypeID   CursorType = "id"
)

// Cursor represents an opaque cursor for pagination
type Cursor struct {
	Type      CursorType `json:"t"`
	Value     any        `json:"v"`
	Direction string     `json:"d"` // "next" or "prev"
}

// EncodeCursor encodes a cursor value to a base64 string
func EncodeCursor(cursorType CursorType, value any, direction string) string {
	c := Cursor{
		Type:      cursorType,
		Value:     value,
		Direction: direction,
	}
	data, _ := json.Marshal(c)
	return base64.URLEncoding.EncodeToString(data)
}

// DecodeCursor decodes a base64 cursor string
func DecodeCursor(encoded string) (*Cursor, error) {
	if encoded == "" {
		return nil, nil
	}
	data, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor encoding: %w", err)
	}

	var c Cursor
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("invalid cursor format: %w", err)
	}

	return &c, nil
}

// GetTimeValue extracts a time.Time from the cursor value
func (c *Cursor) GetTimeValue() (time.Time, error) {
	if c.Type != CursorTypeTime {
		return time.Time{}, fmt.Errorf("cursor is not a time cursor")
	}

	switch v := c.Value.(type) {
	case string:
		return time.Parse(time.RFC3339Nano, v)
	case float64:
		// JSON unmarshals numbers as float64
		return time.Unix(int64(v), 0), nil
	default:
		return time.Time{}, fmt.Errorf("invalid time cursor value type")
	}
}

// GetIDValue extracts a string ID from the cursor value
func (c *Cursor) GetIDValue() (string, error) {
	if c.Type != CursorTypeID {
		return "", fmt.Errorf("cursor is not an ID cursor")
	}

	switch v := c.Value.(type) {
	case string:
		return v, nil
	default:
		return "", fmt.Errorf("invalid ID cursor value type")
	}
}

// PageInfo contains pagination metadata
type PageInfo struct {
	HasNextPage     bool   `json:"hasNextPage"`
	HasPreviousPage bool   `json:"hasPreviousPage"`
	StartCursor     string `json:"startCursor,omitempty"`
	EndCursor       string `json:"endCursor,omitempty"`
	TotalCount      int    `json:"totalCount,omitempty"`
}

// Connection wraps a collection with pagination metadata (Relay-style)
type Connection[T any] struct {
	Edges    []Edge[T] `json:"edges"`
	PageInfo PageInfo  `json:"pageInfo"`
}

// Edge represents a single item in a connection with its cursor
type Edge[T any] struct {
	Node   T      `json:"node"`
	Cursor string `json:"cursor"`
}

// PaginationParams holds common pagination parameters
type PaginationParams struct {
	First  int    // Number of items to return (forward pagination)
	After  string // Cursor for forward pagination
	Last   int    // Number of items to return (backward pagination)
	Before string // Cursor for backward pagination
}

// DefaultPageSize is the default number of items per page
const DefaultPageSize = 20

// MaxPageSize is the maximum allowed page size
const MaxPageSize = 100

// ParsePaginationParams extracts pagination parameters from an HTTP request
func ParsePaginationParams(r *http.Request) PaginationParams {
	q := r.URL.Query()

	params := PaginationParams{
		After:  q.Get("after"),
		Before: q.Get("before"),
	}

	if first := q.Get("first"); first != "" {
		if n, err := strconv.Atoi(first); err == nil && n > 0 {
			params.First = n
			if params.First > MaxPageSize {
				params.First = MaxPageSize
			}
		}
	}

	if last := q.Get("last"); last != "" {
		if n, err := strconv.Atoi(last); err == nil && n > 0 {
			params.Last = n
			if params.Last > MaxPageSize {
				params.Last = MaxPageSize
			}
		}
	}

	// Default to forward pagination if neither specified
	if params.First == 0 && params.Last == 0 {
		params.First = DefaultPageSize
	}

	return params
}

// GetLimit returns the effective limit based on pagination direction
func (p PaginationParams) GetLimit() int {
	if p.Last > 0 {
		return p.Last
	}
	if p.First > 0 {
		return p.First
	}
	return DefaultPageSize
}

// IsBackward returns true if pagination is backward
func (p PaginationParams) IsBackward() bool {
	return p.Last > 0 || p.Before != ""
}

// BuildPageLinks creates Link header values for pagination
func BuildPageLinks(baseURL string, pageInfo PageInfo, currentParams url.Values) string {
	var links []string

	if pageInfo.HasNextPage && pageInfo.EndCursor != "" {
		nextParams := copyParams(currentParams)
		nextParams.Set("after", pageInfo.EndCursor)
		nextParams.Del("before")
		links = append(links, fmt.Sprintf(`<%s?%s>; rel="next"`, baseURL, nextParams.Encode()))
	}

	if pageInfo.HasPreviousPage && pageInfo.StartCursor != "" {
		prevParams := copyParams(currentParams)
		prevParams.Set("before", pageInfo.StartCursor)
		prevParams.Del("after")
		links = append(links, fmt.Sprintf(`<%s?%s>; rel="prev"`, baseURL, prevParams.Encode()))
	}

	return joinLinks(links)
}

func copyParams(params url.Values) url.Values {
	result := make(url.Values)
	for k, v := range params {
		result[k] = v
	}
	return result
}

func joinLinks(links []string) string {
	result := ""
	for i, link := range links {
		if i > 0 {
			result += ", "
		}
		result += link
	}
	return result
}

// OffsetParams holds traditional offset-based pagination (for backwards compatibility)
type OffsetParams struct {
	Limit  int
	Offset int
}

// ParseOffsetParams extracts offset pagination parameters from an HTTP request
func ParseOffsetParams(r *http.Request) OffsetParams {
	q := r.URL.Query()

	params := OffsetParams{
		Limit:  DefaultPageSize,
		Offset: 0,
	}

	if limit := q.Get("limit"); limit != "" {
		if n, err := strconv.Atoi(limit); err == nil && n > 0 {
			params.Limit = n
			if params.Limit > MaxPageSize {
				params.Limit = MaxPageSize
			}
		}
	}

	if offset := q.Get("offset"); offset != "" {
		if n, err := strconv.Atoi(offset); err == nil && n >= 0 {
			params.Offset = n
		}
	}

	return params
}

// PaginatedResponse wraps items with pagination metadata (offset-style)
type PaginatedResponse[T any] struct {
	Items      []T      `json:"items"`
	Pagination Metadata `json:"pagination"`
}

// Metadata contains pagination metadata for offset-based pagination
type Metadata struct {
	Total      int  `json:"total"`
	Limit      int  `json:"limit"`
	Offset     int  `json:"offset"`
	HasMore    bool `json:"hasMore"`
	TotalPages int  `json:"totalPages,omitempty"`
	Page       int  `json:"page,omitempty"`
}

// NewPaginatedResponse creates a paginated response from items and total count
func NewPaginatedResponse[T any](items []T, total, limit, offset int) PaginatedResponse[T] {
	hasMore := offset+len(items) < total
	totalPages := (total + limit - 1) / limit
	page := (offset / limit) + 1

	return PaginatedResponse[T]{
		Items: items,
		Pagination: Metadata{
			Total:      total,
			Limit:      limit,
			Offset:     offset,
			HasMore:    hasMore,
			TotalPages: totalPages,
			Page:       page,
		},
	}
}
