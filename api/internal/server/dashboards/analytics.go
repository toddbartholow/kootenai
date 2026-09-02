package dashboards

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/server/serverutil"
)

// TimeAnalyticsResponse contains comprehensive time tracking analytics.
type TimeAnalyticsResponse struct {
	UserID           string            `json:"userId"`
	Period           string            `json:"period"`
	TotalTimeMinutes int               `json:"totalTimeMinutes"`
	SessionCount     int               `json:"sessionCount"`
	AverageSession   int               `json:"averageSessionMinutes"`
	LongestSession   int               `json:"longestSessionMinutes"`
	DailyBreakdown   []DailyTimeStats  `json:"dailyBreakdown,omitempty"`
	LabBreakdown     []LabTimeStats    `json:"labBreakdown,omitempty"`
	WeeklyTrend      []WeeklyTimeStats `json:"weeklyTrend,omitempty"`
	Stats            TimeStats         `json:"stats"`
}

// DailyTimeStats represents time spent on a specific day.
type DailyTimeStats struct {
	Date         string `json:"date"`
	TimeMinutes  int    `json:"timeMinutes"`
	SessionCount int    `json:"sessionCount"`
}

// LabTimeStats represents time spent on a specific lab.
type LabTimeStats struct {
	LabID          string  `json:"labId"`
	LabName        string  `json:"labName"`
	LabSlug        string  `json:"labSlug"`
	TotalMinutes   int     `json:"totalMinutes"`
	SessionCount   int     `json:"sessionCount"`
	AverageMinutes int     `json:"averageMinutes"`
	BestScore      float64 `json:"bestScore,omitempty"`
}

// WeeklyTimeStats represents time spent in a week.
type WeeklyTimeStats struct {
	WeekStart    string `json:"weekStart"`
	TimeMinutes  int    `json:"timeMinutes"`
	SessionCount int    `json:"sessionCount"`
}

// TimeStats contains computed statistics.
type TimeStats struct {
	TotalLabsCompleted int     `json:"totalLabsCompleted"`
	AverageScore       float64 `json:"averageScore"`
	CurrentStreak      int     `json:"currentStreak"`
	BestStreak         int     `json:"bestStreak"`
	PointsPerMinute    float64 `json:"pointsPerMinute,omitempty"`
	MostActiveDay      string  `json:"mostActiveDay,omitempty"`
}

// handleGetTimeAnalytics godoc
// @Summary Get Time Analytics
// @Description Get time-spent analytics for the authenticated user
// @Tags dashboard
// @Accept json
// @Produce json
// @Param period query string false "Aggregation period (default: month)"
// @Success 200 {object} TimeAnalyticsResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Security BearerAuth
// @Router /analytics/time [get]
func (m *Manager) handleGetTimeAnalytics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, ok := auth.UserFromContext(ctx)
	if !ok || user == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusUnauthorized, "authz.authRequired", nil)
		return
	}

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "month"
	}

	analytics := m.buildTimeAnalytics(ctx, user.ID, period)
	m.responder.JSONResponse(w, http.StatusOK, analytics)
}

// handleGetUserTimeAnalytics godoc
// @Summary Get User Time Analytics
// @Description Get time-spent analytics for a specific user. Callers may read their own; reading another user's requires admin or instructor.
// @Tags dashboard
// @Accept json
// @Produce json
// @Param userID path string true "User ID"
// @Param period query string false "Aggregation period (default: month)"
// @Success 200 {object} TimeAnalyticsResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 403 {object} ErrorResponse "Admin or instructor required"
// @Security BearerAuth
// @Router /users/{userID}/analytics/time [get]
func (m *Manager) handleGetUserTimeAnalytics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, ok := auth.UserFromContext(ctx)
	if !ok || user == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusUnauthorized, "authz.authRequired", nil)
		return
	}

	userID := chi.URLParam(r, "userID")
	if user.ID != userID && !serverutil.IsAdminOrInstructor(user) {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "authz.adminRequired", nil)
		return
	}

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "month"
	}

	analytics := m.buildTimeAnalytics(ctx, userID, period)
	m.responder.JSONResponse(w, http.StatusOK, analytics)
}

// buildTimeAnalytics constructs time analytics from database queries.
func (m *Manager) buildTimeAnalytics(ctx context.Context, userID, period string) TimeAnalyticsResponse {
	analytics := TimeAnalyticsResponse{
		UserID:         userID,
		Period:         period,
		DailyBreakdown: []DailyTimeStats{},
		LabBreakdown:   []LabTimeStats{},
		WeeklyTrend:    []WeeklyTimeStats{},
	}

	if m.sessionRepo == nil {
		return analytics
	}

	stats, err := m.sessionRepo.GetUserStats(ctx, userID)
	if err != nil {
		m.logger.Error("Failed to get user stats for analytics", "error", err, "userId", userID)
		return analytics
	}

	analytics.Stats = TimeStats{
		TotalLabsCompleted: stats.TotalLabsCompleted,
		AverageScore:       stats.AverageScore,
		CurrentStreak:      stats.CurrentStreak,
		BestStreak:         stats.BestStreak,
	}

	var startDate *time.Time
	now := time.Now()
	switch period {
	case "week":
		weekAgo := now.AddDate(0, 0, -7)
		startDate = &weekAgo
	case "month":
		monthAgo := now.AddDate(0, -1, 0)
		startDate = &monthAgo
	}

	active := false
	sessions, err := m.sessionRepo.List(ctx, repositories.SessionFilter{
		UserID: userID,
		Active: &active,
		Limit:  1000,
	})
	if err != nil {
		m.logger.Error("Failed to get sessions for analytics", "error", err, "userId", userID)
		return analytics
	}

	dailyMap := make(map[string]*DailyTimeStats)
	labMap := make(map[string]*LabTimeStats)
	weeklyMap := make(map[string]*WeeklyTimeStats)
	totalMinutes := 0
	longestSession := 0
	sessionCount := 0
	totalPoints := 0

	for _, session := range sessions {
		if session.EndedAt == nil {
			continue
		}
		if startDate != nil && session.EndedAt.Before(*startDate) {
			continue
		}

		duration := int(session.EndedAt.Sub(session.StartedAt).Minutes())
		if duration < 0 {
			continue
		}

		sessionCount++
		totalMinutes += duration
		if duration > longestSession {
			longestSession = duration
		}
		totalPoints += session.EarnedPoints

		dateKey := session.EndedAt.Format("2006-01-02")
		if daily, exists := dailyMap[dateKey]; exists {
			daily.TimeMinutes += duration
			daily.SessionCount++
		} else {
			dailyMap[dateKey] = &DailyTimeStats{
				Date:         dateKey,
				TimeMinutes:  duration,
				SessionCount: 1,
			}
		}

		if session.LabTemplateID != "" {
			if lab, exists := labMap[session.LabTemplateID]; exists {
				lab.TotalMinutes += duration
				lab.SessionCount++
				if session.Percentage > lab.BestScore {
					lab.BestScore = session.Percentage
				}
			} else {
				labMap[session.LabTemplateID] = &LabTimeStats{
					LabID:        session.LabTemplateID,
					TotalMinutes: duration,
					SessionCount: 1,
					BestScore:    session.Percentage,
				}
			}
		}

		weekStart := session.EndedAt.AddDate(0, 0, -int(session.EndedAt.Weekday()-time.Monday))
		if session.EndedAt.Weekday() == time.Sunday {
			weekStart = session.EndedAt.AddDate(0, 0, -6)
		}
		weekKey := weekStart.Format("2006-01-02")
		if weekly, exists := weeklyMap[weekKey]; exists {
			weekly.TimeMinutes += duration
			weekly.SessionCount++
		} else {
			weeklyMap[weekKey] = &WeeklyTimeStats{
				WeekStart:    weekKey,
				TimeMinutes:  duration,
				SessionCount: 1,
			}
		}
	}

	analytics.TotalTimeMinutes = totalMinutes
	analytics.SessionCount = sessionCount
	analytics.LongestSession = longestSession
	if sessionCount > 0 {
		analytics.AverageSession = totalMinutes / sessionCount
	}

	if totalMinutes > 0 {
		analytics.Stats.PointsPerMinute = float64(totalPoints) / float64(totalMinutes)
	}

	maxDailyTime := 0
	for _, daily := range dailyMap {
		analytics.DailyBreakdown = append(analytics.DailyBreakdown, *daily)
		if daily.TimeMinutes > maxDailyTime {
			maxDailyTime = daily.TimeMinutes
			analytics.Stats.MostActiveDay = daily.Date
		}
	}

	if m.labTemplateRepo != nil {
		for labID, lab := range labMap {
			if template, err := m.labTemplateRepo.GetByID(ctx, labID); err == nil && template != nil {
				lab.LabName = template.Name
				lab.LabSlug = template.Slug
			}
			if lab.SessionCount > 0 {
				lab.AverageMinutes = lab.TotalMinutes / lab.SessionCount
			}
			analytics.LabBreakdown = append(analytics.LabBreakdown, *lab)
		}
	} else {
		for _, lab := range labMap {
			if lab.SessionCount > 0 {
				lab.AverageMinutes = lab.TotalMinutes / lab.SessionCount
			}
			analytics.LabBreakdown = append(analytics.LabBreakdown, *lab)
		}
	}

	for _, weekly := range weeklyMap {
		analytics.WeeklyTrend = append(analytics.WeeklyTrend, *weekly)
	}

	return analytics
}
