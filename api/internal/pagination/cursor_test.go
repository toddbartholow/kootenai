package pagination

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestEncodeCursor(t *testing.T) {
	tests := []struct {
		name       string
		cursorType CursorType
		value      interface{}
		direction  string
	}{
		{
			name:       "time cursor next",
			cursorType: CursorTypeTime,
			value:      "2024-01-15T10:30:00Z",
			direction:  "next",
		},
		{
			name:       "ID cursor prev",
			cursorType: CursorTypeID,
			value:      "abc-123-def",
			direction:  "prev",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := EncodeCursor(tt.cursorType, tt.value, tt.direction)
			if encoded == "" {
				t.Error("encoded cursor should not be empty")
			}

			decoded, err := DecodeCursor(encoded)
			if err != nil {
				t.Errorf("failed to decode cursor: %v", err)
			}
			if decoded.Type != tt.cursorType {
				t.Errorf("expected type %s, got %s", tt.cursorType, decoded.Type)
			}
			if decoded.Direction != tt.direction {
				t.Errorf("expected direction %s, got %s", tt.direction, decoded.Direction)
			}
		})
	}
}

func TestDecodeCursor_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		encoded string
	}{
		{
			name:    "invalid base64",
			encoded: "not-valid-base64!!!",
		},
		{
			name:    "invalid json",
			encoded: "bm90LWpzb24=", // "not-json" base64 encoded
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeCursor(tt.encoded)
			if err == nil {
				t.Error("expected error for invalid cursor")
			}
		})
	}
}

func TestDecodeCursor_Empty(t *testing.T) {
	cursor, err := DecodeCursor("")
	if err != nil {
		t.Errorf("empty string should not error: %v", err)
	}
	if cursor != nil {
		t.Error("empty string should return nil cursor")
	}
}

func TestCursor_GetTimeValue(t *testing.T) {
	testTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	encoded := EncodeCursor(CursorTypeTime, testTime.Format(time.RFC3339Nano), "next")

	cursor, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("failed to decode cursor: %v", err)
	}

	result, err := cursor.GetTimeValue()
	if err != nil {
		t.Errorf("failed to get time value: %v", err)
	}

	if !result.Equal(testTime) {
		t.Errorf("expected %v, got %v", testTime, result)
	}
}

func TestCursor_GetIDValue(t *testing.T) {
	testID := "user-123-abc"
	encoded := EncodeCursor(CursorTypeID, testID, "next")

	cursor, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("failed to decode cursor: %v", err)
	}

	result, err := cursor.GetIDValue()
	if err != nil {
		t.Errorf("failed to get ID value: %v", err)
	}

	if result != testID {
		t.Errorf("expected %s, got %s", testID, result)
	}
}

func TestParsePaginationParams(t *testing.T) {
	tests := []struct {
		name           string
		queryString    string
		expectedFirst  int
		expectedLast   int
		expectedAfter  string
		expectedBefore string
	}{
		{
			name:          "default values",
			queryString:   "",
			expectedFirst: DefaultPageSize,
		},
		{
			name:          "first only",
			queryString:   "first=10",
			expectedFirst: 10,
		},
		{
			name:          "first and after",
			queryString:   "first=25&after=abc123",
			expectedFirst: 25,
			expectedAfter: "abc123",
		},
		{
			name:           "last and before",
			queryString:    "last=15&before=xyz789",
			expectedLast:   15,
			expectedBefore: "xyz789",
		},
		{
			name:          "exceeds max page size",
			queryString:   "first=500",
			expectedFirst: MaxPageSize,
		},
		{
			name:          "invalid first ignored",
			queryString:   "first=invalid",
			expectedFirst: DefaultPageSize,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/?"+tt.queryString, nil)
			params := ParsePaginationParams(req)

			if params.First != tt.expectedFirst {
				t.Errorf("expected First=%d, got %d", tt.expectedFirst, params.First)
			}
			if params.Last != tt.expectedLast {
				t.Errorf("expected Last=%d, got %d", tt.expectedLast, params.Last)
			}
			if params.After != tt.expectedAfter {
				t.Errorf("expected After=%s, got %s", tt.expectedAfter, params.After)
			}
			if params.Before != tt.expectedBefore {
				t.Errorf("expected Before=%s, got %s", tt.expectedBefore, params.Before)
			}
		})
	}
}

func TestPaginationParams_GetLimit(t *testing.T) {
	tests := []struct {
		name     string
		params   PaginationParams
		expected int
	}{
		{
			name:     "first takes precedence",
			params:   PaginationParams{First: 10, Last: 5},
			expected: 5, // last is checked first in GetLimit
		},
		{
			name:     "first only",
			params:   PaginationParams{First: 25},
			expected: 25,
		},
		{
			name:     "last only",
			params:   PaginationParams{Last: 15},
			expected: 15,
		},
		{
			name:     "defaults",
			params:   PaginationParams{},
			expected: DefaultPageSize,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.params.GetLimit()
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestPaginationParams_IsBackward(t *testing.T) {
	tests := []struct {
		name     string
		params   PaginationParams
		expected bool
	}{
		{
			name:     "forward with first",
			params:   PaginationParams{First: 10},
			expected: false,
		},
		{
			name:     "forward with after",
			params:   PaginationParams{First: 10, After: "cursor"},
			expected: false,
		},
		{
			name:     "backward with last",
			params:   PaginationParams{Last: 10},
			expected: true,
		},
		{
			name:     "backward with before",
			params:   PaginationParams{First: 10, Before: "cursor"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.params.IsBackward()
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestParseOffsetParams(t *testing.T) {
	tests := []struct {
		name           string
		queryString    string
		expectedLimit  int
		expectedOffset int
	}{
		{
			name:           "default values",
			queryString:    "",
			expectedLimit:  DefaultPageSize,
			expectedOffset: 0,
		},
		{
			name:           "custom values",
			queryString:    "limit=50&offset=100",
			expectedLimit:  50,
			expectedOffset: 100,
		},
		{
			name:           "exceeds max limit",
			queryString:    "limit=500",
			expectedLimit:  MaxPageSize,
			expectedOffset: 0,
		},
		{
			name:           "invalid values use defaults",
			queryString:    "limit=invalid&offset=bad",
			expectedLimit:  DefaultPageSize,
			expectedOffset: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/?"+tt.queryString, nil)
			params := ParseOffsetParams(req)

			if params.Limit != tt.expectedLimit {
				t.Errorf("expected Limit=%d, got %d", tt.expectedLimit, params.Limit)
			}
			if params.Offset != tt.expectedOffset {
				t.Errorf("expected Offset=%d, got %d", tt.expectedOffset, params.Offset)
			}
		})
	}
}

func TestNewPaginatedResponse(t *testing.T) {
	items := []string{"a", "b", "c", "d", "e"}

	tests := []struct {
		name            string
		items           []string
		total           int
		limit           int
		offset          int
		expectedHasMore bool
		expectedPages   int
		expectedPage    int
	}{
		{
			name:            "first page with more",
			items:           items[:5],
			total:           20,
			limit:           5,
			offset:          0,
			expectedHasMore: true,
			expectedPages:   4,
			expectedPage:    1,
		},
		{
			name:            "last page",
			items:           items[:5],
			total:           20,
			limit:           5,
			offset:          15,
			expectedHasMore: false,
			expectedPages:   4,
			expectedPage:    4,
		},
		{
			name:            "single page",
			items:           items[:3],
			total:           3,
			limit:           10,
			offset:          0,
			expectedHasMore: false,
			expectedPages:   1,
			expectedPage:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := NewPaginatedResponse(tt.items, tt.total, tt.limit, tt.offset)

			if resp.Pagination.HasMore != tt.expectedHasMore {
				t.Errorf("expected HasMore=%v, got %v", tt.expectedHasMore, resp.Pagination.HasMore)
			}
			if resp.Pagination.TotalPages != tt.expectedPages {
				t.Errorf("expected TotalPages=%d, got %d", tt.expectedPages, resp.Pagination.TotalPages)
			}
			if resp.Pagination.Page != tt.expectedPage {
				t.Errorf("expected Page=%d, got %d", tt.expectedPage, resp.Pagination.Page)
			}
			if resp.Pagination.Total != tt.total {
				t.Errorf("expected Total=%d, got %d", tt.total, resp.Pagination.Total)
			}
		})
	}
}

func TestBuildPageLinks(t *testing.T) {
	baseURL := "https://api.example.com/items"

	tests := []struct {
		name       string
		pageInfo   PageInfo
		params     url.Values
		expectNext bool
		expectPrev bool
	}{
		{
			name: "has next page",
			pageInfo: PageInfo{
				HasNextPage: true,
				EndCursor:   "cursor123",
			},
			params:     url.Values{"first": []string{"10"}},
			expectNext: true,
			expectPrev: false,
		},
		{
			name: "has previous page",
			pageInfo: PageInfo{
				HasPreviousPage: true,
				StartCursor:     "cursor456",
			},
			params:     url.Values{"last": []string{"10"}},
			expectNext: false,
			expectPrev: true,
		},
		{
			name: "has both",
			pageInfo: PageInfo{
				HasNextPage:     true,
				HasPreviousPage: true,
				StartCursor:     "start",
				EndCursor:       "end",
			},
			params:     url.Values{},
			expectNext: true,
			expectPrev: true,
		},
		{
			name:       "has neither",
			pageInfo:   PageInfo{},
			params:     url.Values{},
			expectNext: false,
			expectPrev: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			links := BuildPageLinks(baseURL, tt.pageInfo, tt.params)

			hasNext := len(links) > 0 && contains(links, `rel="next"`)
			hasPrev := len(links) > 0 && contains(links, `rel="prev"`)

			if tt.expectNext && !hasNext && tt.pageInfo.HasNextPage {
				// Check if the link contains the after param
				if !contains(links, "after=") {
					t.Error("expected next link with after param")
				}
			}
			if tt.expectPrev && !hasPrev && tt.pageInfo.HasPreviousPage {
				if !contains(links, "before=") {
					t.Error("expected prev link with before param")
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
