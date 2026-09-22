package repositories

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"

	"github.com/toddbartholow/kootenai/api/internal/models"
	redisclient "github.com/toddbartholow/kootenai/api/internal/redis"
)

// CachedLabTemplateRepo wraps LabTemplateRepository with Redis caching
type CachedLabTemplateRepo struct {
	repo   LabTemplateRepository
	cache  *redisclient.TemplateCache
	logger *slog.Logger
}

// NewCachedLabTemplateRepo creates a new cached lab template repository
func NewCachedLabTemplateRepo(repo LabTemplateRepository, cache *redisclient.TemplateCache, logger *slog.Logger) *CachedLabTemplateRepo {
	if logger == nil {
		logger = slog.Default()
	}
	return &CachedLabTemplateRepo{
		repo:   repo,
		cache:  cache,
		logger: logger,
	}
}

// Create creates a new template and invalidates relevant caches
func (r *CachedLabTemplateRepo) Create(ctx context.Context, record *models.LabTemplateRecord) error {
	if err := r.repo.Create(ctx, record); err != nil {
		return err
	}
	// Invalidate list caches since a new template was added
	if r.cache != nil {
		if err := r.cache.InvalidateAll(ctx); err != nil {
			r.logger.Warn("failed to invalidate template list cache", "error", err)
		}
	}
	return nil
}

// GetByID retrieves a template by ID with caching
func (r *CachedLabTemplateRepo) GetByID(ctx context.Context, id string) (*models.LabTemplateRecord, error) {
	if r.cache != nil {
		var cached models.LabTemplateRecord
		if err := r.cache.GetTemplate(ctx, id, &cached); err == nil {
			r.logger.Debug("template cache hit", "id", id)
			return &cached, nil
		}
	}

	record, err := r.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, nil
	}

	// Cache the result
	if r.cache != nil {
		if err := r.cache.SetTemplate(ctx, id, record); err != nil {
			r.logger.Warn("failed to cache template", "id", id, "error", err)
		}
	}

	return record, nil
}

// GetByIDs retrieves multiple templates by IDs (delegates to underlying repo without caching)
func (r *CachedLabTemplateRepo) GetByIDs(ctx context.Context, ids []string) ([]*models.LabTemplateRecord, error) {
	return r.repo.GetByIDs(ctx, ids)
}

// GetByName retrieves a template by name with caching
func (r *CachedLabTemplateRepo) GetByName(ctx context.Context, name string) (*models.LabTemplateRecord, error) {
	cacheKey := "name:" + name
	if r.cache != nil {
		var cached models.LabTemplateRecord
		if err := r.cache.GetTemplate(ctx, cacheKey, &cached); err == nil {
			r.logger.Debug("template cache hit", "name", name)
			return &cached, nil
		}
	}

	record, err := r.repo.GetByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, nil
	}

	// Cache the result by name
	if r.cache != nil {
		if err := r.cache.SetTemplate(ctx, cacheKey, record); err != nil {
			r.logger.Warn("failed to cache template", "name", name, "error", err)
		}
		// Also cache by ID for cross-reference
		if err := r.cache.SetTemplate(ctx, record.ID, record); err != nil {
			r.logger.Warn("failed to cache template by ID", "id", record.ID, "error", err)
		}
	}

	return record, nil
}

// List retrieves templates matching the filter with caching
func (r *CachedLabTemplateRepo) List(ctx context.Context, filter LabTemplateFilter) ([]*models.LabTemplateRecord, error) {
	cacheKey := hashFilter(filter)
	if r.cache != nil {
		var cached []*models.LabTemplateRecord
		if err := r.cache.GetTemplateList(ctx, cacheKey, &cached); err == nil {
			r.logger.Debug("template list cache hit", "filter", cacheKey)
			return cached, nil
		}
	}

	records, err := r.repo.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if r.cache != nil {
		if err := r.cache.SetTemplateList(ctx, cacheKey, records); err != nil {
			r.logger.Warn("failed to cache template list", "filter", cacheKey, "error", err)
		}
	}

	return records, nil
}

// Update updates a template and invalidates caches
func (r *CachedLabTemplateRepo) Update(ctx context.Context, record *models.LabTemplateRecord) error {
	if err := r.repo.Update(ctx, record); err != nil {
		return err
	}
	r.invalidateTemplate(ctx, record.ID, record.Name)
	return nil
}

// Delete deletes a template and invalidates caches
func (r *CachedLabTemplateRepo) Delete(ctx context.Context, id string) error {
	// Get the template first to know its name for cache invalidation
	record, _ := r.repo.GetByID(ctx, id)
	if err := r.repo.Delete(ctx, id); err != nil {
		return err
	}
	if record != nil {
		r.invalidateTemplate(ctx, id, record.Name)
	} else {
		r.invalidateTemplate(ctx, id, "")
	}
	return nil
}

// SetActive sets the active status of a template and invalidates caches
func (r *CachedLabTemplateRepo) SetActive(ctx context.Context, id string, active bool) error {
	// Get the template first to know its name for cache invalidation
	record, _ := r.repo.GetByID(ctx, id)
	if err := r.repo.SetActive(ctx, id, active); err != nil {
		return err
	}
	if record != nil {
		r.invalidateTemplate(ctx, id, record.Name)
	} else {
		r.invalidateTemplate(ctx, id, "")
	}
	return nil
}

// CreateVersion delegates to the underlying repo (no caching for versions)
func (r *CachedLabTemplateRepo) CreateVersion(ctx context.Context, version *models.LabTemplateVersion) error {
	return r.repo.CreateVersion(ctx, version)
}

// ListVersions delegates to the underlying repo
func (r *CachedLabTemplateRepo) ListVersions(ctx context.Context, templateID string, limit, offset int) ([]*models.LabTemplateVersion, error) {
	return r.repo.ListVersions(ctx, templateID, limit, offset)
}

// GetVersionByNumber delegates to the underlying repo
func (r *CachedLabTemplateRepo) GetVersionByNumber(ctx context.Context, templateID string, versionNumber int) (*models.LabTemplateVersion, error) {
	return r.repo.GetVersionByNumber(ctx, templateID, versionNumber)
}

// CountVersions delegates to the underlying repo
func (r *CachedLabTemplateRepo) CountVersions(ctx context.Context, templateID string) (int, error) {
	return r.repo.CountVersions(ctx, templateID)
}

// invalidateTemplate removes template from all caches
func (r *CachedLabTemplateRepo) invalidateTemplate(ctx context.Context, id, name string) {
	if r.cache == nil {
		return
	}

	if err := r.cache.InvalidateTemplate(ctx, id); err != nil {
		r.logger.Warn("failed to invalidate template cache", "id", id, "error", err)
	}
	if name != "" {
		if err := r.cache.InvalidateTemplate(ctx, "name:"+name); err != nil {
			r.logger.Warn("failed to invalidate template cache by name", "name", name, "error", err)
		}
	}
	// Invalidate list caches
	if err := r.cache.InvalidateAll(ctx); err != nil {
		r.logger.Warn("failed to invalidate template list caches", "error", err)
	}
}

// hashFilter creates a deterministic cache key from the filter
func hashFilter(filter LabTemplateFilter) string {
	data, _ := json.Marshal(filter)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:8]) // Use first 8 bytes
}

// Verify interface compliance
var _ LabTemplateRepository = (*CachedLabTemplateRepo)(nil)

// CachedPodRepo wraps PodRepository with Redis caching
type CachedPodRepo struct {
	repo   PodRepository
	cache  *redisclient.PodCache
	logger *slog.Logger
}

// NewCachedPodRepo creates a new cached pod repository
func NewCachedPodRepo(repo PodRepository, cache *redisclient.PodCache, logger *slog.Logger) *CachedPodRepo {
	if logger == nil {
		logger = slog.Default()
	}
	return &CachedPodRepo{
		repo:   repo,
		cache:  cache,
		logger: logger,
	}
}

// Create creates a new pod
func (r *CachedPodRepo) Create(ctx context.Context, pod *models.Pod) error {
	return r.repo.Create(ctx, pod)
}

// GetByID retrieves a pod by ID with caching
func (r *CachedPodRepo) GetByID(ctx context.Context, id string) (*models.Pod, error) {
	if r.cache != nil {
		var cached models.Pod
		if err := r.cache.GetPod(ctx, id, &cached); err == nil {
			r.logger.Debug("pod cache hit", "id", id)
			return &cached, nil
		}
	}

	pod, err := r.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if pod == nil {
		return nil, nil
	}

	// Cache the result
	if r.cache != nil {
		if err := r.cache.SetPod(ctx, id, pod); err != nil {
			r.logger.Warn("failed to cache pod", "id", id, "error", err)
		}
	}

	return pod, nil
}

// List retrieves pods matching the filter (not cached - could be stale quickly)
func (r *CachedPodRepo) List(ctx context.Context, filter PodFilter) ([]*models.Pod, error) {
	return r.repo.List(ctx, filter)
}

// Update updates a pod and invalidates cache
func (r *CachedPodRepo) Update(ctx context.Context, pod *models.Pod) error {
	if err := r.repo.Update(ctx, pod); err != nil {
		return err
	}
	if r.cache != nil {
		if err := r.cache.InvalidatePod(ctx, pod.ID); err != nil {
			r.logger.Warn("failed to invalidate pod cache", "id", pod.ID, "error", err)
		}
	}
	return nil
}

// UpdateStatus updates pod status and invalidates cache
func (r *CachedPodRepo) UpdateStatus(ctx context.Context, id string, status models.PodStatus) error {
	if err := r.repo.UpdateStatus(ctx, id, status); err != nil {
		return err
	}
	if r.cache != nil {
		if err := r.cache.InvalidatePod(ctx, id); err != nil {
			r.logger.Warn("failed to invalidate pod cache", "id", id, "error", err)
		}
	}
	return nil
}

// Delete deletes a pod and invalidates cache
func (r *CachedPodRepo) Delete(ctx context.Context, id string) error {
	if err := r.repo.Delete(ctx, id); err != nil {
		return err
	}
	if r.cache != nil {
		if err := r.cache.InvalidatePod(ctx, id); err != nil {
			r.logger.Warn("failed to invalidate pod cache", "id", id, "error", err)
		}
	}
	return nil
}

// GetExpired retrieves expired pods (not cached)
func (r *CachedPodRepo) GetExpired(ctx context.Context) ([]*models.Pod, error) {
	return r.repo.GetExpired(ctx)
}

// CountActive is not cached — callers want a live count.
func (r *CachedPodRepo) CountActive(ctx context.Context) (int64, error) {
	return r.repo.CountActive(ctx)
}

// GetOwnerID returns the owner ID for a pod
func (r *CachedPodRepo) GetOwnerID(ctx context.Context, id string) (string, error) {
	return r.repo.GetOwnerID(ctx, id)
}

// IsOwner checks if a user owns a pod
func (r *CachedPodRepo) IsOwner(ctx context.Context, id, userID string) (bool, error) {
	return r.repo.IsOwner(ctx, id, userID)
}

// GetOrganizationID returns the organization ID for a pod
func (r *CachedPodRepo) GetOrganizationID(ctx context.Context, id string) (*string, error) {
	return r.repo.GetOrganizationID(ctx, id)
}

// Verify interface compliance
var _ PodRepository = (*CachedPodRepo)(nil)

// CachedUserRepo wraps UserRepository with Redis caching
type CachedUserRepo struct {
	repo   UserRepository
	cache  *redisclient.UserCache
	logger *slog.Logger
}

// NewCachedUserRepo creates a new cached user repository
func NewCachedUserRepo(repo UserRepository, cache *redisclient.UserCache, logger *slog.Logger) *CachedUserRepo {
	if logger == nil {
		logger = slog.Default()
	}
	return &CachedUserRepo{
		repo:   repo,
		cache:  cache,
		logger: logger,
	}
}

// Create creates a new user
func (r *CachedUserRepo) Create(ctx context.Context, user *models.User) error {
	return r.repo.Create(ctx, user)
}

// GetByID retrieves a user by ID with caching
func (r *CachedUserRepo) GetByID(ctx context.Context, id string) (*models.User, error) {
	if r.cache != nil {
		var cached models.User
		if err := r.cache.GetUser(ctx, id, &cached); err == nil {
			r.logger.Debug("user cache hit", "id", id)
			return &cached, nil
		}
	}

	user, err := r.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, nil
	}

	// Cache the result
	if r.cache != nil {
		if err := r.cache.SetUser(ctx, id, user); err != nil {
			r.logger.Warn("failed to cache user", "id", id, "error", err)
		}
	}

	return user, nil
}

// GetByIDs retrieves multiple users by IDs (delegates to underlying repo without caching)
func (r *CachedUserRepo) GetByIDs(ctx context.Context, ids []string) ([]*models.User, error) {
	return r.repo.GetByIDs(ctx, ids)
}

// GetByExternalID retrieves a user by external ID with caching
func (r *CachedUserRepo) GetByExternalID(ctx context.Context, externalID string) (*models.User, error) {
	if r.cache != nil {
		var cached models.User
		if err := r.cache.GetUserByExternalID(ctx, externalID, &cached); err == nil {
			r.logger.Debug("user cache hit by external ID", "externalID", externalID)
			return &cached, nil
		}
	}

	user, err := r.repo.GetByExternalID(ctx, externalID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, nil
	}

	// Cache the result
	if r.cache != nil {
		if err := r.cache.SetUserByExternalID(ctx, externalID, user); err != nil {
			r.logger.Warn("failed to cache user by external ID", "externalID", externalID, "error", err)
		}
		if err := r.cache.SetUser(ctx, user.ID, user); err != nil {
			r.logger.Warn("failed to cache user by ID", "id", user.ID, "error", err)
		}
	}

	return user, nil
}

// GetByUsername retrieves a user by username (no caching - used for auth)
func (r *CachedUserRepo) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	return r.repo.GetByUsername(ctx, username)
}

// GetByEmail retrieves a user by email (no caching - used for auth)
func (r *CachedUserRepo) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	return r.repo.GetByEmail(ctx, email)
}

// GetOrCreateByUsername gets or creates a user by username
func (r *CachedUserRepo) GetOrCreateByUsername(ctx context.Context, username string) (*models.User, error) {
	return r.repo.GetOrCreateByUsername(ctx, username)
}

// Update updates a user and invalidates cache
func (r *CachedUserRepo) Update(ctx context.Context, user *models.User) error {
	if err := r.repo.Update(ctx, user); err != nil {
		return err
	}
	r.invalidateUser(ctx, user)
	return nil
}

// UpdateLastLogin updates the last login timestamp
func (r *CachedUserRepo) UpdateLastLogin(ctx context.Context, id string) error {
	if err := r.repo.UpdateLastLogin(ctx, id); err != nil {
		return err
	}
	if r.cache != nil {
		if err := r.cache.InvalidateUser(ctx, id); err != nil {
			r.logger.Warn("failed to invalidate user cache", "id", id, "error", err)
		}
	}
	return nil
}

// List lists users (not cached)
func (r *CachedUserRepo) List(ctx context.Context, opts UserListOptions) ([]*models.User, int, error) {
	return r.repo.List(ctx, opts)
}

// Delete deletes a user and invalidates cache
func (r *CachedUserRepo) Delete(ctx context.Context, id string) error {
	// Get user first to invalidate external ID cache
	user, _ := r.repo.GetByID(ctx, id)
	if err := r.repo.Delete(ctx, id); err != nil {
		return err
	}
	if user != nil {
		r.invalidateUser(ctx, user)
	} else if r.cache != nil {
		_ = r.cache.InvalidateUser(ctx, id)
	}
	return nil
}

// GetByEmailForAuth retrieves a user by email for authentication (no caching - security)
func (r *CachedUserRepo) GetByEmailForAuth(ctx context.Context, email string) (*models.User, error) {
	return r.repo.GetByEmailForAuth(ctx, email)
}

// GetByIDWithPassword retrieves a user by ID with password (no caching - security)
func (r *CachedUserRepo) GetByIDWithPassword(ctx context.Context, id string) (*models.User, error) {
	return r.repo.GetByIDWithPassword(ctx, id)
}

// UpdatePassword updates user's password (invalidate cache)
func (r *CachedUserRepo) UpdatePassword(ctx context.Context, id, passwordHash string, mustChange bool) error {
	if err := r.repo.UpdatePassword(ctx, id, passwordHash, mustChange); err != nil {
		return err
	}
	if r.cache != nil {
		if err := r.cache.InvalidateUser(ctx, id); err != nil {
			r.logger.Warn("failed to invalidate user cache", "id", id, "error", err)
		}
	}
	return nil
}

// ClearMustChangePassword clears the must_change_password flag
func (r *CachedUserRepo) ClearMustChangePassword(ctx context.Context, id string) error {
	if err := r.repo.ClearMustChangePassword(ctx, id); err != nil {
		return err
	}
	if r.cache != nil {
		if err := r.cache.InvalidateUser(ctx, id); err != nil {
			r.logger.Warn("failed to invalidate user cache", "id", id, "error", err)
		}
	}
	return nil
}

// GetPreferredLocale proxies to the underlying repo. The locale column
// isn't currently cached — reads are rare (login / profile view) and the
// column is nullable with no batch-fetch pressure.
func (r *CachedUserRepo) GetPreferredLocale(ctx context.Context, id string) (*string, error) {
	return r.repo.GetPreferredLocale(ctx, id)
}

// UpdatePreferredLocale proxies to the underlying repo and invalidates the
// user cache so downstream reads see the fresh value.
func (r *CachedUserRepo) UpdatePreferredLocale(ctx context.Context, id string, locale *string) error {
	if err := r.repo.UpdatePreferredLocale(ctx, id, locale); err != nil {
		return err
	}
	if r.cache != nil {
		if err := r.cache.InvalidateUser(ctx, id); err != nil {
			r.logger.Warn("failed to invalidate user cache", "id", id, "error", err)
		}
	}
	return nil
}

// invalidateUser removes user from all caches
func (r *CachedUserRepo) invalidateUser(ctx context.Context, user *models.User) {
	if r.cache == nil {
		return
	}
	if err := r.cache.InvalidateUser(ctx, user.ID); err != nil {
		r.logger.Warn("failed to invalidate user cache", "id", user.ID, "error", err)
	}
	// Note: We could also invalidate by external ID but UserCache doesn't support that
}

// Verify interface compliance
var _ UserRepository = (*CachedUserRepo)(nil)
