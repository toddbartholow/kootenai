package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// PodRepo implements PodRepository
type PodRepo struct {
	db DBTX
}

// NewPodRepo creates a new pod repository
func NewPodRepo(db DBTX) *PodRepo {
	return &PodRepo{db: db}
}

// Create inserts a new pod
func (r *PodRepo) Create(ctx context.Context, pod *models.Pod) error {
	vmsJSON, err := json.Marshal(pod.VMs)
	if err != nil {
		return fmt.Errorf("marshaling VMs: %w", err)
	}

	networksJSON, err := json.Marshal(pod.Networks)
	if err != nil {
		return fmt.Errorf("marshaling networks: %w", err)
	}

	metadataJSON, err := json.Marshal(pod.Metadata)
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	query := `
		INSERT INTO pods (id, name, lab_template_id, owner_id, platform, status, vms, networks, expires_at, metadata, organization_id, team_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING created_at`

	err = r.db.QueryRowContext(ctx, query,
		pod.ID,
		nullString(pod.Name),
		pod.LabTemplateID,
		pod.OwnerID,
		pod.Platform,
		pod.Status,
		vmsJSON,
		networksJSON,
		pod.ExpiresAt,
		metadataJSON,
		nullStringPtr(pod.OrganizationID),
		nullStringPtr(pod.TeamID),
	).Scan(&pod.CreatedAt)

	if err != nil {
		return fmt.Errorf("inserting pod: %w", err)
	}

	return nil
}

// GetByID retrieves a pod by ID
func (r *PodRepo) GetByID(ctx context.Context, id string) (*models.Pod, error) {
	query := `
		SELECT p.id, p.name, p.lab_template_id, p.owner_id, p.platform, p.status, p.vms, p.networks,
		       p.created_at, p.expires_at, p.metadata, p.organization_id, p.team_id,
		       COALESCE(lt.name, '') as lab_template_name,
		       COALESCE(u.username, '') as owner_name
		FROM pods p
		LEFT JOIN lab_templates lt ON p.lab_template_id = lt.id
		LEFT JOIN users u ON p.owner_id = u.id
		WHERE p.id = $1`

	var pod models.Pod
	var vmsJSON, networksJSON, metadataJSON []byte
	var organizationID, teamID, podName sql.NullString

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&pod.ID,
		&podName,
		&pod.LabTemplateID,
		&pod.OwnerID,
		&pod.Platform,
		&pod.Status,
		&vmsJSON,
		&networksJSON,
		&pod.CreatedAt,
		&pod.ExpiresAt,
		&metadataJSON,
		&organizationID,
		&teamID,
		&pod.LabTemplate,
		&pod.Owner,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying pod: %w", err)
	}

	if err := json.Unmarshal(vmsJSON, &pod.VMs); err != nil {
		return nil, fmt.Errorf("unmarshaling VMs: %w", err)
	}
	if err := json.Unmarshal(networksJSON, &pod.Networks); err != nil {
		return nil, fmt.Errorf("unmarshaling networks: %w", err)
	}
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &pod.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshaling metadata: %w", err)
		}
	}
	if podName.Valid {
		pod.Name = podName.String
	}
	if organizationID.Valid {
		pod.OrganizationID = &organizationID.String
	}
	if teamID.Valid {
		pod.TeamID = &teamID.String
	}

	return &pod, nil
}

// List retrieves pods matching the filter
func (r *PodRepo) List(ctx context.Context, filter PodFilter) ([]*models.Pod, error) {
	qb := NewQueryBuilder(`
		SELECT p.id, p.name, p.lab_template_id, p.owner_id, p.platform, p.status, p.vms, p.networks,
		       p.created_at, p.expires_at, p.metadata, p.organization_id, p.team_id,
		       COALESCE(lt.name, '') as lab_template_name,
		       COALESCE(u.username, '') as owner_name
		FROM pods p
		LEFT JOIN lab_templates lt ON p.lab_template_id = lt.id
		LEFT JOIN users u ON p.owner_id = u.id
		WHERE 1=1`)

	if filter.OwnerID != "" {
		qb.AddCondition("p.owner_id = $%d", filter.OwnerID)
	}
	if filter.TemplateID != "" {
		qb.AddCondition("p.lab_template_id = $%d", filter.TemplateID)
	}
	if filter.Status != "" {
		qb.AddCondition("p.status = $%d", filter.Status)
	}
	if filter.Platform != "" {
		qb.AddCondition("p.platform = $%d", filter.Platform)
	}
	if filter.OrganizationID != "" {
		qb.AddCondition("p.organization_id = $%d", filter.OrganizationID)
	}
	if filter.TeamID != "" {
		qb.AddCondition("p.team_id = $%d", filter.TeamID)
	}

	qb.OrderByRaw("p.created_at DESC")
	qb.DefaultLimit(filter.Limit, 100)
	qb.Offset(filter.Offset)

	query, args := qb.Build()

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying pods: %w", err)
	}
	defer rows.Close()

	var pods []*models.Pod
	for rows.Next() {
		var pod models.Pod
		var vmsJSON, networksJSON, metadataJSON []byte
		var organizationID, teamID, podName sql.NullString

		err := rows.Scan(
			&pod.ID,
			&podName,
			&pod.LabTemplateID,
			&pod.OwnerID,
			&pod.Platform,
			&pod.Status,
			&vmsJSON,
			&networksJSON,
			&pod.CreatedAt,
			&pod.ExpiresAt,
			&metadataJSON,
			&organizationID,
			&teamID,
			&pod.LabTemplate,
			&pod.Owner,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning pod: %w", err)
		}

		if err := json.Unmarshal(vmsJSON, &pod.VMs); err != nil {
			return nil, fmt.Errorf("unmarshaling VMs: %w", err)
		}
		if err := json.Unmarshal(networksJSON, &pod.Networks); err != nil {
			return nil, fmt.Errorf("unmarshaling networks: %w", err)
		}
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &pod.Metadata); err != nil {
				return nil, fmt.Errorf("unmarshaling metadata: %w", err)
			}
		}
		if podName.Valid {
			pod.Name = podName.String
		}
		if organizationID.Valid {
			pod.OrganizationID = &organizationID.String
		}
		if teamID.Valid {
			pod.TeamID = &teamID.String
		}

		pods = append(pods, &pod)
	}

	return pods, rows.Err()
}

// Update updates a pod
func (r *PodRepo) Update(ctx context.Context, pod *models.Pod) error {
	vmsJSON, err := json.Marshal(pod.VMs)
	if err != nil {
		return fmt.Errorf("marshaling VMs: %w", err)
	}

	networksJSON, err := json.Marshal(pod.Networks)
	if err != nil {
		return fmt.Errorf("marshaling networks: %w", err)
	}

	metadataJSON, err := json.Marshal(pod.Metadata)
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	query := `
		UPDATE pods
		SET status = $2, vms = $3, networks = $4, expires_at = $5, metadata = $6,
		    organization_id = $7, team_id = $8
		WHERE id = $1`

	res, err := r.db.ExecContext(ctx, query,
		pod.ID,
		pod.Status,
		vmsJSON,
		networksJSON,
		pod.ExpiresAt,
		metadataJSON,
		nullStringPtr(pod.OrganizationID),
		nullStringPtr(pod.TeamID),
	)

	if err != nil {
		return fmt.Errorf("updating pod: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("pod not found: %s", pod.ID)
	}

	return nil
}

// UpdateStatus updates only the pod status
func (r *PodRepo) UpdateStatus(ctx context.Context, id string, status models.PodStatus) error {
	query := `UPDATE pods SET status = $2 WHERE id = $1`
	res, err := r.db.ExecContext(ctx, query, id, status)
	if err != nil {
		return fmt.Errorf("updating pod status: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("pod not found: %s", id)
	}
	return nil
}

// Delete removes a pod
func (r *PodRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM pods WHERE id = $1`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting pod: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("pod not found: %s", id)
	}
	return nil
}

// GetOwnerID returns the owner ID for a pod
func (r *PodRepo) GetOwnerID(ctx context.Context, id string) (string, error) {
	query := `SELECT owner_id FROM pods WHERE id = $1`
	var ownerID string
	err := r.db.QueryRowContext(ctx, query, id).Scan(&ownerID)
	if err != nil {
		return "", fmt.Errorf("getting pod owner: %w", err)
	}
	return ownerID, nil
}

// IsOwner checks if a user owns a pod
func (r *PodRepo) IsOwner(ctx context.Context, id, userID string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM pods WHERE id = $1 AND owner_id = $2)`
	var exists bool
	err := r.db.QueryRowContext(ctx, query, id, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking pod ownership: %w", err)
	}
	return exists, nil
}

// GetOrganizationID returns the organization ID for a pod (if any)
func (r *PodRepo) GetOrganizationID(ctx context.Context, id string) (*string, error) {
	query := `SELECT organization_id FROM pods WHERE id = $1`
	var orgID sql.NullString
	err := r.db.QueryRowContext(ctx, query, id).Scan(&orgID)
	if err != nil {
		return nil, fmt.Errorf("getting pod organization: %w", err)
	}
	if orgID.Valid {
		return &orgID.String, nil
	}
	return nil, nil
}

// CountActive returns the number of pods that still hold hypervisor resources:
// everything except `destroyed`.
//
// `destroying` counts. DestroyPod sets that status before touching a single VM
// and then spends up to 30s per VM waiting for shutdown, so a destroying pod
// holds its resources for the whole teardown — and a process that dies
// mid-teardown leaves the pod there permanently. Excluding it would under-count
// every destroy for its duration and hide stuck teardowns entirely.
//
// This is one status narrower than GetExpired's predicate, which also excludes
// `destroying` because it is selecting candidates to destroy rather than
// counting resource holders.
func (r *PodRepo) CountActive(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pods WHERE status <> 'destroyed'`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting active pods: %w", err)
	}
	return count, nil
}

// GetExpired retrieves all expired pods
func (r *PodRepo) GetExpired(ctx context.Context) ([]*models.Pod, error) {
	query := `
		SELECT id, name, lab_template_id, owner_id, platform, status, vms, networks,
		       created_at, expires_at, metadata, organization_id, team_id
		FROM pods
		WHERE expires_at IS NOT NULL AND expires_at < $1
		  AND status NOT IN ('destroyed', 'destroying')`

	rows, err := r.db.QueryContext(ctx, query, time.Now())
	if err != nil {
		return nil, fmt.Errorf("querying expired pods: %w", err)
	}
	defer rows.Close()

	var pods []*models.Pod
	for rows.Next() {
		var pod models.Pod
		var vmsJSON, networksJSON, metadataJSON []byte
		var organizationID, teamID, podName sql.NullString

		err := rows.Scan(
			&pod.ID,
			&podName,
			&pod.LabTemplateID,
			&pod.OwnerID,
			&pod.Platform,
			&pod.Status,
			&vmsJSON,
			&networksJSON,
			&pod.CreatedAt,
			&pod.ExpiresAt,
			&metadataJSON,
			&organizationID,
			&teamID,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning pod: %w", err)
		}

		if err := json.Unmarshal(vmsJSON, &pod.VMs); err != nil {
			return nil, fmt.Errorf("unmarshaling VMs: %w", err)
		}
		if err := json.Unmarshal(networksJSON, &pod.Networks); err != nil {
			return nil, fmt.Errorf("unmarshaling networks: %w", err)
		}
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &pod.Metadata); err != nil {
				return nil, fmt.Errorf("unmarshaling metadata: %w", err)
			}
		}
		if podName.Valid {
			pod.Name = podName.String
		}
		if organizationID.Valid {
			pod.OrganizationID = &organizationID.String
		}
		if teamID.Valid {
			pod.TeamID = &teamID.String
		}

		pods = append(pods, &pod)
	}

	return pods, rows.Err()
}
