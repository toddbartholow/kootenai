package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// AchievementRepository defines operations for achievements
type AchievementRepository interface {
	// Achievement CRUD
	CreateAchievement(ctx context.Context, achievement *models.Achievement) error
	GetAchievementByID(ctx context.Context, id string) (*models.Achievement, error)
	GetAchievementsByIDs(ctx context.Context, ids []string) ([]*models.Achievement, error) // Batch fetch to avoid N+1 queries
	ListAchievements(ctx context.Context, filter AchievementFilter) ([]*models.Achievement, error)
	UpdateAchievement(ctx context.Context, achievement *models.Achievement) error
	DeleteAchievement(ctx context.Context, id string) error

	// User achievements
	AwardAchievement(ctx context.Context, userAchievement *models.UserAchievement) error
	GetUserAchievement(ctx context.Context, userID, achievementID string) (*models.UserAchievement, error)
	ListUserAchievements(ctx context.Context, userID string) ([]*models.UserAchievement, error)
	GetUserAchievementSummary(ctx context.Context, userID string) (*models.UserAchievementSummary, error)
	GetRecentAchievements(ctx context.Context, limit int) ([]*models.UserAchievement, error)
	MarkNotified(ctx context.Context, userAchievementID string) error

	// Progress tracking
	UpdateProgress(ctx context.Context, progress *models.AchievementProgress) error
	GetProgress(ctx context.Context, userID, achievementID string) (*models.AchievementProgress, error)
	ListUserProgress(ctx context.Context, userID string) ([]*models.AchievementProgress, error)

	// Combined views
	ListAchievementsWithProgress(ctx context.Context, userID string) ([]*models.AchievementWithProgress, error)

	// Analytics
	ListAllUserAchievements(ctx context.Context) ([]*models.UserAchievement, error)

	// Leaderboard
	GetLeaderboard(ctx context.Context, limit int, timeRange string) (*models.Leaderboard, error)
}

// AchievementFilter defines filters for achievement queries
type AchievementFilter struct {
	Type     models.AchievementType
	Tier     models.AchievementTier
	IsActive *bool
	IsSecret *bool
	Limit    int
	Offset   int
}

// AchievementRepo implements AchievementRepository
type AchievementRepo struct {
	db DBTX
}

// NewAchievementRepo creates a new achievement repository
func NewAchievementRepo(db DBTX) *AchievementRepo {
	return &AchievementRepo{db: db}
}

// CreateAchievement inserts a new achievement
func (r *AchievementRepo) CreateAchievement(ctx context.Context, achievement *models.Achievement) error {
	criteriaJSON, err := json.Marshal(achievement.Criteria)
	if err != nil {
		return fmt.Errorf("marshaling criteria: %w", err)
	}

	query := `
		INSERT INTO achievements (
			id, name, description, type, tier, icon_url, points,
			is_secret, is_active, criteria
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING created_at, updated_at`

	err = r.db.QueryRowContext(ctx, query,
		achievement.ID,
		achievement.Name,
		achievement.Description,
		achievement.Type,
		achievement.Tier,
		nullString(achievement.IconURL),
		achievement.Points,
		achievement.IsSecret,
		achievement.IsActive,
		criteriaJSON,
	).Scan(&achievement.CreatedAt, &achievement.UpdatedAt)

	if err != nil {
		return fmt.Errorf("inserting achievement: %w", err)
	}

	return nil
}

// GetAchievementByID retrieves an achievement by ID
func (r *AchievementRepo) GetAchievementByID(ctx context.Context, id string) (*models.Achievement, error) {
	query := `
		SELECT id, name, description, type, tier, icon_url, points,
		       is_secret, is_active, criteria, created_at, updated_at
		FROM achievements
		WHERE id = $1`

	achievement, err := r.scanAchievement(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return achievement, err
}

// GetAchievementsByIDs retrieves multiple achievements by their IDs in a single query
// This avoids N+1 queries when loading multiple achievements
func (r *AchievementRepo) GetAchievementsByIDs(ctx context.Context, ids []string) ([]*models.Achievement, error) {
	if len(ids) == 0 {
		return []*models.Achievement{}, nil
	}

	query := `
		SELECT id, name, description, type, tier, icon_url, points,
		       is_secret, is_active, criteria, created_at, updated_at
		FROM achievements
		WHERE id = ANY($1)`

	rows, err := r.db.QueryContext(ctx, query, pq.Array(ids))
	if err != nil {
		return nil, fmt.Errorf("querying achievements by ids: %w", err)
	}
	defer rows.Close()

	return r.scanMultipleAchievements(rows)
}

// ListAchievements retrieves achievements matching the filter
func (r *AchievementRepo) ListAchievements(ctx context.Context, filter AchievementFilter) ([]*models.Achievement, error) {
	qb := NewQueryBuilder(`
		SELECT id, name, description, type, tier, icon_url, points,
		       is_secret, is_active, criteria, created_at, updated_at
		FROM achievements
		WHERE 1=1`)

	if filter.Type != "" {
		qb.AddCondition("type = $%d", filter.Type)
	}
	if filter.Tier != "" {
		qb.AddCondition("tier = $%d", filter.Tier)
	}
	if filter.IsActive != nil {
		qb.AddCondition("is_active = $%d", *filter.IsActive)
	}
	if filter.IsSecret != nil {
		qb.AddCondition("is_secret = $%d", *filter.IsSecret)
	}

	qb.OrderByRaw("tier DESC, points DESC")
	qb.DefaultLimit(filter.Limit, 100)
	qb.Offset(filter.Offset)

	query, args := qb.Build()

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying achievements: %w", err)
	}
	defer rows.Close()

	return r.scanMultipleAchievements(rows)
}

// UpdateAchievement updates an achievement
func (r *AchievementRepo) UpdateAchievement(ctx context.Context, achievement *models.Achievement) error {
	criteriaJSON, err := json.Marshal(achievement.Criteria)
	if err != nil {
		return fmt.Errorf("marshaling criteria: %w", err)
	}

	query := `
		UPDATE achievements
		SET name = $2, description = $3, type = $4, tier = $5,
		    icon_url = $6, points = $7, is_secret = $8, is_active = $9,
		    criteria = $10
		WHERE id = $1`

	_, err = r.db.ExecContext(ctx, query,
		achievement.ID,
		achievement.Name,
		achievement.Description,
		achievement.Type,
		achievement.Tier,
		nullString(achievement.IconURL),
		achievement.Points,
		achievement.IsSecret,
		achievement.IsActive,
		criteriaJSON,
	)

	if err != nil {
		return fmt.Errorf("updating achievement: %w", err)
	}

	return nil
}

// DeleteAchievement deletes an achievement
func (r *AchievementRepo) DeleteAchievement(ctx context.Context, id string) error {
	query := `DELETE FROM achievements WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting achievement: %w", err)
	}
	return nil
}

// AwardAchievement records that a user earned an achievement
func (r *AchievementRepo) AwardAchievement(ctx context.Context, userAchievement *models.UserAchievement) error {
	query := `
		INSERT INTO user_achievements (
			id, user_id, achievement_id, earned_at, session_id, progress, notified
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id, achievement_id) DO NOTHING
		RETURNING created_at`

	err := r.db.QueryRowContext(ctx, query,
		userAchievement.ID,
		userAchievement.UserID,
		userAchievement.AchievementID,
		userAchievement.EarnedAt,
		nullString(userAchievement.SessionID),
		userAchievement.Progress,
		userAchievement.Notified,
	).Scan(&userAchievement.CreatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		// Already exists - this is ok, achievement already earned
		return nil
	}
	if err != nil {
		return fmt.Errorf("awarding achievement: %w", err)
	}

	return nil
}

// GetUserAchievement retrieves a specific user achievement
func (r *AchievementRepo) GetUserAchievement(ctx context.Context, userID, achievementID string) (*models.UserAchievement, error) {
	query := `
		SELECT id, user_id, achievement_id, earned_at, session_id, progress, notified, created_at
		FROM user_achievements
		WHERE user_id = $1 AND achievement_id = $2`

	ua, err := r.scanUserAchievement(r.db.QueryRowContext(ctx, query, userID, achievementID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return ua, err
}

// ListUserAchievements retrieves all achievements earned by a user
func (r *AchievementRepo) ListUserAchievements(ctx context.Context, userID string) ([]*models.UserAchievement, error) {
	query := `
		SELECT id, user_id, achievement_id, earned_at, session_id, progress, notified, created_at
		FROM user_achievements
		WHERE user_id = $1
		ORDER BY earned_at DESC`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("querying user achievements: %w", err)
	}
	defer rows.Close()

	return r.scanMultipleUserAchievements(rows)
}

// GetUserAchievementSummary retrieves achievement statistics for a user
func (r *AchievementRepo) GetUserAchievementSummary(ctx context.Context, userID string) (*models.UserAchievementSummary, error) {
	summary := &models.UserAchievementSummary{
		UserID:          userID,
		RarityBreakdown: make(map[models.AchievementTier]int),
		TypeBreakdown:   make(map[models.AchievementType]int),
	}

	// Get total available achievements
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM achievements WHERE is_active = true`).Scan(&summary.TotalAvailable)
	if err != nil {
		return nil, fmt.Errorf("counting available achievements: %w", err)
	}

	// Get user's earned achievements count and points
	query := `
		SELECT COUNT(ua.id), COALESCE(SUM(a.points), 0)
		FROM user_achievements ua
		JOIN achievements a ON ua.achievement_id = a.id
		WHERE ua.user_id = $1`

	err = r.db.QueryRowContext(ctx, query, userID).Scan(&summary.TotalEarned, &summary.TotalPoints)
	if err != nil {
		return nil, fmt.Errorf("counting earned achievements: %w", err)
	}

	// Calculate completion percentage
	if summary.TotalAvailable > 0 {
		summary.CompletionPercent = float64(summary.TotalEarned) / float64(summary.TotalAvailable) * 100
	}

	// Get recent achievements (last 5)
	recentQuery := `
		SELECT id, user_id, achievement_id, earned_at, session_id, progress, notified, created_at
		FROM user_achievements
		WHERE user_id = $1
		ORDER BY earned_at DESC
		LIMIT 5`

	rows, err := r.db.QueryContext(ctx, recentQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("querying recent achievements: %w", err)
	}
	defer rows.Close()

	summary.RecentAchievements, err = r.scanMultipleUserAchievements(rows)
	if err != nil {
		return nil, err
	}

	// Get rarity breakdown
	rarityQuery := `
		SELECT a.tier, COUNT(ua.id)
		FROM user_achievements ua
		JOIN achievements a ON ua.achievement_id = a.id
		WHERE ua.user_id = $1
		GROUP BY a.tier`

	rarityRows, err := r.db.QueryContext(ctx, rarityQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("querying rarity breakdown: %w", err)
	}
	defer rarityRows.Close()

	for rarityRows.Next() {
		var tier models.AchievementTier
		var count int
		if err := rarityRows.Scan(&tier, &count); err != nil {
			return nil, fmt.Errorf("scanning rarity: %w", err)
		}
		summary.RarityBreakdown[tier] = count
	}

	// Get type breakdown
	typeQuery := `
		SELECT a.type, COUNT(ua.id)
		FROM user_achievements ua
		JOIN achievements a ON ua.achievement_id = a.id
		WHERE ua.user_id = $1
		GROUP BY a.type`

	typeRows, err := r.db.QueryContext(ctx, typeQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("querying type breakdown: %w", err)
	}
	defer typeRows.Close()

	for typeRows.Next() {
		var achType models.AchievementType
		var count int
		if err := typeRows.Scan(&achType, &count); err != nil {
			return nil, fmt.Errorf("scanning type: %w", err)
		}
		summary.TypeBreakdown[achType] = count
	}

	return summary, nil
}

// GetRecentAchievements retrieves recently earned achievements across all users
func (r *AchievementRepo) GetRecentAchievements(ctx context.Context, limit int) ([]*models.UserAchievement, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT id, user_id, achievement_id, earned_at, session_id, progress, notified, created_at
		FROM user_achievements
		ORDER BY earned_at DESC
		LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("querying recent achievements: %w", err)
	}
	defer rows.Close()

	return r.scanMultipleUserAchievements(rows)
}

// MarkNotified marks a user achievement as notified
func (r *AchievementRepo) MarkNotified(ctx context.Context, userAchievementID string) error {
	query := `UPDATE user_achievements SET notified = true WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, userAchievementID)
	if err != nil {
		return fmt.Errorf("marking achievement notified: %w", err)
	}
	return nil
}

// UpdateProgress updates or creates achievement progress for a user
func (r *AchievementRepo) UpdateProgress(ctx context.Context, progress *models.AchievementProgress) error {
	metadataJSON, err := json.Marshal(progress.Metadata)
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	query := `
		INSERT INTO achievement_progress (
			user_id, achievement_id, progress, current_value, required_value, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, achievement_id) DO UPDATE
		SET progress = $3, current_value = $4, required_value = $5, metadata = $6
		RETURNING updated_at`

	err = r.db.QueryRowContext(ctx, query,
		progress.UserID,
		progress.AchievementID,
		progress.Progress,
		progress.Current,
		progress.Required,
		metadataJSON,
	).Scan(&progress.UpdatedAt)

	if err != nil {
		return fmt.Errorf("updating achievement progress: %w", err)
	}

	return nil
}

// GetProgress retrieves achievement progress for a user
func (r *AchievementRepo) GetProgress(ctx context.Context, userID, achievementID string) (*models.AchievementProgress, error) {
	query := `
		SELECT user_id, achievement_id, progress, current_value, required_value, metadata, updated_at
		FROM achievement_progress
		WHERE user_id = $1 AND achievement_id = $2`

	progress, err := r.scanProgress(r.db.QueryRowContext(ctx, query, userID, achievementID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return progress, err
}

// ListUserProgress retrieves all achievement progress for a user
func (r *AchievementRepo) ListUserProgress(ctx context.Context, userID string) ([]*models.AchievementProgress, error) {
	query := `
		SELECT user_id, achievement_id, progress, current_value, required_value, metadata, updated_at
		FROM achievement_progress
		WHERE user_id = $1
		ORDER BY progress DESC`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("querying user progress: %w", err)
	}
	defer rows.Close()

	return r.scanMultipleProgress(rows)
}

// ListAchievementsWithProgress retrieves all achievements with user progress
func (r *AchievementRepo) ListAchievementsWithProgress(ctx context.Context, userID string) ([]*models.AchievementWithProgress, error) {
	query := `
		SELECT
			a.id, a.name, a.description, a.type, a.tier, a.icon_url, a.points,
			a.is_secret, a.is_active, a.criteria, a.created_at, a.updated_at,
			ua.earned_at,
			ap.progress, ap.current_value, ap.required_value, ap.metadata, ap.updated_at
		FROM achievements a
		LEFT JOIN user_achievements ua ON a.id = ua.achievement_id AND ua.user_id = $1
		LEFT JOIN achievement_progress ap ON a.id = ap.achievement_id AND ap.user_id = $1
		WHERE a.is_active = true
		ORDER BY
			CASE WHEN ua.earned_at IS NOT NULL THEN 0 ELSE 1 END,
			a.tier DESC,
			a.points DESC`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("querying achievements with progress: %w", err)
	}
	defer rows.Close()

	var results []*models.AchievementWithProgress
	for rows.Next() {
		awp := &models.AchievementWithProgress{}
		var earnedAt sql.NullTime
		var iconURL sql.NullString
		var criteriaJSON []byte

		// Progress fields (nullable)
		var progressPct sql.NullFloat64
		var currentVal, requiredVal sql.NullInt64
		var metadataJSON []byte
		var progressUpdated sql.NullTime

		err := rows.Scan(
			&awp.ID, &awp.Name, &awp.Description, &awp.Type, &awp.Tier,
			&iconURL, &awp.Points, &awp.IsSecret, &awp.IsActive,
			&criteriaJSON, &awp.CreatedAt, &awp.UpdatedAt,
			&earnedAt,
			&progressPct, &currentVal, &requiredVal, &metadataJSON, &progressUpdated,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning achievement with progress: %w", err)
		}

		awp.IconURL = iconURL.String
		if err := json.Unmarshal(criteriaJSON, &awp.Criteria); err != nil {
			return nil, fmt.Errorf("unmarshaling criteria: %w", err)
		}

		if earnedAt.Valid {
			awp.Earned = true
			awp.EarnedAt = &earnedAt.Time
		}

		if progressPct.Valid {
			awp.Progress = &models.AchievementProgress{
				UserID:        userID,
				AchievementID: awp.ID,
				Progress:      progressPct.Float64,
				Current:       int(currentVal.Int64),
				Required:      int(requiredVal.Int64),
				Earned:        earnedAt.Valid,
				UpdatedAt:     progressUpdated.Time,
			}
			if earnedAt.Valid {
				awp.Progress.EarnedAt = &earnedAt.Time
			}
			if len(metadataJSON) > 0 {
				_ = json.Unmarshal(metadataJSON, &awp.Progress.Metadata)
			}
		}

		results = append(results, awp)
	}

	return results, rows.Err()
}

// Helper methods

func (r *AchievementRepo) scanAchievement(row scannable) (*models.Achievement, error) {
	var achievement models.Achievement
	var iconURL sql.NullString
	var criteriaJSON []byte

	err := row.Scan(
		&achievement.ID,
		&achievement.Name,
		&achievement.Description,
		&achievement.Type,
		&achievement.Tier,
		&iconURL,
		&achievement.Points,
		&achievement.IsSecret,
		&achievement.IsActive,
		&criteriaJSON,
		&achievement.CreatedAt,
		&achievement.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("scanning achievement: %w", err)
	}

	achievement.IconURL = iconURL.String
	if err := json.Unmarshal(criteriaJSON, &achievement.Criteria); err != nil {
		return nil, fmt.Errorf("unmarshaling criteria: %w", err)
	}

	return &achievement, nil
}

func (r *AchievementRepo) scanMultipleAchievements(rows *sql.Rows) ([]*models.Achievement, error) {
	var achievements []*models.Achievement
	for rows.Next() {
		achievement, err := r.scanAchievementRow(rows)
		if err != nil {
			return nil, err
		}
		achievements = append(achievements, achievement)
	}
	return achievements, rows.Err()
}

func (r *AchievementRepo) scanAchievementRow(rows *sql.Rows) (*models.Achievement, error) {
	return r.scanAchievement(rows)
}

func (r *AchievementRepo) scanUserAchievement(row scannable) (*models.UserAchievement, error) {
	var ua models.UserAchievement
	var sessionID sql.NullString

	err := row.Scan(
		&ua.ID,
		&ua.UserID,
		&ua.AchievementID,
		&ua.EarnedAt,
		&sessionID,
		&ua.Progress,
		&ua.Notified,
		&ua.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("scanning user achievement: %w", err)
	}

	ua.SessionID = sessionID.String
	return &ua, nil
}

func (r *AchievementRepo) scanMultipleUserAchievements(rows *sql.Rows) ([]*models.UserAchievement, error) {
	var userAchievements []*models.UserAchievement
	for rows.Next() {
		ua, err := r.scanUserAchievement(rows)
		if err != nil {
			return nil, err
		}
		userAchievements = append(userAchievements, ua)
	}
	return userAchievements, rows.Err()
}

func (r *AchievementRepo) scanProgress(row scannable) (*models.AchievementProgress, error) {
	var progress models.AchievementProgress
	var metadataJSON []byte

	err := row.Scan(
		&progress.UserID,
		&progress.AchievementID,
		&progress.Progress,
		&progress.Current,
		&progress.Required,
		&metadataJSON,
		&progress.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("scanning progress: %w", err)
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &progress.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshaling metadata: %w", err)
		}
	}

	return &progress, nil
}

func (r *AchievementRepo) scanMultipleProgress(rows *sql.Rows) ([]*models.AchievementProgress, error) {
	var progressList []*models.AchievementProgress
	for rows.Next() {
		progress, err := r.scanProgress(rows)
		if err != nil {
			return nil, err
		}
		progressList = append(progressList, progress)
	}
	return progressList, rows.Err()
}

// ListAllUserAchievements retrieves all user achievements across all users (for leaderboard)
func (r *AchievementRepo) ListAllUserAchievements(ctx context.Context) ([]*models.UserAchievement, error) {
	query := `
		SELECT id, user_id, achievement_id, earned_at, session_id, progress, notified, created_at
		FROM user_achievements
		ORDER BY earned_at DESC
		LIMIT 10000`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying all user achievements: %w", err)
	}
	defer rows.Close()

	return r.scanMultipleUserAchievements(rows)
}

// GetLeaderboard retrieves the achievement leaderboard
func (r *AchievementRepo) GetLeaderboard(ctx context.Context, limit int, timeRange string) (*models.Leaderboard, error) {
	if limit <= 0 {
		limit = 50
	}

	// Build time constraint
	timeConstraint := ""
	switch timeRange {
	case "week":
		timeConstraint = "AND ua.earned_at >= NOW() - INTERVAL '7 days'"
	case "month":
		timeConstraint = "AND ua.earned_at >= NOW() - INTERVAL '30 days'"
	default:
		timeRange = "all"
	}

	// #nosec G201 -- timeConstraint is from a switch statement with only hardcoded
	// values ("week", "month", or empty string). No user input is interpolated.
	query := fmt.Sprintf(`
		WITH user_stats AS (
			SELECT
				ua.user_id,
				COALESCE(SUM(a.points), 0) as total_points,
				COUNT(*) as total_achievements,
				COUNT(*) FILTER (WHERE a.tier = 'bronze') as bronze_count,
				COUNT(*) FILTER (WHERE a.tier = 'silver') as silver_count,
				COUNT(*) FILTER (WHERE a.tier = 'gold') as gold_count,
				COUNT(*) FILTER (WHERE a.tier = 'platinum') as platinum_count,
				COUNT(*) FILTER (WHERE a.tier = 'diamond') as diamond_count,
				MAX(ua.earned_at) as last_earned_at
			FROM user_achievements ua
			JOIN achievements a ON ua.achievement_id = a.id
			WHERE 1=1 %s
			GROUP BY ua.user_id
		)
		SELECT
			us.user_id,
			u.username,
			COALESCE(u.display_name, u.username) as display_name,
			us.total_points,
			us.total_achievements,
			us.bronze_count,
			us.silver_count,
			us.gold_count,
			us.platinum_count,
			us.diamond_count,
			us.last_earned_at
		FROM user_stats us
		JOIN users u ON us.user_id = u.id
		ORDER BY us.total_points DESC, us.total_achievements DESC
		LIMIT $1`, timeConstraint)

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("querying leaderboard: %w", err)
	}
	defer rows.Close()

	var entries []*models.LeaderboardEntry
	rank := 0
	for rows.Next() {
		rank++
		entry := &models.LeaderboardEntry{Rank: rank}
		var lastEarned sql.NullTime

		err := rows.Scan(
			&entry.UserID,
			&entry.Username,
			&entry.DisplayName,
			&entry.TotalPoints,
			&entry.TotalAchievements,
			&entry.BronzeCount,
			&entry.SilverCount,
			&entry.GoldCount,
			&entry.PlatinumCount,
			&entry.DiamondCount,
			&lastEarned,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning leaderboard entry: %w", err)
		}

		if lastEarned.Valid {
			entry.LastEarnedAt = lastEarned.Time
		}

		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating leaderboard rows: %w", err)
	}

	// Get total user count
	var totalUsers int
	countQuery := fmt.Sprintf(`
		SELECT COUNT(DISTINCT ua.user_id)
		FROM user_achievements ua
		WHERE 1=1 %s`, timeConstraint)
	if err := r.db.QueryRowContext(ctx, countQuery).Scan(&totalUsers); err != nil {
		return nil, fmt.Errorf("counting leaderboard users: %w", err)
	}

	return &models.Leaderboard{
		Entries:    entries,
		TotalUsers: totalUsers,
		UpdatedAt:  time.Now(),
		TimeRange:  timeRange,
	}, nil
}
