package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"github.com/lib/pq"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// EventRepo implements EventRepository
type EventRepo struct {
	db DBTX
}

// NewEventRepo creates a new event repository
func NewEventRepo(db DBTX) *EventRepo {
	return &EventRepo{db: db}
}

// Create inserts a new event
func (r *EventRepo) Create(ctx context.Context, event *models.Event) error {
	query := `
		INSERT INTO events (
			pod_id, session_id, vm_name, agent_id, event_type,
			rule_id, rule_level, description, data, processed, matched_checkpoints
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, timestamp`

	var ruleID sql.NullInt64
	if event.WazuhRuleID != "" {
		if parsed, err := strconv.ParseInt(event.WazuhRuleID, 10, 64); err == nil {
			ruleID = sql.NullInt64{Int64: parsed, Valid: true}
		}
	}

	var ruleLevel sql.NullInt64
	if event.WazuhLevel > 0 {
		ruleLevel = sql.NullInt64{Int64: int64(event.WazuhLevel), Valid: true}
	}

	err := r.db.QueryRowContext(ctx, query,
		event.PodID,
		nullString(event.SessionID),
		event.VMName,
		event.AgentID,
		event.EventType,
		ruleID,
		ruleLevel,
		event.WazuhRuleDesc,
		event.Data,
		event.Processed,
		pq.Array(event.MatchedCheckpoints),
	).Scan(&event.ID, &event.Timestamp)

	if err != nil {
		return fmt.Errorf("inserting event: %w", err)
	}

	return nil
}

// GetByID retrieves an event by ID
func (r *EventRepo) GetByID(ctx context.Context, id int64) (*models.Event, error) {
	query := `
		SELECT id, timestamp, pod_id, session_id, vm_name, agent_id,
		       event_type, rule_id, rule_level, description, data,
		       processed, matched_checkpoints
		FROM events
		WHERE id = $1`

	event, err := r.scanEvent(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return event, err
}

// GetByPodID retrieves events for a pod
func (r *EventRepo) GetByPodID(ctx context.Context, podID string, limit int) ([]*models.Event, error) {
	query := `
		SELECT id, timestamp, pod_id, session_id, vm_name, agent_id,
		       event_type, rule_id, rule_level, description, data,
		       processed, matched_checkpoints
		FROM events
		WHERE pod_id = $1
		ORDER BY timestamp DESC
		LIMIT $2`

	return r.queryMultiple(ctx, query, podID, limit)
}

// GetBySessionID retrieves events for a session
func (r *EventRepo) GetBySessionID(ctx context.Context, sessionID string, limit int) ([]*models.Event, error) {
	query := `
		SELECT id, timestamp, pod_id, session_id, vm_name, agent_id,
		       event_type, rule_id, rule_level, description, data,
		       processed, matched_checkpoints
		FROM events
		WHERE session_id = $1
		ORDER BY timestamp DESC
		LIMIT $2`

	return r.queryMultiple(ctx, query, sessionID, limit)
}

// GetUnprocessed retrieves unprocessed events
func (r *EventRepo) GetUnprocessed(ctx context.Context, limit int) ([]*models.Event, error) {
	query := `
		SELECT id, timestamp, pod_id, session_id, vm_name, agent_id,
		       event_type, rule_id, rule_level, description, data,
		       processed, matched_checkpoints
		FROM events
		WHERE processed = false
		ORDER BY timestamp ASC
		LIMIT $1`

	return r.queryMultiple(ctx, query, limit)
}

// MarkProcessed marks an event as processed
func (r *EventRepo) MarkProcessed(ctx context.Context, id int64, matchedCheckpoints []string) error {
	query := `
		UPDATE events
		SET processed = true, matched_checkpoints = $2
		WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, id, pq.Array(matchedCheckpoints))
	if err != nil {
		return fmt.Errorf("marking event processed: %w", err)
	}
	return nil
}

// Query retrieves events matching the filter
func (r *EventRepo) Query(ctx context.Context, filter EventFilter) ([]*models.Event, error) {
	qb := NewQueryBuilder(`
		SELECT id, timestamp, pod_id, session_id, vm_name, agent_id,
		       event_type, rule_id, rule_level, description, data,
		       processed, matched_checkpoints
		FROM events
		WHERE 1=1`)

	if filter.PodID != "" {
		qb.AddCondition("pod_id = $%d", filter.PodID)
	}
	if filter.SessionID != "" {
		qb.AddCondition("session_id = $%d", filter.SessionID)
	}
	if filter.VMName != "" {
		qb.AddCondition("vm_name = $%d", filter.VMName)
	}
	if filter.EventType != "" {
		qb.AddCondition("event_type = $%d", filter.EventType)
	}
	if filter.StartTime != nil {
		qb.AddCondition("timestamp >= $%d", *filter.StartTime)
	}
	if filter.EndTime != nil {
		qb.AddCondition("timestamp <= $%d", *filter.EndTime)
	}

	qb.OrderByRaw("timestamp DESC")
	qb.DefaultLimit(filter.Limit, 100)
	qb.Offset(filter.Offset)

	query, args := qb.Build()

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying events: %w", err)
	}
	defer rows.Close()

	return r.scanMultiple(rows)
}

// Helper methods

func (r *EventRepo) queryMultiple(ctx context.Context, query string, args ...any) ([]*models.Event, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying events: %w", err)
	}
	defer rows.Close()

	return r.scanMultiple(rows)
}

func (r *EventRepo) scanMultiple(rows *sql.Rows) ([]*models.Event, error) {
	var events []*models.Event
	for rows.Next() {
		event, err := r.scanEventRow(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

type scannableEvent interface {
	Scan(dest ...any) error
}

func (r *EventRepo) scanEvent(row scannableEvent) (*models.Event, error) {
	var event models.Event
	var sessionID sql.NullString
	var ruleID, ruleLevel sql.NullInt64
	var description sql.NullString
	var matchedCheckpoints pq.StringArray

	err := row.Scan(
		&event.ID,
		&event.Timestamp,
		&event.PodID,
		&sessionID,
		&event.VMName,
		&event.AgentID,
		&event.EventType,
		&ruleID,
		&ruleLevel,
		&description,
		&event.Data,
		&event.Processed,
		&matchedCheckpoints,
	)

	if err != nil {
		return nil, fmt.Errorf("scanning event: %w", err)
	}

	event.SessionID = sessionID.String
	if ruleID.Valid {
		event.WazuhRuleID = fmt.Sprintf("%d", ruleID.Int64)
	}
	event.WazuhLevel = int(ruleLevel.Int64)
	event.WazuhRuleDesc = description.String
	event.MatchedCheckpoints = matchedCheckpoints

	return &event, nil
}

func (r *EventRepo) scanEventRow(rows *sql.Rows) (*models.Event, error) {
	return r.scanEvent(rows)
}
