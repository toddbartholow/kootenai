package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// AuditLogRepo implements AuditLogRepository
type AuditLogRepo struct {
	db DBTX
}

// NewAuditLogRepo creates a new audit log repository
func NewAuditLogRepo(db DBTX) *AuditLogRepo {
	return &AuditLogRepo{db: db}
}

// Create inserts a new audit log entry
func (r *AuditLogRepo) Create(ctx context.Context, entry *models.AuditEntry) error {
	query := `
		INSERT INTO audit_log (
			actor_id, actor_type, action, resource_type, resource_id,
			details, ip_address, user_agent
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, timestamp`

	err := r.db.QueryRowContext(ctx, query,
		entry.ActorID,
		entry.ActorType,
		entry.Action,
		entry.ResourceType,
		entry.ResourceID,
		entry.Details,
		entry.IPAddress,
		entry.UserAgent,
	).Scan(&entry.ID, &entry.Timestamp)

	if err != nil {
		return fmt.Errorf("inserting audit entry: %w", err)
	}

	return nil
}

// Query retrieves audit entries matching the filter
func (r *AuditLogRepo) Query(ctx context.Context, filter AuditFilter) ([]*models.AuditEntry, error) {
	qb := NewQueryBuilder(`
		SELECT id, timestamp, actor_id, actor_type, action, resource_type,
		       resource_id, details, ip_address, user_agent
		FROM audit_log
		WHERE 1=1`)

	if filter.ActorID != "" {
		qb.AddCondition("actor_id = $%d", filter.ActorID)
	}
	if filter.Action != "" {
		qb.AddCondition("action = $%d", filter.Action)
	}
	if filter.ResourceType != "" {
		qb.AddCondition("resource_type = $%d", filter.ResourceType)
	}
	if filter.ResourceID != "" {
		qb.AddCondition("resource_id = $%d", filter.ResourceID)
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
		return nil, fmt.Errorf("querying audit logs: %w", err)
	}
	defer rows.Close()

	var entries []*models.AuditEntry
	for rows.Next() {
		var entry models.AuditEntry
		var resourceID, ipAddress, userAgent sql.NullString
		var detailsJSON []byte

		err := rows.Scan(
			&entry.ID,
			&entry.Timestamp,
			&entry.ActorID,
			&entry.ActorType,
			&entry.Action,
			&entry.ResourceType,
			&resourceID,
			&detailsJSON,
			&ipAddress,
			&userAgent,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning audit entry: %w", err)
		}

		if resourceID.Valid {
			entry.ResourceID = resourceID.String
		}
		if ipAddress.Valid {
			entry.IPAddress = ipAddress.String
		}
		if userAgent.Valid {
			entry.UserAgent = userAgent.String
		}
		if len(detailsJSON) > 0 {
			entry.Details = json.RawMessage(detailsJSON)
		}

		entries = append(entries, &entry)
	}

	return entries, rows.Err()
}
