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

// CachedAchievementRepo wraps AchievementRepository with Redis caching
type CachedAchievementRepo struct {
	repo   AchievementRepository
	cache  *redisclient.AchievementCache
	logger *slog.Logger
}

// NewCachedAchievementRepo creates a new cached achievement repository
func NewCachedAchievementRepo(repo AchievementRepository, cache *redisclient.AchievementCache, logger *slog.Logger) *CachedAchievementRepo {
	if logger == nil {
		logger = slog.Default()
	}
	return &CachedAchievementRepo{
		repo:   repo,
		cache:  cache,
		logger: logger,
	}
}

// CreateAchievement creates a new achievement and invalidates caches
func (r *CachedAchievementRepo) CreateAchievement(ctx context.Context, achievement *models.Achievement) error {
	if err := r.repo.CreateAchievement(ctx, achievement); err != nil {
		return err
	}
	// Invalidate list caches since a new achievement was added
	if r.cache != nil {
		if err := r.cache.InvalidateAllAchievements(ctx); err != nil {
			r.logger.Warn("failed to invalidate achievement caches", "error", err)
		}
	}
	return nil
}

// GetAchievementByID retrieves an achievement by ID with caching
func (r *CachedAchievementRepo) GetAchievementByID(ctx context.Context, id string) (*models.Achievement, error) {
	if r.cache != nil {
		var cached models.Achievement
		if err := r.cache.GetAchievement(ctx, id, &cached); err == nil {
			r.logger.Debug("achievement cache hit", "id", id)
			return &cached, nil
		}
	}

	achievement, err := r.repo.GetAchievementByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if achievement == nil {
		return nil, nil
	}

	// Cache the result
	if r.cache != nil {
		if err := r.cache.SetAchievement(ctx, id, achievement); err != nil {
			r.logger.Warn("failed to cache achievement", "id", id, "error", err)
		}
	}

	return achievement, nil
}

// GetAchievementsByIDs retrieves multiple achievements by IDs (delegates to underlying repo without caching)
func (r *CachedAchievementRepo) GetAchievementsByIDs(ctx context.Context, ids []string) ([]*models.Achievement, error) {
	return r.repo.GetAchievementsByIDs(ctx, ids)
}

// ListAchievements retrieves achievements matching the filter with caching
func (r *CachedAchievementRepo) ListAchievements(ctx context.Context, filter AchievementFilter) ([]*models.Achievement, error) {
	cacheKey := hashAchievementFilter(filter)
	if r.cache != nil {
		var cached []*models.Achievement
		if err := r.cache.GetAchievementList(ctx, cacheKey, &cached); err == nil {
			r.logger.Debug("achievement list cache hit", "filter", cacheKey)
			return cached, nil
		}
	}

	achievements, err := r.repo.ListAchievements(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if r.cache != nil {
		if err := r.cache.SetAchievementList(ctx, cacheKey, achievements); err != nil {
			r.logger.Warn("failed to cache achievement list", "filter", cacheKey, "error", err)
		}
	}

	return achievements, nil
}

// UpdateAchievement updates an achievement and invalidates caches
func (r *CachedAchievementRepo) UpdateAchievement(ctx context.Context, achievement *models.Achievement) error {
	if err := r.repo.UpdateAchievement(ctx, achievement); err != nil {
		return err
	}
	r.invalidateAchievement(ctx, achievement.ID)
	return nil
}

// DeleteAchievement deletes an achievement and invalidates caches
func (r *CachedAchievementRepo) DeleteAchievement(ctx context.Context, id string) error {
	if err := r.repo.DeleteAchievement(ctx, id); err != nil {
		return err
	}
	r.invalidateAchievement(ctx, id)
	return nil
}

// AwardAchievement awards an achievement to a user and invalidates user caches
func (r *CachedAchievementRepo) AwardAchievement(ctx context.Context, userAchievement *models.UserAchievement) error {
	if err := r.repo.AwardAchievement(ctx, userAchievement); err != nil {
		return err
	}
	// Invalidate user-specific achievement caches
	if r.cache != nil {
		if err := r.cache.InvalidateUserAchievements(ctx, userAchievement.UserID); err != nil {
			r.logger.Warn("failed to invalidate user achievements cache", "userID", userAchievement.UserID, "error", err)
		}
	}
	return nil
}

// GetUserAchievement retrieves a user achievement (not cached - infrequent)
func (r *CachedAchievementRepo) GetUserAchievement(ctx context.Context, userID, achievementID string) (*models.UserAchievement, error) {
	return r.repo.GetUserAchievement(ctx, userID, achievementID)
}

// ListUserAchievements retrieves user achievements with caching
func (r *CachedAchievementRepo) ListUserAchievements(ctx context.Context, userID string) ([]*models.UserAchievement, error) {
	if r.cache != nil {
		var cached []*models.UserAchievement
		if err := r.cache.GetUserAchievements(ctx, userID, &cached); err == nil {
			r.logger.Debug("user achievements cache hit", "userID", userID)
			return cached, nil
		}
	}

	achievements, err := r.repo.ListUserAchievements(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if r.cache != nil {
		if err := r.cache.SetUserAchievements(ctx, userID, achievements); err != nil {
			r.logger.Warn("failed to cache user achievements", "userID", userID, "error", err)
		}
	}

	return achievements, nil
}

// GetUserAchievementSummary retrieves user achievement summary with caching
func (r *CachedAchievementRepo) GetUserAchievementSummary(ctx context.Context, userID string) (*models.UserAchievementSummary, error) {
	if r.cache != nil {
		var cached models.UserAchievementSummary
		if err := r.cache.GetUserAchievementSummary(ctx, userID, &cached); err == nil {
			r.logger.Debug("user achievement summary cache hit", "userID", userID)
			return &cached, nil
		}
	}

	summary, err := r.repo.GetUserAchievementSummary(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if r.cache != nil && summary != nil {
		if err := r.cache.SetUserAchievementSummary(ctx, userID, summary); err != nil {
			r.logger.Warn("failed to cache user achievement summary", "userID", userID, "error", err)
		}
	}

	return summary, nil
}

// GetRecentAchievements retrieves recent achievements (not cached - changes frequently)
func (r *CachedAchievementRepo) GetRecentAchievements(ctx context.Context, limit int) ([]*models.UserAchievement, error) {
	return r.repo.GetRecentAchievements(ctx, limit)
}

// MarkNotified marks an achievement as notified
func (r *CachedAchievementRepo) MarkNotified(ctx context.Context, userAchievementID string) error {
	return r.repo.MarkNotified(ctx, userAchievementID)
}

// UpdateProgress updates achievement progress and invalidates user caches
func (r *CachedAchievementRepo) UpdateProgress(ctx context.Context, progress *models.AchievementProgress) error {
	if err := r.repo.UpdateProgress(ctx, progress); err != nil {
		return err
	}
	// Invalidate user-specific caches since progress changed
	if r.cache != nil {
		if err := r.cache.InvalidateUserAchievements(ctx, progress.UserID); err != nil {
			r.logger.Warn("failed to invalidate user achievements cache", "userID", progress.UserID, "error", err)
		}
	}
	return nil
}

// GetProgress retrieves achievement progress (not cached - changes frequently)
func (r *CachedAchievementRepo) GetProgress(ctx context.Context, userID, achievementID string) (*models.AchievementProgress, error) {
	return r.repo.GetProgress(ctx, userID, achievementID)
}

// ListUserProgress retrieves user progress (not cached - changes frequently)
func (r *CachedAchievementRepo) ListUserProgress(ctx context.Context, userID string) ([]*models.AchievementProgress, error) {
	return r.repo.ListUserProgress(ctx, userID)
}

// ListAchievementsWithProgress retrieves achievements with progress with caching
func (r *CachedAchievementRepo) ListAchievementsWithProgress(ctx context.Context, userID string) ([]*models.AchievementWithProgress, error) {
	if r.cache != nil {
		var cached []*models.AchievementWithProgress
		if err := r.cache.GetAchievementsWithProgress(ctx, userID, &cached); err == nil {
			r.logger.Debug("achievements with progress cache hit", "userID", userID)
			return cached, nil
		}
	}

	achievements, err := r.repo.ListAchievementsWithProgress(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if r.cache != nil {
		if err := r.cache.SetAchievementsWithProgress(ctx, userID, achievements); err != nil {
			r.logger.Warn("failed to cache achievements with progress", "userID", userID, "error", err)
		}
	}

	return achievements, nil
}

// ListAllUserAchievements retrieves all user achievements (not cached - admin use)
func (r *CachedAchievementRepo) ListAllUserAchievements(ctx context.Context) ([]*models.UserAchievement, error) {
	return r.repo.ListAllUserAchievements(ctx)
}

// invalidateAchievement removes achievement from all caches
func (r *CachedAchievementRepo) invalidateAchievement(ctx context.Context, id string) {
	if r.cache == nil {
		return
	}

	if err := r.cache.InvalidateAchievement(ctx, id); err != nil {
		r.logger.Warn("failed to invalidate achievement cache", "id", id, "error", err)
	}
	// Invalidate list caches
	if err := r.cache.InvalidateAllAchievements(ctx); err != nil {
		r.logger.Warn("failed to invalidate achievement list caches", "error", err)
	}
}

// hashAchievementFilter creates a deterministic cache key from the filter
func hashAchievementFilter(filter AchievementFilter) string {
	data, _ := json.Marshal(filter)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:8]) // Use first 8 bytes
}

// GetLeaderboard retrieves the achievement leaderboard (no caching for real-time accuracy)
func (r *CachedAchievementRepo) GetLeaderboard(ctx context.Context, limit int, timeRange string) (*models.Leaderboard, error) {
	return r.repo.GetLeaderboard(ctx, limit, timeRange)
}

// Verify interface compliance
var _ AchievementRepository = (*CachedAchievementRepo)(nil)
