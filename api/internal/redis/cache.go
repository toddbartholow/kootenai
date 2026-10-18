package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache provides a generic caching layer backed by Redis
type Cache struct {
	client     *Client
	defaultTTL time.Duration
	logger     *slog.Logger
}

// NewCache creates a new cache instance
func NewCache(client *Client, defaultTTL time.Duration, logger *slog.Logger) *Cache {
	if defaultTTL == 0 {
		defaultTTL = DefaultCacheTTL
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Cache{
		client:     client,
		defaultTTL: defaultTTL,
		logger:     logger,
	}
}

// Get retrieves a cached value and unmarshals it into the destination
func (c *Cache) Get(ctx context.Context, key string, dest any) error {
	data, err := c.client.GetBytes(ctx, PrefixCache+key)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return ErrCacheMiss
		}
		return fmt.Errorf("cache get failed: %w", err)
	}

	if err := json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("cache unmarshal failed: %w", err)
	}

	return nil
}

// Set caches a value with the default TTL
func (c *Cache) Set(ctx context.Context, key string, value any) error {
	return c.SetWithTTL(ctx, key, value, c.defaultTTL)
}

// SetWithTTL caches a value with a custom TTL
func (c *Cache) SetWithTTL(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("cache marshal failed: %w", err)
	}

	if err := c.client.Set(ctx, PrefixCache+key, data, ttl); err != nil {
		return fmt.Errorf("cache set failed: %w", err)
	}

	return nil
}

// Delete removes a value from the cache
func (c *Cache) Delete(ctx context.Context, keys ...string) error {
	prefixedKeys := make([]string, len(keys))
	for i, k := range keys {
		prefixedKeys[i] = PrefixCache + k
	}
	return c.client.Delete(ctx, prefixedKeys...)
}

// GetOrSet retrieves from cache or executes fn and caches the result
func (c *Cache) GetOrSet(ctx context.Context, key string, dest any, ttl time.Duration, fn func() (any, error)) error {
	// Try to get from cache first
	if err := c.Get(ctx, key, dest); err == nil {
		c.logger.Debug("cache hit", slog.String("key", key))
		return nil
	}

	c.logger.Debug("cache miss", slog.String("key", key))

	// Execute the function
	value, err := fn()
	if err != nil {
		return err
	}

	// Cache the result
	if err := c.SetWithTTL(ctx, key, value, ttl); err != nil {
		c.logger.Warn("failed to cache value", slog.String("key", key), slog.Any("error", err))
	}

	// Copy the value to dest
	data, _ := json.Marshal(value)
	return json.Unmarshal(data, dest)
}

// Invalidate removes cached values matching a pattern
func (c *Cache) Invalidate(ctx context.Context, pattern string) error {
	fullPattern := c.client.Key(PrefixCache + pattern)

	var cursor uint64
	var err error

	for {
		var keys []string
		keys, cursor, err = c.client.rdb.Scan(ctx, cursor, fullPattern, 100).Result()
		if err != nil {
			return fmt.Errorf("cache invalidation scan failed: %w", err)
		}

		if len(keys) > 0 {
			if err := c.client.rdb.Del(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("cache invalidation delete failed: %w", err)
			}
		}

		if cursor == 0 {
			break
		}
	}

	return nil
}

// ErrCacheMiss indicates the key was not found in cache
var ErrCacheMiss = errors.New("cache miss")

// PermissionCache provides caching for RBAC permission lookups
type PermissionCache struct {
	cache *Cache
	ttl   time.Duration
}

// NewPermissionCache creates a permission-specific cache
func NewPermissionCache(client *Client, logger *slog.Logger) *PermissionCache {
	ttl := 5 * time.Minute
	return &PermissionCache{
		cache: NewCache(client, ttl, logger),
		ttl:   ttl,
	}
}

// GetUserPermissions retrieves cached permissions for a user in an organization
func (pc *PermissionCache) GetUserPermissions(ctx context.Context, userID string, orgID *string) ([]string, error) {
	var perms []string
	if err := pc.cache.Get(ctx, pc.key(userID, orgID), &perms); err != nil {
		return nil, err
	}
	return perms, nil
}

// SetUserPermissions caches permissions for a user in an organization
func (pc *PermissionCache) SetUserPermissions(ctx context.Context, userID string, orgID *string, perms []string) error {
	return pc.cache.SetWithTTL(ctx, pc.key(userID, orgID), perms, pc.ttl)
}

// InvalidateUser removes all cached permissions for a user
func (pc *PermissionCache) InvalidateUser(ctx context.Context, userID string) error {
	return pc.cache.Invalidate(ctx, "perms:"+userID+":*")
}

// InvalidateAll removes all permission caches
func (pc *PermissionCache) InvalidateAll(ctx context.Context) error {
	return pc.cache.Invalidate(ctx, "perms:*")
}

func (pc *PermissionCache) key(userID string, orgID *string) string {
	org := "global"
	if orgID != nil {
		org = *orgID
	}
	return "perms:" + userID + ":" + org
}

// --- Specialized Cache Types ---

// TemplateCache provides caching for lab templates
type TemplateCache struct {
	cache *Cache
	ttl   time.Duration
}

// NewTemplateCache creates a template-specific cache
func NewTemplateCache(client *Client, logger *slog.Logger) *TemplateCache {
	return &TemplateCache{
		cache: NewCache(client, DefaultTemplateTTL, logger),
		ttl:   DefaultTemplateTTL,
	}
}

// GetTemplate retrieves a cached template
func (tc *TemplateCache) GetTemplate(ctx context.Context, templateID string, dest any) error {
	return tc.cache.Get(ctx, "template:"+templateID, dest)
}

// SetTemplate caches a template
func (tc *TemplateCache) SetTemplate(ctx context.Context, templateID string, template any) error {
	return tc.cache.SetWithTTL(ctx, "template:"+templateID, template, tc.ttl)
}

// InvalidateTemplate removes a template from cache
func (tc *TemplateCache) InvalidateTemplate(ctx context.Context, templateID string) error {
	return tc.cache.Delete(ctx, "template:"+templateID)
}

// InvalidateAll removes all cached templates
func (tc *TemplateCache) InvalidateAll(ctx context.Context) error {
	return tc.cache.Invalidate(ctx, "template:*")
}

// GetTemplateList retrieves a cached template list
func (tc *TemplateCache) GetTemplateList(ctx context.Context, cacheKey string, dest any) error {
	return tc.cache.Get(ctx, "templates:"+cacheKey, dest)
}

// SetTemplateList caches a template list
func (tc *TemplateCache) SetTemplateList(ctx context.Context, cacheKey string, templates any) error {
	return tc.cache.SetWithTTL(ctx, "templates:"+cacheKey, templates, tc.ttl)
}

// PodCache provides caching for pod status and information
type PodCache struct {
	cache *Cache
	ttl   time.Duration
}

// NewPodCache creates a pod-specific cache
func NewPodCache(client *Client, logger *slog.Logger) *PodCache {
	return &PodCache{
		cache: NewCache(client, DefaultPodStatusTTL, logger),
		ttl:   DefaultPodStatusTTL,
	}
}

// GetPodStatus retrieves cached pod status
func (pc *PodCache) GetPodStatus(ctx context.Context, podID string, dest any) error {
	return pc.cache.Get(ctx, "pod:status:"+podID, dest)
}

// SetPodStatus caches pod status
func (pc *PodCache) SetPodStatus(ctx context.Context, podID string, status any) error {
	return pc.cache.SetWithTTL(ctx, "pod:status:"+podID, status, pc.ttl)
}

// InvalidatePodStatus removes pod status from cache
func (pc *PodCache) InvalidatePodStatus(ctx context.Context, podID string) error {
	return pc.cache.Delete(ctx, "pod:status:"+podID)
}

// GetPod retrieves a cached pod
func (pc *PodCache) GetPod(ctx context.Context, podID string, dest any) error {
	return pc.cache.Get(ctx, "pod:"+podID, dest)
}

// SetPod caches a pod
func (pc *PodCache) SetPod(ctx context.Context, podID string, pod any) error {
	return pc.cache.SetWithTTL(ctx, "pod:"+podID, pod, 5*time.Minute)
}

// InvalidatePod removes a pod from cache
func (pc *PodCache) InvalidatePod(ctx context.Context, podID string) error {
	return pc.cache.Delete(ctx, "pod:"+podID, "pod:status:"+podID)
}

// UserCache provides caching for user data
type UserCache struct {
	cache *Cache
	ttl   time.Duration
}

// NewUserCache creates a user-specific cache
func NewUserCache(client *Client, logger *slog.Logger) *UserCache {
	return &UserCache{
		cache: NewCache(client, 15*time.Minute, logger),
		ttl:   15 * time.Minute,
	}
}

// GetUser retrieves a cached user
func (uc *UserCache) GetUser(ctx context.Context, userID string, dest any) error {
	return uc.cache.Get(ctx, "user:"+userID, dest)
}

// SetUser caches a user
func (uc *UserCache) SetUser(ctx context.Context, userID string, user any) error {
	return uc.cache.SetWithTTL(ctx, "user:"+userID, user, uc.ttl)
}

// InvalidateUser removes a user from cache
func (uc *UserCache) InvalidateUser(ctx context.Context, userID string) error {
	return uc.cache.Delete(ctx, "user:"+userID)
}

// GetUserByExternalID retrieves a cached user by external ID
func (uc *UserCache) GetUserByExternalID(ctx context.Context, externalID string, dest any) error {
	return uc.cache.Get(ctx, "user:ext:"+externalID, dest)
}

// SetUserByExternalID caches a user by external ID
func (uc *UserCache) SetUserByExternalID(ctx context.Context, externalID string, user any) error {
	return uc.cache.SetWithTTL(ctx, "user:ext:"+externalID, user, uc.ttl)
}

// GetUserRoles retrieves cached user roles
func (uc *UserCache) GetUserRoles(ctx context.Context, userID string) ([]string, error) {
	var roles []string
	if err := uc.cache.Get(ctx, "user:roles:"+userID, &roles); err != nil {
		return nil, err
	}
	return roles, nil
}

// SetUserRoles caches user roles
func (uc *UserCache) SetUserRoles(ctx context.Context, userID string, roles []string) error {
	return uc.cache.SetWithTTL(ctx, "user:roles:"+userID, roles, uc.ttl)
}

// AchievementCache provides caching for achievements
type AchievementCache struct {
	cache *Cache
	ttl   time.Duration
}

// NewAchievementCache creates an achievement-specific cache
func NewAchievementCache(client *Client, logger *slog.Logger) *AchievementCache {
	// Achievements rarely change, so use a longer TTL (30 minutes)
	ttl := 30 * time.Minute
	return &AchievementCache{
		cache: NewCache(client, ttl, logger),
		ttl:   ttl,
	}
}

// GetAchievement retrieves a cached achievement
func (ac *AchievementCache) GetAchievement(ctx context.Context, id string, dest any) error {
	return ac.cache.Get(ctx, "achievement:"+id, dest)
}

// SetAchievement caches an achievement
func (ac *AchievementCache) SetAchievement(ctx context.Context, id string, achievement any) error {
	return ac.cache.SetWithTTL(ctx, "achievement:"+id, achievement, ac.ttl)
}

// InvalidateAchievement removes an achievement from cache
func (ac *AchievementCache) InvalidateAchievement(ctx context.Context, id string) error {
	return ac.cache.Delete(ctx, "achievement:"+id)
}

// GetAchievementList retrieves a cached achievement list
func (ac *AchievementCache) GetAchievementList(ctx context.Context, cacheKey string, dest any) error {
	return ac.cache.Get(ctx, "achievements:"+cacheKey, dest)
}

// SetAchievementList caches an achievement list
func (ac *AchievementCache) SetAchievementList(ctx context.Context, cacheKey string, achievements any) error {
	return ac.cache.SetWithTTL(ctx, "achievements:"+cacheKey, achievements, ac.ttl)
}

// InvalidateAchievementList removes achievement list cache
func (ac *AchievementCache) InvalidateAchievementList(ctx context.Context, pattern string) error {
	return ac.cache.Invalidate(ctx, "achievements:"+pattern+"*")
}

// InvalidateAllAchievements removes all achievement caches
func (ac *AchievementCache) InvalidateAllAchievements(ctx context.Context) error {
	return ac.cache.Invalidate(ctx, "achievement*")
}

// GetUserAchievements retrieves cached user achievements
func (ac *AchievementCache) GetUserAchievements(ctx context.Context, userID string, dest any) error {
	return ac.cache.Get(ctx, "user:achievements:"+userID, dest)
}

// SetUserAchievements caches user achievements
func (ac *AchievementCache) SetUserAchievements(ctx context.Context, userID string, achievements any) error {
	// User achievements change more often, use shorter TTL (5 minutes)
	return ac.cache.SetWithTTL(ctx, "user:achievements:"+userID, achievements, 5*time.Minute)
}

// InvalidateUserAchievements removes user achievements from cache
func (ac *AchievementCache) InvalidateUserAchievements(ctx context.Context, userID string) error {
	return ac.cache.Delete(ctx, "user:achievements:"+userID, "user:achievement:summary:"+userID, "user:achievements:progress:"+userID)
}

// GetUserAchievementSummary retrieves cached user achievement summary
func (ac *AchievementCache) GetUserAchievementSummary(ctx context.Context, userID string, dest any) error {
	return ac.cache.Get(ctx, "user:achievement:summary:"+userID, dest)
}

// SetUserAchievementSummary caches user achievement summary
func (ac *AchievementCache) SetUserAchievementSummary(ctx context.Context, userID string, summary any) error {
	return ac.cache.SetWithTTL(ctx, "user:achievement:summary:"+userID, summary, 5*time.Minute)
}

// GetAchievementsWithProgress retrieves cached achievements with progress for a user
func (ac *AchievementCache) GetAchievementsWithProgress(ctx context.Context, userID string, dest any) error {
	return ac.cache.Get(ctx, "user:achievements:progress:"+userID, dest)
}

// SetAchievementsWithProgress caches achievements with progress for a user
func (ac *AchievementCache) SetAchievementsWithProgress(ctx context.Context, userID string, data any) error {
	return ac.cache.SetWithTTL(ctx, "user:achievements:progress:"+userID, data, 5*time.Minute)
}

// DashboardCache provides caching for dashboard data
type DashboardCache struct {
	cache *Cache
	ttl   time.Duration
}

// NewDashboardCache creates a dashboard-specific cache
func NewDashboardCache(client *Client, logger *slog.Logger) *DashboardCache {
	// Dashboard data changes frequently but can tolerate 2-minute staleness
	ttl := 2 * time.Minute
	return &DashboardCache{
		cache: NewCache(client, ttl, logger),
		ttl:   ttl,
	}
}

// GetUserStats retrieves cached user statistics
func (dc *DashboardCache) GetUserStats(ctx context.Context, userID string, dest any) error {
	return dc.cache.Get(ctx, "dashboard:stats:"+userID, dest)
}

// SetUserStats caches user statistics
func (dc *DashboardCache) SetUserStats(ctx context.Context, userID string, stats any) error {
	return dc.cache.SetWithTTL(ctx, "dashboard:stats:"+userID, stats, dc.ttl)
}

// GetDashboard retrieves cached full dashboard data
func (dc *DashboardCache) GetDashboard(ctx context.Context, userID string, dest any) error {
	return dc.cache.Get(ctx, "dashboard:full:"+userID, dest)
}

// SetDashboard caches full dashboard data
func (dc *DashboardCache) SetDashboard(ctx context.Context, userID string, data any) error {
	return dc.cache.SetWithTTL(ctx, "dashboard:full:"+userID, data, dc.ttl)
}

// InvalidateUserDashboard removes user dashboard data from cache
func (dc *DashboardCache) InvalidateUserDashboard(ctx context.Context, userID string) error {
	return dc.cache.Delete(ctx, "dashboard:stats:"+userID, "dashboard:full:"+userID)
}

// GetLeaderboard retrieves cached leaderboard data
func (dc *DashboardCache) GetLeaderboard(ctx context.Context, limit int, dest any) error {
	return dc.cache.Get(ctx, fmt.Sprintf("leaderboard:%d", limit), dest)
}

// SetLeaderboard caches leaderboard data
func (dc *DashboardCache) SetLeaderboard(ctx context.Context, limit int, data any) error {
	// Leaderboard can be cached longer (5 minutes)
	return dc.cache.SetWithTTL(ctx, fmt.Sprintf("leaderboard:%d", limit), data, 5*time.Minute)
}

// InvalidateLeaderboard removes all leaderboard caches
func (dc *DashboardCache) InvalidateLeaderboard(ctx context.Context) error {
	return dc.cache.Invalidate(ctx, "leaderboard:*")
}

// PathwayCache provides caching for pathway and enrollment data
type PathwayCache struct {
	cache *Cache
	ttl   time.Duration
}

// NewPathwayCache creates a pathway-specific cache
func NewPathwayCache(client *Client, logger *slog.Logger) *PathwayCache {
	ttl := 10 * time.Minute
	return &PathwayCache{
		cache: NewCache(client, ttl, logger),
		ttl:   ttl,
	}
}

// GetPathway retrieves a cached pathway
func (pc *PathwayCache) GetPathway(ctx context.Context, id string, dest any) error {
	return pc.cache.Get(ctx, "pathway:"+id, dest)
}

// SetPathway caches a pathway
func (pc *PathwayCache) SetPathway(ctx context.Context, id string, pathway any) error {
	return pc.cache.SetWithTTL(ctx, "pathway:"+id, pathway, pc.ttl)
}

// GetPathwayBySlug retrieves a cached pathway by slug
func (pc *PathwayCache) GetPathwayBySlug(ctx context.Context, slug string, dest any) error {
	return pc.cache.Get(ctx, "pathway:slug:"+slug, dest)
}

// SetPathwayBySlug caches a pathway by slug
func (pc *PathwayCache) SetPathwayBySlug(ctx context.Context, slug string, pathway any) error {
	return pc.cache.SetWithTTL(ctx, "pathway:slug:"+slug, pathway, pc.ttl)
}

// InvalidatePathway removes a pathway from cache
func (pc *PathwayCache) InvalidatePathway(ctx context.Context, id, slug string) error {
	keys := []string{"pathway:" + id}
	if slug != "" {
		keys = append(keys, "pathway:slug:"+slug)
	}
	return pc.cache.Delete(ctx, keys...)
}

// GetUserEnrollments retrieves cached user enrollments
func (pc *PathwayCache) GetUserEnrollments(ctx context.Context, userID string, dest any) error {
	return pc.cache.Get(ctx, "enrollments:user:"+userID, dest)
}

// SetUserEnrollments caches user enrollments
func (pc *PathwayCache) SetUserEnrollments(ctx context.Context, userID string, enrollments any) error {
	// Enrollments change more often, shorter TTL
	return pc.cache.SetWithTTL(ctx, "enrollments:user:"+userID, enrollments, 3*time.Minute)
}

// InvalidateUserEnrollments removes user enrollments from cache
func (pc *PathwayCache) InvalidateUserEnrollments(ctx context.Context, userID string) error {
	return pc.cache.Delete(ctx, "enrollments:user:"+userID)
}

// GetEnrollmentProgress retrieves cached enrollment progress
func (pc *PathwayCache) GetEnrollmentProgress(ctx context.Context, enrollmentID string, dest any) error {
	return pc.cache.Get(ctx, "enrollment:progress:"+enrollmentID, dest)
}

// SetEnrollmentProgress caches enrollment progress
func (pc *PathwayCache) SetEnrollmentProgress(ctx context.Context, enrollmentID string, progress any) error {
	return pc.cache.SetWithTTL(ctx, "enrollment:progress:"+enrollmentID, progress, 2*time.Minute)
}

// InvalidateEnrollmentProgress removes enrollment progress from cache
func (pc *PathwayCache) InvalidateEnrollmentProgress(ctx context.Context, enrollmentID string) error {
	return pc.cache.Delete(ctx, "enrollment:progress:"+enrollmentID)
}
