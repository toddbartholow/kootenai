// Package repositories provides database repository implementations
package repositories

import (
	"fmt"
	"strings"
)

// QueryBuilder provides a safe way to build dynamic SQL queries with parameterized conditions.
// It tracks argument numbers automatically to prevent misalignment between placeholders and arguments.
type QueryBuilder struct {
	baseQuery  string
	conditions []string
	args       []any
	argNum     int
	orderBy    string
	limit      int
	offset     int
}

// NewQueryBuilder creates a new query builder with the given base query.
// The base query should include "WHERE 1=1" or similar to allow appending conditions.
func NewQueryBuilder(baseQuery string) *QueryBuilder {
	return &QueryBuilder{
		baseQuery:  baseQuery,
		conditions: make([]string, 0),
		args:       make([]any, 0),
		argNum:     1,
	}
}

// AddCondition adds a condition with a single parameter.
// The condition should use %d as a placeholder for the parameter number, e.g., "column = $%d".
func (qb *QueryBuilder) AddCondition(conditionFormat string, arg any) *QueryBuilder {
	condition := fmt.Sprintf(conditionFormat, qb.argNum)
	qb.conditions = append(qb.conditions, condition)
	qb.args = append(qb.args, arg)
	qb.argNum++
	return qb
}

// AddRawCondition adds a condition without parameters.
// Use sparingly - only for static conditions without user input.
func (qb *QueryBuilder) AddRawCondition(condition string) *QueryBuilder {
	qb.conditions = append(qb.conditions, condition)
	return qb
}

// AddConditionMulti adds a single condition that consumes multiple positional
// placeholders. The format string should contain N "%d" tokens — one per arg —
// and the N args are appended to the argument list in order.
//
// Typical use is a multi-column ILIKE search where the same logical clause
// spans several positional placeholders:
//
//	qb.AddConditionMulti(
//	    "(name ILIKE $%d OR email ILIKE $%d)",
//	    "%"+search+"%", "%"+search+"%",
//	)
//
// Prefer AddCondition for single-placeholder conditions; use this only when
// the condition legitimately references multiple placeholders in one OR/AND
// group.
func (qb *QueryBuilder) AddConditionMulti(format string, args ...any) *QueryBuilder {
	if len(args) == 0 {
		qb.conditions = append(qb.conditions, format)
		return qb
	}
	placeholderNums := make([]any, len(args))
	for i := range args {
		placeholderNums[i] = qb.argNum + i
	}
	qb.conditions = append(qb.conditions, fmt.Sprintf(format, placeholderNums...))
	qb.args = append(qb.args, args...)
	qb.argNum += len(args)
	return qb
}

// validOrderColumns defines allowed columns for ORDER BY clauses.
// This prevents SQL injection via ORDER BY.
var validOrderColumns = map[string]bool{
	// Common columns across tables
	"id": true, "name": true, "created_at": true, "updated_at": true,
	// Lab template columns
	"platform": true, "difficulty": true, "is_active": true, "visibility": true,
	// Enrollment columns
	"enrolled_at": true, "started_at": true, "completed_at": true,
	"last_activity_at": true, "percentage": true, "status": true,
	// Session columns
	"score": true, "max_score": true,
	// Module columns
	"display_order": true,
	// With table prefixes
	"e.id": true, "e.user_id": true, "e.pathway_id": true, "e.status": true,
	"e.enrolled_at": true, "e.started_at": true, "e.completed_at": true,
	"e.last_activity_at": true, "e.percentage": true,
	"pm.display_order": true,
	"mp.display_order": true,
	"ml.display_order": true,
}

// validOrderDirections defines allowed ORDER BY directions.
var validOrderDirections = map[string]bool{
	"ASC": true, "DESC": true, "asc": true, "desc": true,
	"ASC NULLS FIRST": true, "ASC NULLS LAST": true,
	"DESC NULLS FIRST": true, "DESC NULLS LAST": true,
}

// OrderBy sets the ORDER BY clause with validation.
// Only whitelisted column names and directions are allowed to prevent SQL injection.
// For complex ORDER BY clauses, use OrderByRaw with caution.
func (qb *QueryBuilder) OrderBy(orderBy string) *QueryBuilder {
	// Parse and validate the ORDER BY clause
	// Expected format: "column [ASC|DESC] [NULLS FIRST|LAST], ..."
	parts := strings.Split(orderBy, ",")
	var validatedParts []string

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Split into column and direction
		tokens := strings.Fields(part)
		if len(tokens) == 0 {
			continue
		}

		column := tokens[0]
		if !validOrderColumns[column] {
			// Skip invalid columns silently (could also log a warning)
			continue
		}

		// Reconstruct with validated direction
		if len(tokens) > 1 {
			direction := strings.Join(tokens[1:], " ")
			if validOrderDirections[direction] {
				validatedParts = append(validatedParts, column+" "+strings.ToUpper(direction))
			} else {
				// Default to ASC if direction is invalid
				validatedParts = append(validatedParts, column+" ASC")
			}
		} else {
			validatedParts = append(validatedParts, column)
		}
	}

	if len(validatedParts) > 0 {
		qb.orderBy = strings.Join(validatedParts, ", ")
	}
	return qb
}

// OrderByRaw sets a raw ORDER BY clause without validation.
// WARNING: Only use this with hardcoded strings, never with user input.
// This should only be used for complex expressions that can't be validated.
func (qb *QueryBuilder) OrderByRaw(orderBy string) *QueryBuilder {
	qb.orderBy = orderBy
	return qb
}

// Limit sets the LIMIT clause.
func (qb *QueryBuilder) Limit(limit int) *QueryBuilder {
	if limit > 0 {
		qb.limit = limit
	}
	return qb
}

// DefaultLimit sets the LIMIT clause with a fallback default.
// If limit is <= 0, defaultLimit is used instead, preventing unbounded queries.
func (qb *QueryBuilder) DefaultLimit(limit, defaultLimit int) *QueryBuilder {
	if limit > 0 {
		qb.limit = limit
	} else {
		qb.limit = defaultLimit
	}
	return qb
}

// Offset sets the OFFSET clause.
func (qb *QueryBuilder) Offset(offset int) *QueryBuilder {
	if offset > 0 {
		qb.offset = offset
	}
	return qb
}

// Build constructs the final query and returns it along with the arguments.
// Build is idempotent — calling it multiple times produces the same result.
func (qb *QueryBuilder) Build() (string, []any) {
	var sb strings.Builder
	sb.WriteString(qb.baseQuery)

	// Add conditions
	for _, cond := range qb.conditions {
		sb.WriteString(" AND ")
		sb.WriteString(cond)
	}

	// Add ORDER BY
	if qb.orderBy != "" {
		sb.WriteString(" ORDER BY ")
		sb.WriteString(qb.orderBy)
	}

	// Work on copies to keep Build() idempotent
	args := make([]any, len(qb.args))
	copy(args, qb.args)
	argNum := qb.argNum

	// Add LIMIT
	if qb.limit > 0 {
		sb.WriteString(fmt.Sprintf(" LIMIT $%d", argNum))
		args = append(args, qb.limit)
		argNum++
	}

	// Add OFFSET
	if qb.offset > 0 {
		sb.WriteString(fmt.Sprintf(" OFFSET $%d", argNum))
		args = append(args, qb.offset)
	}

	return sb.String(), args
}

// Args returns the current list of arguments.
func (qb *QueryBuilder) Args() []any {
	return qb.args
}

// ArgCount returns the current number of arguments.
func (qb *QueryBuilder) ArgCount() int {
	return len(qb.args)
}
