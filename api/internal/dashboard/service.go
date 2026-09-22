// Package dashboard provides the service layer for dashboard operations.
package dashboard

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/achievements"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// Service handles dashboard business logic.
type Service struct {
	sessionRepo     repositories.SessionRepository
	labTemplateRepo repositories.LabTemplateRepository
	pathwayRepo     repositories.PathwayRepository
	enrollmentRepo  repositories.EnrollmentRepository
	achievementRepo repositories.AchievementRepository
	achievementSvc  *achievements.Service
	userRepo        repositories.UserRepository
	logger          *slog.Logger
}

// NewService creates a new dashboard service.
func NewService(logger *slog.Logger) *Service {
	return &Service{
		logger: logger,
	}
}

// Builder pattern for optional dependencies

// WithSessionRepo sets the session repository.
func (s *Service) WithSessionRepo(repo repositories.SessionRepository) *Service {
	s.sessionRepo = repo
	return s
}

// WithLabTemplateRepo sets the lab template repository.
func (s *Service) WithLabTemplateRepo(repo repositories.LabTemplateRepository) *Service {
	s.labTemplateRepo = repo
	return s
}

// WithPathwayRepo sets the pathway repository.
func (s *Service) WithPathwayRepo(repo repositories.PathwayRepository) *Service {
	s.pathwayRepo = repo
	return s
}

// WithEnrollmentRepo sets the enrollment repository.
func (s *Service) WithEnrollmentRepo(repo repositories.EnrollmentRepository) *Service {
	s.enrollmentRepo = repo
	return s
}

// WithAchievementRepo sets the achievement repository.
func (s *Service) WithAchievementRepo(repo repositories.AchievementRepository) *Service {
	s.achievementRepo = repo
	return s
}

// WithAchievementService sets the achievement service.
func (s *Service) WithAchievementService(svc *achievements.Service) *Service {
	s.achievementSvc = svc
	return s
}

// WithUserRepo sets the user repository.
func (s *Service) WithUserRepo(repo repositories.UserRepository) *Service {
	s.userRepo = repo
	return s
}

// -----------------------------------------------------------------------------
// Dashboard Types
// -----------------------------------------------------------------------------

// DashboardData represents the aggregated dashboard data
type DashboardData struct {
	User             UserInfo         `json:"user"`
	EnrolledPathways []PathwayInfo    `json:"enrolledPathways"`
	RecentSessions   []SessionInfo    `json:"recentSessions"`
	Achievements     AchievementsInfo `json:"achievements"`
	Stats            StatsInfo        `json:"stats"`
	RecommendedNext  []Recommendation `json:"recommendedNext"`
}

// UserInfo contains user info for the dashboard
type UserInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email,omitempty"`
	TotalPoints int    `json:"totalPoints"`
	CurrentRank string `json:"currentRank,omitempty"`
}

// PathwayInfo represents an enrolled pathway summary
type PathwayInfo struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	Slug             string     `json:"slug"`
	Description      string     `json:"description,omitempty"`
	IconURL          string     `json:"iconUrl,omitempty"`
	CompletedModules int        `json:"completedModules"`
	TotalModules     int        `json:"totalModules"`
	Percentage       float64    `json:"percentage"`
	EnrolledAt       time.Time  `json:"enrolledAt"`
	LastActivityAt   *time.Time `json:"lastActivityAt,omitempty"`
	Status           string     `json:"status"`
	CurrentModule    *string    `json:"currentModule,omitempty"`
}

// SessionInfo represents a recent lab session
type SessionInfo struct {
	ID            string     `json:"id"`
	LabName       string     `json:"labName"`
	LabTemplateID string     `json:"labTemplateId"`
	StartedAt     time.Time  `json:"startedAt"`
	EndedAt       *time.Time `json:"endedAt,omitempty"`
	EarnedPoints  int        `json:"earnedPoints"`
	MaxPoints     int        `json:"maxPoints"`
	Passed        bool       `json:"passed"`
	Duration      *int       `json:"durationMinutes,omitempty"`
}

// AchievementsInfo contains achievement summary for dashboard
type AchievementsInfo struct {
	TotalEarned        int               `json:"totalEarned"`
	TotalAvailable     int               `json:"totalAvailable"`
	TotalPoints        int               `json:"totalPoints"`
	RecentAchievements []AchievementInfo `json:"recentAchievements"`
}

// AchievementInfo represents a recently earned achievement
type AchievementInfo struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Tier        string    `json:"tier"`
	IconURL     string    `json:"iconUrl,omitempty"`
	EarnedAt    time.Time `json:"earnedAt"`
	Points      int       `json:"points"`
}

// StatsInfo contains aggregate statistics
type StatsInfo struct {
	TotalLabsCompleted int     `json:"totalLabsCompleted"`
	TotalTimeSpentMins int     `json:"totalTimeSpentMins"`
	CurrentStreak      int     `json:"currentStreak"`
	BestStreak         int     `json:"bestStreak"`
	AverageScore       float64 `json:"averageScore"`
	PathwaysCompleted  int     `json:"pathwaysCompleted"`
	PathwaysInProgress int     `json:"pathwaysInProgress"`
}

// Recommendation represents a recommended next action
type Recommendation struct {
	Type        string `json:"type"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Action      string `json:"action"`
	Priority    int    `json:"priority"`
	LabID       string `json:"labId,omitempty"`
	PathwayID   string `json:"pathwayId,omitempty"`
	ModuleID    string `json:"moduleId,omitempty"`
	Difficulty  string `json:"difficulty,omitempty"`
}

// -----------------------------------------------------------------------------
// Activity Types
// -----------------------------------------------------------------------------

// ActivityData represents user activity feed
type ActivityData struct {
	Activities []ActivityItem `json:"activities"`
	HasMore    bool           `json:"hasMore"`
}

// ActivityItem represents a single activity entry
type ActivityItem struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Timestamp   time.Time      `json:"timestamp"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// -----------------------------------------------------------------------------
// Leaderboard Types
// -----------------------------------------------------------------------------

// LeaderboardEntry represents a user's ranking on the leaderboard
type LeaderboardEntry struct {
	Rank             int    `json:"rank"`
	UserID           string `json:"userId"`
	DisplayName      string `json:"displayName"`
	TotalPoints      int    `json:"totalPoints"`
	AchievementCount int    `json:"achievementCount"`
	LabsCompleted    int    `json:"labsCompleted"`
	IsCurrentUser    bool   `json:"isCurrentUser"`
}

// LeaderboardData contains the leaderboard data
type LeaderboardData struct {
	Entries     []LeaderboardEntry `json:"entries"`
	CurrentUser *LeaderboardEntry  `json:"currentUser,omitempty"`
	TotalUsers  int                `json:"totalUsers"`
}

// -----------------------------------------------------------------------------
// Service Methods
// -----------------------------------------------------------------------------

// GetDashboard builds the dashboard data for a user.
// GetDashboard orchestrates loading the user's dashboard: enrolled pathways,
// session stats, achievements, and recommendations. Individual sections fail
// soft — a failure to load (e.g.) achievements leaves that section empty
// rather than aborting the whole response.
func (s *Service) GetDashboard(ctx context.Context, userID, displayName, email string) (*DashboardData, error) {
	dashboard := &DashboardData{
		User: UserInfo{
			ID:          userID,
			DisplayName: displayName,
			Email:       email,
		},
		EnrolledPathways: []PathwayInfo{},
		RecentSessions:   []SessionInfo{},
		Achievements:     AchievementsInfo{},
		Stats:            StatsInfo{},
		RecommendedNext:  []Recommendation{},
	}

	s.loadEnrolledPathways(ctx, dashboard, userID)
	s.loadUserStatsAndRecentSessions(ctx, dashboard, userID)
	s.loadAchievements(ctx, dashboard, userID)

	dashboard.RecommendedNext = s.generateRecommendations(ctx, userID, dashboard)
	return dashboard, nil
}

// loadEnrolledPathways populates dashboard.EnrolledPathways and the pathway
// counters in dashboard.Stats. Best-effort: errors are logged and the slice
// stays empty.
func (s *Service) loadEnrolledPathways(ctx context.Context, dashboard *DashboardData, userID string) {
	if s.enrollmentRepo == nil || s.pathwayRepo == nil {
		return
	}
	enrollments, err := s.enrollmentRepo.List(ctx, models.EnrollmentListOptions{
		UserID: userID,
		Limit:  10,
	})
	if err != nil {
		s.logger.Error("Failed to get user enrollments", "error", err, "userId", userID)
		return
	}
	if len(enrollments) == 0 {
		return
	}

	pathwayIDs := make([]string, 0, len(enrollments))
	for _, enrollment := range enrollments {
		pathwayIDs = append(pathwayIDs, enrollment.PathwayID)
	}
	pathways, err := s.pathwayRepo.GetByIDs(ctx, pathwayIDs)
	if err != nil {
		s.logger.Error("Failed to batch fetch pathways", "error", err)
	}
	pathwayMap := make(map[string]*models.Pathway, len(pathways))
	for _, p := range pathways {
		pathwayMap[p.ID] = p
	}

	for _, enrollment := range enrollments {
		pathway := pathwayMap[enrollment.PathwayID]
		if pathway == nil {
			continue
		}
		dashboard.EnrolledPathways = append(dashboard.EnrolledPathways, PathwayInfo{
			ID:               enrollment.ID,
			Name:             pathway.Name,
			Slug:             pathway.Slug,
			Description:      pathway.Description,
			IconURL:          pathway.CoverImageURL,
			CompletedModules: enrollment.CompletedModules,
			TotalModules:     enrollment.TotalModules,
			Percentage:       enrollment.Percentage,
			EnrolledAt:       enrollment.EnrolledAt,
			LastActivityAt:   enrollment.LastActivityAt,
			Status:           string(enrollment.Status),
		})
		switch enrollment.Status {
		case models.EnrollmentStatusCompleted:
			dashboard.Stats.PathwaysCompleted++
		case models.EnrollmentStatusInProgress, models.EnrollmentStatusEnrolled:
			dashboard.Stats.PathwaysInProgress++
		}
	}
}

// loadUserStatsAndRecentSessions fills the aggregate Stats fields plus
// RecentSessions (the last 5 sessions). Lab-template names are batch-fetched
// to avoid N+1.
func (s *Service) loadUserStatsAndRecentSessions(ctx context.Context, dashboard *DashboardData, userID string) {
	if s.sessionRepo == nil {
		return
	}

	if userStats, err := s.sessionRepo.GetUserStats(ctx, userID); err != nil {
		s.logger.Error("Failed to get user stats", "error", err, "userId", userID)
	} else if userStats != nil {
		dashboard.Stats.TotalLabsCompleted = userStats.TotalLabsCompleted
		dashboard.Stats.TotalTimeSpentMins = userStats.TotalTimeSpentMins
		dashboard.Stats.AverageScore = userStats.AverageScore
		dashboard.Stats.CurrentStreak = userStats.CurrentStreak
		dashboard.Stats.BestStreak = userStats.BestStreak
	}

	sessions, err := s.sessionRepo.List(ctx, repositories.SessionFilter{
		UserID: userID,
		Limit:  5,
	})
	if err != nil {
		s.logger.Error("Failed to get user sessions", "error", err, "userId", userID)
		return
	}
	if len(sessions) == 0 {
		return
	}

	labTemplateMap := s.batchFetchLabTemplates(ctx, sessions)
	for _, session := range sessions {
		entry := SessionInfo{
			ID:            session.ID,
			LabTemplateID: session.LabTemplateID,
			StartedAt:     session.StartedAt,
			EndedAt:       session.EndedAt,
			EarnedPoints:  session.EarnedPoints,
			MaxPoints:     session.MaxPoints,
			Passed:        session.Passed,
		}
		if tmpl := labTemplateMap[session.LabTemplateID]; tmpl != nil {
			entry.LabName = tmpl.Name
		}
		if session.EndedAt != nil {
			duration := int(session.EndedAt.Sub(session.StartedAt).Minutes())
			entry.Duration = &duration
		}
		dashboard.RecentSessions = append(dashboard.RecentSessions, entry)
	}
}

// batchFetchLabTemplates returns a map keyed by template ID for every
// distinct LabTemplateID referenced by the sessions. Used to avoid N+1
// queries when annotating recent sessions with lab names.
func (s *Service) batchFetchLabTemplates(ctx context.Context, sessions []*models.Session) map[string]*models.LabTemplateRecord {
	if s.labTemplateRepo == nil || len(sessions) == 0 {
		return nil
	}
	labIDs := make([]string, 0, len(sessions))
	for _, session := range sessions {
		labIDs = append(labIDs, session.LabTemplateID)
	}
	templates, err := s.labTemplateRepo.GetByIDs(ctx, labIDs)
	if err != nil {
		s.logger.Error("Failed to batch fetch lab templates", "error", err)
		return nil
	}
	out := make(map[string]*models.LabTemplateRecord, len(templates))
	for _, t := range templates {
		out[t.ID] = t
	}
	return out
}

// loadAchievements populates dashboard.Achievements (counts + 5 most-recent)
// and User.TotalPoints. Best-effort: summary load failure leaves the section
// empty.
func (s *Service) loadAchievements(ctx context.Context, dashboard *DashboardData, userID string) {
	if s.achievementSvc == nil {
		return
	}
	summary, err := s.achievementSvc.GetUserSummary(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get achievement summary", "error", err, "userId", userID)
		return
	}
	if summary == nil {
		return
	}

	dashboard.Achievements.TotalEarned = summary.TotalEarned
	dashboard.Achievements.TotalAvailable = summary.TotalAvailable
	dashboard.Achievements.TotalPoints = summary.TotalPoints
	dashboard.User.TotalPoints = summary.TotalPoints

	if len(summary.RecentAchievements) == 0 || s.achievementRepo == nil {
		return
	}
	recentUAs := summary.RecentAchievements
	if len(recentUAs) > 5 {
		recentUAs = recentUAs[:5]
	}
	achievementIDs := make([]string, 0, len(recentUAs))
	for _, ua := range recentUAs {
		achievementIDs = append(achievementIDs, ua.AchievementID)
	}
	achievements, err := s.achievementRepo.GetAchievementsByIDs(ctx, achievementIDs)
	if err != nil {
		s.logger.Error("Failed to batch fetch achievements", "error", err)
		return
	}
	achievementMap := make(map[string]*models.Achievement, len(achievements))
	for _, a := range achievements {
		achievementMap[a.ID] = a
	}
	for _, ua := range recentUAs {
		achievement := achievementMap[ua.AchievementID]
		if achievement == nil {
			continue
		}
		dashboard.Achievements.RecentAchievements = append(dashboard.Achievements.RecentAchievements, AchievementInfo{
			ID:          achievement.ID,
			Name:        achievement.Name,
			Description: achievement.Description,
			Tier:        string(achievement.Tier),
			IconURL:     achievement.IconURL,
			EarnedAt:    ua.EarnedAt,
			Points:      achievement.Points,
		})
	}
}

// generateRecommendations creates personalized recommendations based on user progress
func (s *Service) generateRecommendations(ctx context.Context, userID string, dashboard *DashboardData) []Recommendation {
	recommendations := []Recommendation{}

	// 1. Recommend continuing active pathways (highest priority)
	for _, pathway := range dashboard.EnrolledPathways {
		if pathway.Status == "active" && pathway.Percentage < 100 {
			recommendations = append(recommendations, Recommendation{
				Type:        "continue_pathway",
				Title:       "Continue " + pathway.Name,
				Description: fmt.Sprintf("You're %.0f%% complete", pathway.Percentage),
				Action:      "/pathways/" + pathway.Slug,
				PathwayID:   pathway.ID,
				Priority:    1,
			})
			if len(recommendations) >= 2 {
				break
			}
		}
	}

	// 2. Recommend standalone labs based on user skill level
	if len(recommendations) < 4 && s.labTemplateRepo != nil && s.sessionRepo != nil {
		s.addLabRecommendations(ctx, userID, &recommendations)
	}

	// 3. If no active pathways, recommend enrolling in one
	if len(dashboard.EnrolledPathways) == 0 && s.pathwayRepo != nil && len(recommendations) < 4 {
		isFeatured := true
		pathways, err := s.pathwayRepo.List(ctx, models.PathwayListOptions{
			Status:     models.PathwayStatusPublished,
			IsFeatured: &isFeatured,
			Limit:      2,
		})
		if err == nil {
			for _, pathway := range pathways {
				if len(recommendations) >= 4 {
					break
				}
				recommendations = append(recommendations, Recommendation{
					Type:        "enroll_pathway",
					Title:       "Start " + pathway.Name,
					Description: truncateDescription(pathway.Description, 100),
					Action:      "/pathways/" + pathway.Slug,
					PathwayID:   pathway.ID,
					Priority:    3,
				})
			}
		}
	}

	// Limit to max 4 recommendations
	if len(recommendations) > 4 {
		recommendations = recommendations[:4]
	}

	return recommendations
}

// addLabRecommendations adds lab-based recommendations based on user skill level
func (s *Service) addLabRecommendations(ctx context.Context, userID string, recommendations *[]Recommendation) {
	// Get user stats to determine appropriate difficulty
	stats, err := s.sessionRepo.GetUserStats(ctx, userID)
	if err != nil {
		s.logger.Warn("Failed to get user stats for recommendations", "error", err)
		return
	}

	// Determine recommended difficulty based on user progress
	recommendedDifficulty := determineRecommendedDifficulty(stats)

	// Get user's completed lab IDs to exclude them
	completedLabIDs := make(map[string]bool)
	active := false
	sessions, err := s.sessionRepo.List(ctx, repositories.SessionFilter{
		UserID: userID,
		Active: &active,
		Limit:  100,
	})
	if err == nil {
		for _, session := range sessions {
			if session.Passed {
				completedLabIDs[session.LabTemplateID] = true
			}
		}
	}

	// Get active labs
	activeFilter := true
	labs, err := s.labTemplateRepo.List(ctx, repositories.LabTemplateFilter{
		Active: &activeFilter,
		Limit:  20,
	})
	if err != nil {
		s.logger.Warn("Failed to get labs for recommendations", "error", err)
		return
	}

	// Find labs matching recommended difficulty that user hasn't completed
	for _, lab := range labs {
		if len(*recommendations) >= 4 {
			break
		}
		if completedLabIDs[lab.ID] {
			continue
		}

		// Match difficulty level
		if lab.Difficulty == recommendedDifficulty {
			*recommendations = append(*recommendations, Recommendation{
				Type:        "try_lab",
				Title:       lab.Name,
				Description: truncateDescription(lab.Description, 80),
				Action:      "/labs/" + lab.Slug,
				LabID:       lab.ID,
				Difficulty:  lab.Difficulty,
				Priority:    2,
			})
		}
	}

	// If no labs at recommended difficulty, suggest trying next level up
	if len(*recommendations) < 3 && stats.TotalLabsCompleted > 0 {
		nextDifficulty := getNextDifficulty(recommendedDifficulty)
		if nextDifficulty != "" {
			for _, lab := range labs {
				if len(*recommendations) >= 4 {
					break
				}
				if completedLabIDs[lab.ID] {
					continue
				}
				if lab.Difficulty == nextDifficulty {
					*recommendations = append(*recommendations, Recommendation{
						Type:        "next_difficulty",
						Title:       "Challenge: " + lab.Name,
						Description: fmt.Sprintf("Ready to try %s difficulty?", nextDifficulty),
						Action:      "/labs/" + lab.Slug,
						LabID:       lab.ID,
						Difficulty:  lab.Difficulty,
						Priority:    2,
					})
					break // Only add one challenge recommendation
				}
			}
		}
	}
}

// GetActivity builds the activity data for a user.
func (s *Service) GetActivity(ctx context.Context, userID string, limit int) (*ActivityData, error) {
	if limit <= 0 {
		limit = 20
	}

	activities := []ActivityItem{}

	// Get recent completed sessions as activities (batch fetch lab templates)
	if s.sessionRepo != nil {
		active := false
		sessions, err := s.sessionRepo.List(ctx, repositories.SessionFilter{
			UserID: userID,
			Active: &active,
			Limit:  limit,
		})
		if err != nil {
			s.logger.Error("Failed to get user sessions", "error", err, "userId", userID)
		} else if len(sessions) > 0 {
			// Batch fetch lab templates
			labTemplateMap := make(map[string]*models.LabTemplateRecord)
			if s.labTemplateRepo != nil {
				labIDs := make([]string, 0, len(sessions))
				for _, session := range sessions {
					labIDs = append(labIDs, session.LabTemplateID)
				}
				templates, err := s.labTemplateRepo.GetByIDs(ctx, labIDs)
				if err != nil {
					s.logger.Error("Failed to batch fetch lab templates", "error", err)
				} else {
					for _, t := range templates {
						labTemplateMap[t.ID] = t
					}
				}
			}

			for _, session := range sessions {
				if session.EndedAt == nil {
					continue
				}

				labName := session.LabTemplateID
				if template := labTemplateMap[session.LabTemplateID]; template != nil {
					labName = template.Name
				}

				status := "completed"
				if session.Passed {
					status = "passed"
				}

				activities = append(activities, ActivityItem{
					ID:          session.ID,
					Type:        "lab_" + status,
					Title:       labName,
					Description: fmt.Sprintf("Scored %d/%d points", session.EarnedPoints, session.MaxPoints),
					Timestamp:   *session.EndedAt,
					Metadata: map[string]any{
						"labTemplateId": session.LabTemplateID,
						"earnedPoints":  session.EarnedPoints,
						"maxPoints":     session.MaxPoints,
						"passed":        session.Passed,
					},
				})
			}
		}
	}

	// Get recent achievements (batch fetch achievements to avoid N+1)
	if s.achievementRepo != nil {
		userAchievements, err := s.achievementRepo.ListUserAchievements(ctx, userID)
		if err != nil {
			s.logger.Error("Failed to get user achievements", "error", err, "userId", userID)
		} else if len(userAchievements) > 0 {
			// Batch fetch achievements
			achievementIDs := make([]string, 0, len(userAchievements))
			for _, ua := range userAchievements {
				achievementIDs = append(achievementIDs, ua.AchievementID)
			}
			achievements, err := s.achievementRepo.GetAchievementsByIDs(ctx, achievementIDs)
			if err != nil {
				s.logger.Error("Failed to batch fetch achievements", "error", err)
			} else {
				achievementMap := make(map[string]*models.Achievement, len(achievements))
				for _, a := range achievements {
					achievementMap[a.ID] = a
				}

				for _, ua := range userAchievements {
					achievement := achievementMap[ua.AchievementID]
					if achievement == nil {
						continue
					}

					activities = append(activities, ActivityItem{
						ID:          ua.ID,
						Type:        "achievement_earned",
						Title:       achievement.Name,
						Description: achievement.Description,
						Timestamp:   ua.EarnedAt,
						Metadata: map[string]any{
							"achievementId": achievement.ID,
							"tier":          achievement.Tier,
							"points":        achievement.Points,
						},
					})
				}
			}
		}
	}

	// Sort activities by timestamp (most recent first)
	sortActivitiesByTime(activities)

	// Limit results
	hasMore := len(activities) >= limit
	if len(activities) > limit {
		activities = activities[:limit]
	}

	return &ActivityData{
		Activities: activities,
		HasMore:    hasMore,
	}, nil
}

// GetLeaderboard builds the leaderboard data.
// GetLeaderboard ranks users by achievement points across the whole system.
// Sections are loaded independently and failures degrade gracefully (e.g. a
// failed user-name fetch falls back to "User <prefix>").
func (s *Service) GetLeaderboard(ctx context.Context, currentUserID string, limit int) (*LeaderboardData, error) {
	if s.achievementRepo == nil || s.sessionRepo == nil {
		return &LeaderboardData{Entries: []LeaderboardEntry{}}, nil
	}
	limit = clampLeaderboardLimit(limit)

	userStats := s.buildAchievementLeaderboardEntries(ctx)
	s.addLabCompletionCounts(ctx, userStats)
	s.populateLeaderboardDisplayNames(ctx, userStats)

	entries, currentUser := finalizeLeaderboard(userStats, currentUserID)
	response := &LeaderboardData{
		Entries:     entries,
		CurrentUser: currentUser,
		TotalUsers:  len(entries),
	}
	if len(response.Entries) > limit {
		response.Entries = response.Entries[:limit]
	}
	return response, nil
}

func clampLeaderboardLimit(limit int) int {
	switch {
	case limit <= 0:
		return 10
	case limit > 100:
		return 100
	default:
		return limit
	}
}

// buildAchievementLeaderboardEntries seeds userStats with per-user
// AchievementCount + TotalPoints derived from every user-achievement record.
func (s *Service) buildAchievementLeaderboardEntries(ctx context.Context) map[string]*LeaderboardEntry {
	userStats := make(map[string]*LeaderboardEntry)
	userAchievements, err := s.achievementRepo.ListAllUserAchievements(ctx)
	if err != nil {
		s.logger.Error("Failed to get achievements for leaderboard", "error", err)
		return userStats
	}
	if len(userAchievements) == 0 {
		return userStats
	}

	achievementIDs := make([]string, 0, len(userAchievements))
	for _, ua := range userAchievements {
		achievementIDs = append(achievementIDs, ua.AchievementID)
	}
	achievements, err := s.achievementRepo.GetAchievementsByIDs(ctx, achievementIDs)
	if err != nil {
		s.logger.Error("Failed to batch fetch achievements", "error", err)
	}
	achievementMap := make(map[string]*models.Achievement, len(achievements))
	for _, a := range achievements {
		achievementMap[a.ID] = a
	}

	for _, ua := range userAchievements {
		entry, ok := userStats[ua.UserID]
		if !ok {
			entry = &LeaderboardEntry{UserID: ua.UserID}
			userStats[ua.UserID] = entry
		}
		entry.AchievementCount++
		if achievement := achievementMap[ua.AchievementID]; achievement != nil {
			entry.TotalPoints += achievement.Points
		}
	}
	return userStats
}

// addLabCompletionCounts fills LabsCompleted on each entry via a single SQL
// aggregate. Creates entries for users who only have lab completions
// (no achievements) so they still appear in the leaderboard.
func (s *Service) addLabCompletionCounts(ctx context.Context, userStats map[string]*LeaderboardEntry) {
	labCounts, err := s.sessionRepo.CountCompletedLabsByUser(ctx)
	if err != nil {
		s.logger.Error("Failed to get lab completion counts for leaderboard", "error", err)
		return
	}
	for userID, count := range labCounts {
		entry, ok := userStats[userID]
		if !ok {
			entry = &LeaderboardEntry{UserID: userID}
			userStats[userID] = entry
		}
		entry.LabsCompleted = count
	}
}

// populateLeaderboardDisplayNames batch-fetches user records and fills
// DisplayName (falling back to Username, then to "User <id-prefix>" so the
// leaderboard never shows raw UUIDs).
func (s *Service) populateLeaderboardDisplayNames(ctx context.Context, userStats map[string]*LeaderboardEntry) {
	if s.userRepo == nil || len(userStats) == 0 {
		return
	}
	userIDs := make([]string, 0, len(userStats))
	for userID := range userStats {
		userIDs = append(userIDs, userID)
	}
	users, err := s.userRepo.GetByIDs(ctx, userIDs)
	if err != nil {
		s.logger.Error("Failed to batch fetch users", "error", err)
	}
	userMap := make(map[string]*models.User, len(users))
	for _, u := range users {
		userMap[u.ID] = u
	}

	for userID, entry := range userStats {
		dbUser := userMap[userID]
		switch {
		case dbUser != nil && dbUser.DisplayName != "":
			entry.DisplayName = dbUser.DisplayName
		case dbUser != nil:
			entry.DisplayName = dbUser.Username
		case len(userID) >= 8:
			entry.DisplayName = "User " + userID[:8]
		default:
			entry.DisplayName = "User " + userID
		}
	}
}

// finalizeLeaderboard converts the map of entries to a sorted slice, assigns
// ranks, and identifies the current user (if any). Returns the sorted slice
// and a *LeaderboardEntry pointing at the current user's rank entry.
func finalizeLeaderboard(userStats map[string]*LeaderboardEntry, currentUserID string) ([]LeaderboardEntry, *LeaderboardEntry) {
	entries := make([]LeaderboardEntry, 0, len(userStats))
	for _, entry := range userStats {
		entries = append(entries, *entry)
	}

	// Descending by TotalPoints. O(n²) selection-sort matches the original
	// implementation; entry count is bounded by active users which is small.
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].TotalPoints > entries[i].TotalPoints {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	var currentUser *LeaderboardEntry
	for i := range entries {
		entries[i].Rank = i + 1
		if currentUserID != "" && entries[i].UserID == currentUserID {
			entries[i].IsCurrentUser = true
			copyEntry := entries[i]
			currentUser = &copyEntry
		}
	}
	return entries, currentUser
}

// -----------------------------------------------------------------------------
// Helper Functions
// -----------------------------------------------------------------------------

// determineRecommendedDifficulty suggests an appropriate difficulty level based on user stats
func determineRecommendedDifficulty(stats *repositories.UserSessionStats) string {
	if stats == nil || stats.TotalLabsCompleted == 0 {
		return "beginner"
	}

	// Based on average score and completed labs, suggest difficulty
	avgScore := stats.AverageScore
	completed := stats.TotalLabsCompleted

	switch {
	case completed >= 20 && avgScore >= 85:
		return "expert"
	case completed >= 10 && avgScore >= 80:
		return "advanced"
	case completed >= 5 && avgScore >= 70:
		return "intermediate"
	default:
		return "beginner"
	}
}

// getNextDifficulty returns the next difficulty level up
func getNextDifficulty(current string) string {
	switch current {
	case "beginner":
		return "intermediate"
	case "intermediate":
		return "advanced"
	case "advanced":
		return "expert"
	default:
		return ""
	}
}

// truncateDescription limits description length for display
func truncateDescription(desc string, maxLen int) string {
	if len(desc) <= maxLen {
		return desc
	}
	return desc[:maxLen-3] + "..."
}

// -----------------------------------------------------------------------------
// Insights Types
// -----------------------------------------------------------------------------

// InsightsData represents learning insights for a user
type InsightsData struct {
	Skills         []SkillProficiency `json:"skills"`
	Strengths      []string           `json:"strengths"`
	Weaknesses     []string           `json:"weaknesses"`
	StreakCalendar map[string]int     `json:"streakCalendar"`
	WeeklyTime     []WeeklyTimeEntry  `json:"weeklyTime"`
}

// SkillProficiency represents proficiency in a skill tag
type SkillProficiency struct {
	Tag           string  `json:"tag"`
	Proficiency   float64 `json:"proficiency"`
	LabsCompleted int     `json:"labsCompleted"`
	AvgScore      float64 `json:"avgScore"`
}

// WeeklyTimeEntry represents time spent in a given ISO week
type WeeklyTimeEntry struct {
	Week    string `json:"week"`
	Minutes int    `json:"minutes"`
}

// GetInsights builds learning insights for a user.
// tagAggregateStats accumulates per-tag scoring during insights generation.
type tagAggregateStats struct {
	totalScore float64
	maxScore   float64
	labIDs     map[string]bool
}

// GetInsights produces skill proficiencies, strengths/weaknesses, the streak
// calendar (last 90 days), and weekly time-on-task (last 4 weeks).
func (s *Service) GetInsights(ctx context.Context, userID string) (*InsightsData, error) {
	insights := &InsightsData{
		Skills:         []SkillProficiency{},
		Strengths:      []string{},
		Weaknesses:     []string{},
		StreakCalendar: map[string]int{},
		WeeklyTime:     []WeeklyTimeEntry{},
	}
	if s.sessionRepo == nil || s.labTemplateRepo == nil {
		return insights, nil
	}

	sessions, templateMap, err := s.loadInsightsData(ctx, userID)
	if err != nil || len(sessions) == 0 {
		return insights, nil
	}

	tagMap, weeklyMinutes := s.aggregateSessionsForInsights(sessions, templateMap, insights)

	insights.Skills = buildSkillProficiencies(tagMap)
	insights.Strengths, insights.Weaknesses = extractStrengthsAndWeaknesses(insights.Skills)
	insights.WeeklyTime = sortWeeklyTime(weeklyMinutes)
	return insights, nil
}

// loadInsightsData fetches the user's completed sessions plus the
// (deduplicated) lab templates they reference.
func (s *Service) loadInsightsData(ctx context.Context, userID string) ([]*models.Session, map[string]*models.LabTemplateRecord, error) {
	active := false
	sessions, err := s.sessionRepo.List(ctx, repositories.SessionFilter{
		UserID: userID,
		Active: &active,
		Limit:  500,
	})
	if err != nil {
		s.logger.Error("Failed to get sessions for insights", "error", err, "userId", userID)
		return nil, nil, err
	}
	if len(sessions) == 0 {
		return nil, nil, nil
	}

	labIDs := make([]string, 0, len(sessions))
	for _, session := range sessions {
		labIDs = append(labIDs, session.LabTemplateID)
	}
	templates, err := s.labTemplateRepo.GetByIDs(ctx, labIDs)
	if err != nil {
		s.logger.Error("Failed to batch fetch lab templates for insights", "error", err)
		return nil, nil, err
	}
	templateMap := make(map[string]*models.LabTemplateRecord, len(templates))
	for _, t := range templates {
		templateMap[t.ID] = t
	}
	return sessions, templateMap, nil
}

// aggregateSessionsForInsights folds every session into three running
// aggregates: tag scoring, streak calendar (mutated in place on insights),
// and weekly minute totals.
func (s *Service) aggregateSessionsForInsights(
	sessions []*models.Session,
	templateMap map[string]*models.LabTemplateRecord,
	insights *InsightsData,
) (map[string]*tagAggregateStats, map[string]int) {
	tagMap := make(map[string]*tagAggregateStats)
	weeklyMinutes := make(map[string]int)
	now := time.Now()
	ninetyDaysAgo := now.AddDate(0, 0, -90)
	fourWeeksAgo := now.AddDate(0, 0, -28)

	for _, session := range sessions {
		tmpl := templateMap[session.LabTemplateID]

		if session.Passed && tmpl != nil && len(tmpl.Tags) > 0 {
			var scorePct float64
			if session.MaxPoints > 0 {
				scorePct = float64(session.EarnedPoints) / float64(session.MaxPoints) * 100
			}
			for _, tag := range tmpl.Tags {
				ts, ok := tagMap[tag]
				if !ok {
					ts = &tagAggregateStats{labIDs: make(map[string]bool)}
					tagMap[tag] = ts
				}
				ts.totalScore += scorePct
				ts.maxScore += 100
				ts.labIDs[session.LabTemplateID] = true
			}
		}

		if !session.StartedAt.Before(ninetyDaysAgo) {
			insights.StreakCalendar[session.StartedAt.Format("2006-01-02")]++
		}

		if session.EndedAt != nil && !session.StartedAt.Before(fourWeeksAgo) {
			if dur := int(session.EndedAt.Sub(session.StartedAt).Minutes()); dur > 0 {
				yr, wk := session.StartedAt.ISOWeek()
				weeklyMinutes[fmt.Sprintf("%d-W%02d", yr, wk)] += dur
			}
		}
	}
	return tagMap, weeklyMinutes
}

// buildSkillProficiencies converts the per-tag aggregates into sorted
// SkillProficiency entries (proficiency desc).
func buildSkillProficiencies(tagMap map[string]*tagAggregateStats) []SkillProficiency {
	skills := make([]SkillProficiency, 0, len(tagMap))
	for tag, ts := range tagMap {
		avgScore := 0.0
		if ts.maxScore > 0 {
			// maxScore is 100 * (session count), so totalScore / (maxScore/100)
			// is the mean per-session percent.
			avgScore = ts.totalScore / (ts.maxScore / 100)
		}
		skills = append(skills, SkillProficiency{
			Tag:           tag,
			Proficiency:   avgScore,
			LabsCompleted: len(ts.labIDs),
			AvgScore:      avgScore,
		})
	}
	for i := 0; i < len(skills)-1; i++ {
		for j := i + 1; j < len(skills); j++ {
			if skills[j].Proficiency > skills[i].Proficiency {
				skills[i], skills[j] = skills[j], skills[i]
			}
		}
	}
	return skills
}

// extractStrengthsAndWeaknesses picks the top 3 tags by proficiency for
// strengths and the bottom 3 (excluding any already in strengths) for
// weaknesses.
func extractStrengthsAndWeaknesses(skills []SkillProficiency) (strengths, weaknesses []string) {
	strengths = make([]string, 0, 3)
	weaknesses = make([]string, 0, 3)
	for i, skill := range skills {
		if i < 3 {
			strengths = append(strengths, skill.Tag)
		}
	}
	for i := len(skills) - 1; i >= 0 && len(weaknesses) < 3; i-- {
		isStrength := false
		for _, s := range strengths {
			if s == skills[i].Tag {
				isStrength = true
				break
			}
		}
		if !isStrength {
			weaknesses = append(weaknesses, skills[i].Tag)
		}
	}
	return strengths, weaknesses
}

// sortWeeklyTime converts the bucket map into a slice sorted by ISO week.
func sortWeeklyTime(weeklyMinutes map[string]int) []WeeklyTimeEntry {
	out := make([]WeeklyTimeEntry, 0, len(weeklyMinutes))
	for wk, mins := range weeklyMinutes {
		out = append(out, WeeklyTimeEntry{Week: wk, Minutes: mins})
	}
	for i := 0; i < len(out)-1; i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Week < out[i].Week {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// sortActivitiesByTime sorts activities by timestamp descending
func sortActivitiesByTime(activities []ActivityItem) {
	for i := 0; i < len(activities)-1; i++ {
		for j := i + 1; j < len(activities); j++ {
			if activities[j].Timestamp.After(activities[i].Timestamp) {
				activities[i], activities[j] = activities[j], activities[i]
			}
		}
	}
}
