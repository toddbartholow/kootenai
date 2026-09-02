package dashboard

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// Helper to create test logger
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestNewService(t *testing.T) {
	logger := testLogger()

	svc := NewService(logger)

	assert.NotNil(t, svc)
	assert.Equal(t, logger, svc.logger)
	assert.Nil(t, svc.sessionRepo)
	assert.Nil(t, svc.labTemplateRepo)
	assert.Nil(t, svc.pathwayRepo)
	assert.Nil(t, svc.enrollmentRepo)
}

func TestService_BuilderPattern(t *testing.T) {
	logger := testLogger()

	svc := NewService(logger)

	// Test that builder methods return the service for chaining
	result := svc.WithSessionRepo(nil)
	assert.Same(t, svc, result)

	result = svc.WithLabTemplateRepo(nil)
	assert.Same(t, svc, result)

	result = svc.WithPathwayRepo(nil)
	assert.Same(t, svc, result)

	result = svc.WithEnrollmentRepo(nil)
	assert.Same(t, svc, result)

	result = svc.WithAchievementRepo(nil)
	assert.Same(t, svc, result)

	result = svc.WithAchievementService(nil)
	assert.Same(t, svc, result)

	result = svc.WithUserRepo(nil)
	assert.Same(t, svc, result)
}

func TestService_GetDashboard_NoRepos(t *testing.T) {
	logger := testLogger()
	svc := NewService(logger)

	result, err := svc.GetDashboard(t.Context(), "user-123", "Test User", "test@example.com")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "user-123", result.User.ID)
	assert.Equal(t, "Test User", result.User.DisplayName)
	assert.Equal(t, "test@example.com", result.User.Email)
	assert.Empty(t, result.EnrolledPathways)
	assert.Empty(t, result.RecentSessions)
	assert.Empty(t, result.RecommendedNext)
}

func TestService_GetActivity_NoRepos(t *testing.T) {
	logger := testLogger()
	svc := NewService(logger)

	result, err := svc.GetActivity(t.Context(), "user-123", 20)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.Activities)
	assert.False(t, result.HasMore)
}

func TestService_GetActivity_DefaultLimit(t *testing.T) {
	logger := testLogger()
	svc := NewService(logger)

	// Test with 0 limit - should use default of 20
	result, err := svc.GetActivity(t.Context(), "user-123", 0)

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestService_GetLeaderboard_NoRepos(t *testing.T) {
	logger := testLogger()
	svc := NewService(logger)

	result, err := svc.GetLeaderboard(t.Context(), "user-123", 10)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.Entries)
	assert.Equal(t, 0, result.TotalUsers)
}

func TestService_GetLeaderboard_LimitValidation(t *testing.T) {
	logger := testLogger()
	svc := NewService(logger)

	// Test with 0 limit - should use default of 10
	result, err := svc.GetLeaderboard(t.Context(), "", 0)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Test with over-max limit - should cap at 100
	result, err = svc.GetLeaderboard(t.Context(), "", 200)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

// -----------------------------------------------------------------------------
// Type Format Tests
// -----------------------------------------------------------------------------

func TestDashboardData_Format(t *testing.T) {
	data := &DashboardData{
		User: UserInfo{
			ID:          "user-123",
			DisplayName: "Test User",
			Email:       "test@example.com",
			TotalPoints: 100,
		},
		EnrolledPathways: []PathwayInfo{},
		RecentSessions:   []SessionInfo{},
		Achievements:     AchievementsInfo{TotalEarned: 5, TotalAvailable: 20},
		Stats:            StatsInfo{TotalLabsCompleted: 10},
		RecommendedNext:  []Recommendation{},
	}

	assert.Equal(t, "user-123", data.User.ID)
	assert.Equal(t, "Test User", data.User.DisplayName)
	assert.Equal(t, 100, data.User.TotalPoints)
	assert.Equal(t, 5, data.Achievements.TotalEarned)
	assert.Equal(t, 10, data.Stats.TotalLabsCompleted)
}

func TestPathwayInfo_Format(t *testing.T) {
	now := time.Now()
	info := PathwayInfo{
		ID:               "pathway-123",
		Name:             "Security Basics",
		Slug:             "security-basics",
		CompletedModules: 3,
		TotalModules:     10,
		Percentage:       30.0,
		EnrolledAt:       now,
		Status:           "active",
	}

	assert.Equal(t, "pathway-123", info.ID)
	assert.Equal(t, "Security Basics", info.Name)
	assert.Equal(t, 3, info.CompletedModules)
	assert.Equal(t, 30.0, info.Percentage)
}

func TestSessionInfo_Format(t *testing.T) {
	now := time.Now()
	duration := 30
	info := SessionInfo{
		ID:            "session-123",
		LabName:       "Network Lab",
		LabTemplateID: "template-456",
		StartedAt:     now,
		EarnedPoints:  80,
		MaxPoints:     100,
		Passed:        true,
		Duration:      &duration,
	}

	assert.Equal(t, "session-123", info.ID)
	assert.Equal(t, "Network Lab", info.LabName)
	assert.Equal(t, 80, info.EarnedPoints)
	assert.True(t, info.Passed)
	assert.Equal(t, 30, *info.Duration)
}

func TestActivityItem_Format(t *testing.T) {
	now := time.Now()
	item := ActivityItem{
		ID:          "activity-123",
		Type:        "lab_passed",
		Title:       "Security Lab",
		Description: "Scored 80/100",
		Timestamp:   now,
		Metadata: map[string]interface{}{
			"earnedPoints": 80,
			"passed":       true,
		},
	}

	assert.Equal(t, "activity-123", item.ID)
	assert.Equal(t, "lab_passed", item.Type)
	assert.Equal(t, 80, item.Metadata["earnedPoints"])
}

func TestLeaderboardEntry_Format(t *testing.T) {
	entry := LeaderboardEntry{
		Rank:             1,
		UserID:           "user-123",
		DisplayName:      "Top Player",
		TotalPoints:      1000,
		AchievementCount: 15,
		LabsCompleted:    20,
		IsCurrentUser:    true,
	}

	assert.Equal(t, 1, entry.Rank)
	assert.Equal(t, "Top Player", entry.DisplayName)
	assert.Equal(t, 1000, entry.TotalPoints)
	assert.True(t, entry.IsCurrentUser)
}

func TestRecommendation_Format(t *testing.T) {
	rec := Recommendation{
		Type:        "continue_pathway",
		Title:       "Continue Security Basics",
		Description: "You're 50% complete",
		Action:      "/pathways/security-basics",
		PathwayID:   "pathway-123",
		Priority:    1,
	}

	assert.Equal(t, "continue_pathway", rec.Type)
	assert.Equal(t, "Continue Security Basics", rec.Title)
	assert.Equal(t, 1, rec.Priority)
}

// -----------------------------------------------------------------------------
// Helper Function Tests
// -----------------------------------------------------------------------------

func TestDetermineRecommendedDifficulty(t *testing.T) {
	tests := []struct {
		name     string
		stats    *repositories.UserSessionStats
		expected string
	}{
		{"nil stats returns beginner", nil, "beginner"},
		{"no completed labs returns beginner", &repositories.UserSessionStats{TotalLabsCompleted: 0}, "beginner"},
		{"few labs low score returns beginner", &repositories.UserSessionStats{TotalLabsCompleted: 3, AverageScore: 60}, "beginner"},
		{"5 labs with 70% score returns intermediate", &repositories.UserSessionStats{TotalLabsCompleted: 5, AverageScore: 70}, "intermediate"},
		{"10 labs with 80% score returns advanced", &repositories.UserSessionStats{TotalLabsCompleted: 10, AverageScore: 80}, "advanced"},
		{"20 labs with 85% score returns expert", &repositories.UserSessionStats{TotalLabsCompleted: 20, AverageScore: 85}, "expert"},
		{"many labs but low score stays beginner", &repositories.UserSessionStats{TotalLabsCompleted: 25, AverageScore: 50}, "beginner"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineRecommendedDifficulty(tt.stats)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetNextDifficulty(t *testing.T) {
	tests := []struct {
		current  string
		expected string
	}{
		{"beginner", "intermediate"},
		{"intermediate", "advanced"},
		{"advanced", "expert"},
		{"expert", ""},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.current, func(t *testing.T) {
			result := getNextDifficulty(tt.current)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTruncateDescription(t *testing.T) {
	tests := []struct {
		name     string
		desc     string
		maxLen   int
		expected string
	}{
		{"short string unchanged", "Hello", 10, "Hello"},
		{"exact length unchanged", "Hello", 5, "Hello"},
		{"long string truncated", "Hello World!", 8, "Hello..."},
		{"empty string", "", 10, ""},
		{"very short max", "Hello World", 4, "H..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateDescription(tt.desc, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSortActivitiesByTime(t *testing.T) {
	t.Run("empty slice", func(t *testing.T) {
		activities := []ActivityItem{}
		sortActivitiesByTime(activities)
		assert.Empty(t, activities)
	})

	t.Run("single item", func(t *testing.T) {
		now := time.Now()
		activities := []ActivityItem{
			{ID: "1", Timestamp: now},
		}
		sortActivitiesByTime(activities)
		assert.Len(t, activities, 1)
		assert.Equal(t, "1", activities[0].ID)
	})

	t.Run("reverse order gets sorted", func(t *testing.T) {
		now := time.Now()
		activities := []ActivityItem{
			{ID: "3", Timestamp: now.Add(-2 * time.Hour)},
			{ID: "2", Timestamp: now.Add(-time.Hour)},
			{ID: "1", Timestamp: now},
		}
		sortActivitiesByTime(activities)
		assert.Equal(t, "1", activities[0].ID)
		assert.Equal(t, "2", activities[1].ID)
		assert.Equal(t, "3", activities[2].ID)
	})

	t.Run("random order gets sorted", func(t *testing.T) {
		now := time.Now()
		activities := []ActivityItem{
			{ID: "2", Timestamp: now.Add(-time.Hour)},
			{ID: "4", Timestamp: now.Add(-3 * time.Hour)},
			{ID: "1", Timestamp: now},
			{ID: "3", Timestamp: now.Add(-2 * time.Hour)},
		}
		sortActivitiesByTime(activities)
		assert.Equal(t, "1", activities[0].ID)
		assert.Equal(t, "2", activities[1].ID)
		assert.Equal(t, "3", activities[2].ID)
		assert.Equal(t, "4", activities[3].ID)
	})
}

// -----------------------------------------------------------------------------
// Error Tests
// -----------------------------------------------------------------------------

func TestErrors(t *testing.T) {
	assert.Equal(t, "user not found", ErrUserNotFound.Error())
	assert.Equal(t, "dashboard repository not available", ErrRepoNotAvailable.Error())
}

// -----------------------------------------------------------------------------
// Additional Coverage Tests
// -----------------------------------------------------------------------------

func TestService_GetActivity_NegativeLimit(t *testing.T) {
	logger := testLogger()
	svc := NewService(logger)

	// Test with negative limit - should use default of 20
	result, err := svc.GetActivity(t.Context(), "user-123", -5)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.Activities)
}

func TestService_GetLeaderboard_NegativeLimit(t *testing.T) {
	logger := testLogger()
	svc := NewService(logger)

	// Test with negative limit - should use default of 10
	result, err := svc.GetLeaderboard(t.Context(), "", -10)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.Entries)
}

func TestGenerateRecommendations_NoPathways(t *testing.T) {
	logger := testLogger()
	svc := NewService(logger)

	// Dashboard with no enrolled pathways
	dashboard := &DashboardData{
		EnrolledPathways: []PathwayInfo{},
	}

	result := svc.generateRecommendations(t.Context(), "user-123", dashboard)

	// Should return empty when no repos and no pathways
	assert.Empty(t, result)
}

func TestGenerateRecommendations_WithActivePathways(t *testing.T) {
	logger := testLogger()
	svc := NewService(logger)

	// Dashboard with active pathways
	dashboard := &DashboardData{
		EnrolledPathways: []PathwayInfo{
			{
				ID:         "pathway-1",
				Name:       "Test Pathway",
				Slug:       "test-pathway",
				Status:     "active",
				Percentage: 50.0,
			},
		},
	}

	result := svc.generateRecommendations(t.Context(), "user-123", dashboard)

	assert.Len(t, result, 1)
	assert.Equal(t, "continue_pathway", result[0].Type)
	assert.Equal(t, "Continue Test Pathway", result[0].Title)
	assert.Equal(t, "/pathways/test-pathway", result[0].Action)
}

func TestGenerateRecommendations_MultipleActivePathways(t *testing.T) {
	logger := testLogger()
	svc := NewService(logger)

	// Dashboard with multiple active pathways
	dashboard := &DashboardData{
		EnrolledPathways: []PathwayInfo{
			{ID: "p1", Name: "Pathway 1", Slug: "p1", Status: "active", Percentage: 25.0},
			{ID: "p2", Name: "Pathway 2", Slug: "p2", Status: "active", Percentage: 50.0},
			{ID: "p3", Name: "Pathway 3", Slug: "p3", Status: "active", Percentage: 75.0},
		},
	}

	result := svc.generateRecommendations(t.Context(), "user-123", dashboard)

	// Should only recommend first 2 active pathways
	assert.Len(t, result, 2)
	assert.Equal(t, "Continue Pathway 1", result[0].Title)
	assert.Equal(t, "Continue Pathway 2", result[1].Title)
}

func TestGenerateRecommendations_CompletedPathway(t *testing.T) {
	logger := testLogger()
	svc := NewService(logger)

	// Dashboard with completed pathway (100%)
	dashboard := &DashboardData{
		EnrolledPathways: []PathwayInfo{
			{ID: "p1", Name: "Completed", Slug: "completed", Status: "active", Percentage: 100.0},
		},
	}

	result := svc.generateRecommendations(t.Context(), "user-123", dashboard)

	// Should not recommend completed pathway
	assert.Empty(t, result)
}

func TestGenerateRecommendations_InactiveStatus(t *testing.T) {
	logger := testLogger()
	svc := NewService(logger)

	// Dashboard with inactive pathway
	dashboard := &DashboardData{
		EnrolledPathways: []PathwayInfo{
			{ID: "p1", Name: "Inactive", Slug: "inactive", Status: "completed", Percentage: 50.0},
		},
	}

	result := svc.generateRecommendations(t.Context(), "user-123", dashboard)

	// Should not recommend non-active pathway
	assert.Empty(t, result)
}

func TestDetermineRecommendedDifficulty_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		stats    *repositories.UserSessionStats
		expected string
	}{
		{
			name:     "exactly 5 labs with 70 score",
			stats:    &repositories.UserSessionStats{TotalLabsCompleted: 5, AverageScore: 70},
			expected: "intermediate",
		},
		{
			name:     "exactly 10 labs with 80 score",
			stats:    &repositories.UserSessionStats{TotalLabsCompleted: 10, AverageScore: 80},
			expected: "advanced",
		},
		{
			name:     "exactly 20 labs with 85 score",
			stats:    &repositories.UserSessionStats{TotalLabsCompleted: 20, AverageScore: 85},
			expected: "expert",
		},
		{
			name:     "19 labs with 85 score stays advanced",
			stats:    &repositories.UserSessionStats{TotalLabsCompleted: 19, AverageScore: 85},
			expected: "advanced",
		},
		{
			name:     "20 labs with 84 score stays advanced",
			stats:    &repositories.UserSessionStats{TotalLabsCompleted: 20, AverageScore: 84},
			expected: "advanced",
		},
		{
			name:     "9 labs with 80 score stays intermediate",
			stats:    &repositories.UserSessionStats{TotalLabsCompleted: 9, AverageScore: 80},
			expected: "intermediate",
		},
		{
			name:     "10 labs with 79 score stays intermediate",
			stats:    &repositories.UserSessionStats{TotalLabsCompleted: 10, AverageScore: 79},
			expected: "intermediate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineRecommendedDifficulty(tt.stats)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSortActivitiesByTime_AlreadySorted(t *testing.T) {
	now := time.Now()
	activities := []ActivityItem{
		{ID: "1", Timestamp: now},
		{ID: "2", Timestamp: now.Add(-time.Hour)},
		{ID: "3", Timestamp: now.Add(-2 * time.Hour)},
	}
	sortActivitiesByTime(activities)
	assert.Equal(t, "1", activities[0].ID)
	assert.Equal(t, "2", activities[1].ID)
	assert.Equal(t, "3", activities[2].ID)
}

func TestSortActivitiesByTime_SameTimestamps(t *testing.T) {
	now := time.Now()
	activities := []ActivityItem{
		{ID: "1", Timestamp: now},
		{ID: "2", Timestamp: now},
		{ID: "3", Timestamp: now},
	}
	sortActivitiesByTime(activities)
	// Order should be maintained when timestamps are equal
	assert.Len(t, activities, 3)
}

func TestTruncateDescription_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		desc     string
		maxLen   int
		expected string
	}{
		{"maxLen of 3", "Hello", 3, "..."},
		{"unicode string", "Hello 世界!", 8, "Hello..."},
		{"whitespace only", "     ", 3, "..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateDescription(tt.desc, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLeaderboardData_EmptyCurrentUser(t *testing.T) {
	data := &LeaderboardData{
		Entries:     []LeaderboardEntry{},
		CurrentUser: nil,
		TotalUsers:  0,
	}

	assert.Nil(t, data.CurrentUser)
	assert.Empty(t, data.Entries)
	assert.Equal(t, 0, data.TotalUsers)
}

func TestActivityData_HasMore(t *testing.T) {
	data := &ActivityData{
		Activities: []ActivityItem{
			{ID: "1"},
			{ID: "2"},
		},
		HasMore: true,
	}

	assert.True(t, data.HasMore)
	assert.Len(t, data.Activities, 2)
}

func TestAchievementsInfo_AllFields(t *testing.T) {
	now := time.Now()
	info := AchievementsInfo{
		TotalEarned:    10,
		TotalAvailable: 50,
		TotalPoints:    500,
		RecentAchievements: []AchievementInfo{
			{
				ID:          "ach-1",
				Name:        "First Steps",
				Description: "Complete your first lab",
				Tier:        "bronze",
				IconURL:     "https://example.com/icon.png",
				EarnedAt:    now,
				Points:      50,
			},
		},
	}

	assert.Equal(t, 10, info.TotalEarned)
	assert.Equal(t, 50, info.TotalAvailable)
	assert.Equal(t, 500, info.TotalPoints)
	assert.Len(t, info.RecentAchievements, 1)
	assert.Equal(t, "First Steps", info.RecentAchievements[0].Name)
}

func TestStatsInfo_AllFields(t *testing.T) {
	stats := StatsInfo{
		TotalLabsCompleted: 25,
		TotalTimeSpentMins: 1500,
		CurrentStreak:      5,
		BestStreak:         10,
		AverageScore:       85.5,
		PathwaysCompleted:  3,
		PathwaysInProgress: 2,
	}

	assert.Equal(t, 25, stats.TotalLabsCompleted)
	assert.Equal(t, 1500, stats.TotalTimeSpentMins)
	assert.Equal(t, 5, stats.CurrentStreak)
	assert.Equal(t, 10, stats.BestStreak)
	assert.Equal(t, 85.5, stats.AverageScore)
	assert.Equal(t, 3, stats.PathwaysCompleted)
	assert.Equal(t, 2, stats.PathwaysInProgress)
}

// -----------------------------------------------------------------------------
// Mock Repository Implementations
// -----------------------------------------------------------------------------

type mockSessionRepo struct {
	sessions  []*models.Session
	userStats *repositories.UserSessionStats
	listErr   error
	statsErr  error
}

func (m *mockSessionRepo) Create(ctx context.Context, session *models.Session) error {
	return nil
}
func (m *mockSessionRepo) GetByID(ctx context.Context, id string) (*models.Session, error) {
	return nil, nil
}
func (m *mockSessionRepo) GetByPodID(ctx context.Context, podID string) ([]*models.Session, error) {
	return nil, nil
}
func (m *mockSessionRepo) GetActiveByUserID(ctx context.Context, userID string) ([]*models.Session, error) {
	return nil, nil
}
func (m *mockSessionRepo) List(ctx context.Context, filter repositories.SessionFilter) ([]*models.Session, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.sessions, nil
}
func (m *mockSessionRepo) Update(ctx context.Context, session *models.Session) error {
	return nil
}
func (m *mockSessionRepo) End(ctx context.Context, id string) error {
	return nil
}
func (m *mockSessionRepo) Delete(ctx context.Context, id string) error {
	return nil
}
func (m *mockSessionRepo) UpdateGrade(ctx context.Context, id string, earnedPoints int, passed bool) error {
	return nil
}
func (m *mockSessionRepo) MarkGradeSynced(ctx context.Context, id string, syncedAt time.Time) error {
	return nil
}
func (m *mockSessionRepo) MarkGradeSyncFailed(ctx context.Context, id string, errorMsg string) error {
	return nil
}
func (m *mockSessionRepo) GetUserID(ctx context.Context, id string) (string, error) {
	return "", nil
}
func (m *mockSessionRepo) IsOwner(ctx context.Context, id, userID string) (bool, error) {
	return false, nil
}
func (m *mockSessionRepo) GetOrganizationID(ctx context.Context, id string) (*string, error) {
	return nil, nil
}
func (m *mockSessionRepo) ListAll(ctx context.Context) ([]*models.Session, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.sessions, nil
}

func (m *mockSessionRepo) EndStaleSessions(ctx context.Context, maxAge time.Duration) (int64, error) {
	return 0, nil
}

func (m *mockSessionRepo) DeleteEndedBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return 0, nil
}

func (m *mockSessionRepo) GetUserStats(ctx context.Context, userID string) (*repositories.UserSessionStats, error) {
	if m.statsErr != nil {
		return nil, m.statsErr
	}
	return m.userStats, nil
}
func (m *mockSessionRepo) ListWithLabNames(ctx context.Context, filter repositories.SessionFilter) ([]*repositories.SessionWithLabName, error) {
	return nil, nil
}

func (m *mockSessionRepo) CountCompletedLabsByUser(ctx context.Context) (map[string]int, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	result := make(map[string]int)
	for _, s := range m.sessions {
		if s.EndedAt != nil && s.Passed {
			result[s.UserID]++
		}
	}
	return result, nil
}

type mockLabTemplateRepo struct {
	templates []*models.LabTemplateRecord
	listErr   error
}

func (m *mockLabTemplateRepo) Create(ctx context.Context, record *models.LabTemplateRecord) error {
	return nil
}
func (m *mockLabTemplateRepo) GetByID(ctx context.Context, id string) (*models.LabTemplateRecord, error) {
	for _, t := range m.templates {
		if t.ID == id {
			return t, nil
		}
	}
	return nil, nil
}
func (m *mockLabTemplateRepo) GetByIDs(ctx context.Context, ids []string) ([]*models.LabTemplateRecord, error) {
	result := make([]*models.LabTemplateRecord, 0)
	idMap := make(map[string]bool)
	for _, id := range ids {
		idMap[id] = true
	}
	for _, t := range m.templates {
		if idMap[t.ID] {
			result = append(result, t)
		}
	}
	return result, nil
}
func (m *mockLabTemplateRepo) GetByName(ctx context.Context, name string) (*models.LabTemplateRecord, error) {
	return nil, nil
}
func (m *mockLabTemplateRepo) List(ctx context.Context, filter repositories.LabTemplateFilter) ([]*models.LabTemplateRecord, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.templates, nil
}
func (m *mockLabTemplateRepo) Update(ctx context.Context, record *models.LabTemplateRecord) error {
	return nil
}
func (m *mockLabTemplateRepo) Delete(ctx context.Context, id string) error {
	return nil
}
func (m *mockLabTemplateRepo) SetActive(ctx context.Context, id string, active bool) error {
	return nil
}
func (m *mockLabTemplateRepo) CreateVersion(_ context.Context, _ *models.LabTemplateVersion) error {
	return nil
}
func (m *mockLabTemplateRepo) ListVersions(_ context.Context, _ string, _, _ int) ([]*models.LabTemplateVersion, error) {
	return nil, nil
}
func (m *mockLabTemplateRepo) GetVersionByNumber(_ context.Context, _ string, _ int) (*models.LabTemplateVersion, error) {
	return nil, nil
}
func (m *mockLabTemplateRepo) CountVersions(_ context.Context, _ string) (int, error) { return 0, nil }

type mockEnrollmentRepo struct {
	enrollments []*models.PathwayEnrollment
	listErr     error
}

func (m *mockEnrollmentRepo) Create(ctx context.Context, enrollment *models.PathwayEnrollment) error {
	return nil
}
func (m *mockEnrollmentRepo) GetByID(ctx context.Context, id string) (*models.PathwayEnrollment, error) {
	return nil, nil
}
func (m *mockEnrollmentRepo) GetByUserAndPathway(ctx context.Context, userID, pathwayID string) (*models.PathwayEnrollment, error) {
	return nil, nil
}
func (m *mockEnrollmentRepo) List(ctx context.Context, opts models.EnrollmentListOptions) ([]*models.PathwayEnrollment, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.enrollments, nil
}
func (m *mockEnrollmentRepo) Update(ctx context.Context, enrollment *models.PathwayEnrollment) error {
	return nil
}
func (m *mockEnrollmentRepo) Delete(ctx context.Context, id string) error {
	return nil
}
func (m *mockEnrollmentRepo) UpdateStatus(ctx context.Context, id string, status models.EnrollmentStatus) error {
	return nil
}
func (m *mockEnrollmentRepo) GetWithProgress(ctx context.Context, id string) (*models.PathwayEnrollment, error) {
	return nil, nil
}
func (m *mockEnrollmentRepo) InitializeModuleProgress(ctx context.Context, enrollmentID, pathwayID string) error {
	return nil
}
func (m *mockEnrollmentRepo) GetModuleProgress(ctx context.Context, enrollmentID, moduleID string) (*models.ModuleProgress, error) {
	return nil, nil
}
func (m *mockEnrollmentRepo) ListModuleProgress(ctx context.Context, enrollmentID string) ([]*models.ModuleProgress, error) {
	return nil, nil
}
func (m *mockEnrollmentRepo) UpdateModuleProgress(ctx context.Context, progress *models.ModuleProgress) error {
	return nil
}
func (m *mockEnrollmentRepo) UnlockModule(ctx context.Context, enrollmentID, moduleID string) error {
	return nil
}
func (m *mockEnrollmentRepo) UnlockNextModules(ctx context.Context, enrollmentID, completedModuleID string) ([]string, error) {
	return nil, nil
}
func (m *mockEnrollmentRepo) GetLabProgress(ctx context.Context, enrollmentID, labTemplateID string) (*models.LabProgress, error) {
	return nil, nil
}
func (m *mockEnrollmentRepo) ListLabProgress(ctx context.Context, enrollmentID, moduleID string) ([]*models.LabProgress, error) {
	return nil, nil
}
func (m *mockEnrollmentRepo) UpdateLabProgress(ctx context.Context, progress *models.LabProgress) error {
	return nil
}
func (m *mockEnrollmentRepo) RecordLabAttempt(ctx context.Context, enrollmentID, moduleID, labTemplateID, sessionID string, score int, passed bool) error {
	return nil
}
func (m *mockEnrollmentRepo) RecalculateModuleProgress(ctx context.Context, enrollmentID, moduleID string) error {
	return nil
}
func (m *mockEnrollmentRepo) RecalculateEnrollmentProgress(ctx context.Context, enrollmentID string) error {
	return nil
}

type mockPathwayRepo struct {
	pathways []*models.Pathway
	listErr  error
}

func (m *mockPathwayRepo) Create(ctx context.Context, pathway *models.Pathway) error {
	return nil
}
func (m *mockPathwayRepo) GetByID(ctx context.Context, id string) (*models.Pathway, error) {
	for _, p := range m.pathways {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, nil
}
func (m *mockPathwayRepo) GetByIDs(ctx context.Context, ids []string) ([]*models.Pathway, error) {
	result := make([]*models.Pathway, 0)
	idMap := make(map[string]bool)
	for _, id := range ids {
		idMap[id] = true
	}
	for _, p := range m.pathways {
		if idMap[p.ID] {
			result = append(result, p)
		}
	}
	return result, nil
}
func (m *mockPathwayRepo) GetBySlug(ctx context.Context, slug string) (*models.Pathway, error) {
	return nil, nil
}
func (m *mockPathwayRepo) List(ctx context.Context, opts models.PathwayListOptions) ([]*models.Pathway, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.pathways, nil
}
func (m *mockPathwayRepo) Update(ctx context.Context, pathway *models.Pathway) error {
	return nil
}
func (m *mockPathwayRepo) Delete(ctx context.Context, id string) error {
	return nil
}
func (m *mockPathwayRepo) UpdateStatus(ctx context.Context, id string, status models.PathwayStatus) error {
	return nil
}
func (m *mockPathwayRepo) GetWithModules(ctx context.Context, id string) (*models.Pathway, error) {
	return nil, nil
}
func (m *mockPathwayRepo) GetWithModulesBySlug(ctx context.Context, slug string) (*models.Pathway, error) {
	return nil, nil
}
func (m *mockPathwayRepo) CreateModule(ctx context.Context, module *models.PathwayModule) error {
	return nil
}
func (m *mockPathwayRepo) GetModuleByID(ctx context.Context, id string) (*models.PathwayModule, error) {
	return nil, nil
}
func (m *mockPathwayRepo) ListModules(ctx context.Context, pathwayID string) ([]*models.PathwayModule, error) {
	return nil, nil
}
func (m *mockPathwayRepo) UpdateModule(ctx context.Context, module *models.PathwayModule) error {
	return nil
}
func (m *mockPathwayRepo) DeleteModule(ctx context.Context, id string) error {
	return nil
}
func (m *mockPathwayRepo) ReorderModules(ctx context.Context, pathwayID string, moduleIDs []string) error {
	return nil
}
func (m *mockPathwayRepo) AddLabToModule(ctx context.Context, moduleLab *models.ModuleLab) error {
	return nil
}
func (m *mockPathwayRepo) RemoveLabFromModule(ctx context.Context, moduleID, labTemplateID string) error {
	return nil
}
func (m *mockPathwayRepo) ListModuleLabs(ctx context.Context, moduleID string) ([]*models.ModuleLab, error) {
	return nil, nil
}
func (m *mockPathwayRepo) ReorderModuleLabs(ctx context.Context, moduleID string, labTemplateIDs []string) error {
	return nil
}
func (m *mockPathwayRepo) GetStats(ctx context.Context, pathwayID string) (*models.PathwayStats, error) {
	return nil, nil
}

type mockAchievementRepo struct {
	achievements     []*models.Achievement
	userAchievements []*models.UserAchievement
	listErr          error
}

func (m *mockAchievementRepo) CreateAchievement(ctx context.Context, achievement *models.Achievement) error {
	return nil
}
func (m *mockAchievementRepo) GetAchievementByID(ctx context.Context, id string) (*models.Achievement, error) {
	for _, a := range m.achievements {
		if a.ID == id {
			return a, nil
		}
	}
	return nil, nil
}
func (m *mockAchievementRepo) GetAchievementsByIDs(ctx context.Context, ids []string) ([]*models.Achievement, error) {
	result := make([]*models.Achievement, 0)
	idMap := make(map[string]bool)
	for _, id := range ids {
		idMap[id] = true
	}
	for _, a := range m.achievements {
		if idMap[a.ID] {
			result = append(result, a)
		}
	}
	return result, nil
}
func (m *mockAchievementRepo) ListAchievements(ctx context.Context, filter repositories.AchievementFilter) ([]*models.Achievement, error) {
	return m.achievements, nil
}
func (m *mockAchievementRepo) UpdateAchievement(ctx context.Context, achievement *models.Achievement) error {
	return nil
}
func (m *mockAchievementRepo) DeleteAchievement(ctx context.Context, id string) error {
	return nil
}
func (m *mockAchievementRepo) AwardAchievement(ctx context.Context, userAchievement *models.UserAchievement) error {
	return nil
}
func (m *mockAchievementRepo) GetUserAchievement(ctx context.Context, userID, achievementID string) (*models.UserAchievement, error) {
	return nil, nil
}
func (m *mockAchievementRepo) ListUserAchievements(ctx context.Context, userID string) ([]*models.UserAchievement, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.userAchievements, nil
}
func (m *mockAchievementRepo) GetUserAchievementSummary(ctx context.Context, userID string) (*models.UserAchievementSummary, error) {
	return nil, nil
}
func (m *mockAchievementRepo) GetRecentAchievements(ctx context.Context, limit int) ([]*models.UserAchievement, error) {
	return nil, nil
}
func (m *mockAchievementRepo) MarkNotified(ctx context.Context, userAchievementID string) error {
	return nil
}
func (m *mockAchievementRepo) UpdateProgress(ctx context.Context, progress *models.AchievementProgress) error {
	return nil
}
func (m *mockAchievementRepo) GetProgress(ctx context.Context, userID, achievementID string) (*models.AchievementProgress, error) {
	return nil, nil
}
func (m *mockAchievementRepo) ListUserProgress(ctx context.Context, userID string) ([]*models.AchievementProgress, error) {
	return nil, nil
}
func (m *mockAchievementRepo) ListAchievementsWithProgress(ctx context.Context, userID string) ([]*models.AchievementWithProgress, error) {
	return nil, nil
}
func (m *mockAchievementRepo) ListAllUserAchievements(ctx context.Context) ([]*models.UserAchievement, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.userAchievements, nil
}
func (m *mockAchievementRepo) GetLeaderboard(ctx context.Context, limit int, timeRange string) (*models.Leaderboard, error) {
	return nil, nil
}

type mockUserRepo struct {
	users []*models.User
}

func (m *mockUserRepo) Create(ctx context.Context, user *models.User) error {
	return nil
}
func (m *mockUserRepo) GetByID(ctx context.Context, id string) (*models.User, error) {
	for _, u := range m.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, nil
}
func (m *mockUserRepo) GetByIDs(ctx context.Context, ids []string) ([]*models.User, error) {
	result := make([]*models.User, 0)
	idMap := make(map[string]bool)
	for _, id := range ids {
		idMap[id] = true
	}
	for _, u := range m.users {
		if idMap[u.ID] {
			result = append(result, u)
		}
	}
	return result, nil
}
func (m *mockUserRepo) GetByExternalID(ctx context.Context, externalID string) (*models.User, error) {
	return nil, nil
}
func (m *mockUserRepo) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	return nil, nil
}
func (m *mockUserRepo) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	return nil, nil
}
func (m *mockUserRepo) GetOrCreateByUsername(ctx context.Context, username string) (*models.User, error) {
	return nil, nil
}
func (m *mockUserRepo) Update(ctx context.Context, user *models.User) error {
	return nil
}
func (m *mockUserRepo) UpdateLastLogin(ctx context.Context, id string) error {
	return nil
}
func (m *mockUserRepo) List(ctx context.Context, opts repositories.UserListOptions) ([]*models.User, int, error) {
	return m.users, len(m.users), nil
}
func (m *mockUserRepo) Delete(ctx context.Context, id string) error {
	return nil
}
func (m *mockUserRepo) GetByEmailForAuth(ctx context.Context, email string) (*models.User, error) {
	return nil, nil
}
func (m *mockUserRepo) GetByIDWithPassword(ctx context.Context, id string) (*models.User, error) {
	return nil, nil
}
func (m *mockUserRepo) UpdatePassword(ctx context.Context, id, passwordHash string, mustChange bool) error {
	return nil
}
func (m *mockUserRepo) ClearMustChangePassword(ctx context.Context, id string) error {
	return nil
}
func (m *mockUserRepo) GetPreferredLocale(ctx context.Context, id string) (*string, error) {
	return nil, nil
}
func (m *mockUserRepo) UpdatePreferredLocale(ctx context.Context, id string, locale *string) error {
	return nil
}

// -----------------------------------------------------------------------------
// Dashboard Service Tests with Mocked Repositories
// -----------------------------------------------------------------------------

func TestService_GetDashboard_WithEnrollments(t *testing.T) {
	logger := testLogger()
	now := time.Now()

	enrollmentRepo := &mockEnrollmentRepo{
		enrollments: []*models.PathwayEnrollment{
			{
				ID:               "enroll-1",
				UserID:           "user-123",
				PathwayID:        "pathway-1",
				Status:           models.EnrollmentStatusInProgress,
				CompletedModules: 3,
				TotalModules:     10,
				Percentage:       30.0,
				EnrolledAt:       now.Add(-7 * 24 * time.Hour),
			},
			{
				ID:               "enroll-2",
				UserID:           "user-123",
				PathwayID:        "pathway-2",
				Status:           models.EnrollmentStatusCompleted,
				CompletedModules: 5,
				TotalModules:     5,
				Percentage:       100.0,
				EnrolledAt:       now.Add(-30 * 24 * time.Hour),
			},
		},
	}

	pathwayRepo := &mockPathwayRepo{
		pathways: []*models.Pathway{
			{ID: "pathway-1", Name: "Security Basics", Slug: "security-basics", Description: "Learn security fundamentals"},
			{ID: "pathway-2", Name: "Advanced Networking", Slug: "advanced-networking", Description: "Master networking"},
		},
	}

	svc := NewService(logger).
		WithEnrollmentRepo(enrollmentRepo).
		WithPathwayRepo(pathwayRepo)

	result, err := svc.GetDashboard(t.Context(), "user-123", "Test User", "test@example.com")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.EnrolledPathways, 2)
	assert.Equal(t, "Security Basics", result.EnrolledPathways[0].Name)
	assert.Equal(t, 30.0, result.EnrolledPathways[0].Percentage)
	assert.Equal(t, 1, result.Stats.PathwaysCompleted)
	assert.Equal(t, 1, result.Stats.PathwaysInProgress)
}

func TestService_GetDashboard_WithSessions(t *testing.T) {
	logger := testLogger()
	now := time.Now()
	endedAt := now.Add(-time.Hour)

	sessionRepo := &mockSessionRepo{
		sessions: []*models.Session{
			{
				ID:            "session-1",
				UserID:        "user-123",
				LabTemplateID: "template-1",
				StartedAt:     now.Add(-2 * time.Hour),
				EndedAt:       &endedAt,
				EarnedPoints:  80,
				MaxPoints:     100,
				Passed:        true,
			},
		},
		userStats: &repositories.UserSessionStats{
			TotalLabsCompleted: 10,
			TotalTimeSpentMins: 500,
			AverageScore:       75.5,
			CurrentStreak:      3,
			BestStreak:         7,
		},
	}

	labTemplateRepo := &mockLabTemplateRepo{
		templates: []*models.LabTemplateRecord{
			{ID: "template-1", Name: "Network Security Lab", Slug: "network-security"},
		},
	}

	svc := NewService(logger).
		WithSessionRepo(sessionRepo).
		WithLabTemplateRepo(labTemplateRepo)

	result, err := svc.GetDashboard(t.Context(), "user-123", "Test User", "test@example.com")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.RecentSessions, 1)
	assert.Equal(t, "Network Security Lab", result.RecentSessions[0].LabName)
	assert.Equal(t, 80, result.RecentSessions[0].EarnedPoints)
	assert.Equal(t, 10, result.Stats.TotalLabsCompleted)
	assert.Equal(t, 75.5, result.Stats.AverageScore)
	assert.Equal(t, 3, result.Stats.CurrentStreak)
}

func TestService_GetDashboard_WithErrors(t *testing.T) {
	logger := testLogger()

	t.Run("enrollment list error", func(t *testing.T) {
		enrollmentRepo := &mockEnrollmentRepo{
			listErr: errors.New("database error"),
		}
		pathwayRepo := &mockPathwayRepo{}

		svc := NewService(logger).
			WithEnrollmentRepo(enrollmentRepo).
			WithPathwayRepo(pathwayRepo)

		result, err := svc.GetDashboard(t.Context(), "user-123", "Test", "test@example.com")

		// Should not return error, just log it
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Empty(t, result.EnrolledPathways)
	})

	t.Run("session stats error", func(t *testing.T) {
		sessionRepo := &mockSessionRepo{
			statsErr: errors.New("stats error"),
		}

		svc := NewService(logger).WithSessionRepo(sessionRepo)

		result, err := svc.GetDashboard(t.Context(), "user-123", "Test", "test@example.com")

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 0, result.Stats.TotalLabsCompleted)
	})
}

func TestService_GetActivity_WithSessions(t *testing.T) {
	logger := testLogger()
	now := time.Now()
	endedAt := now.Add(-time.Hour)

	sessionRepo := &mockSessionRepo{
		sessions: []*models.Session{
			{
				ID:            "session-1",
				UserID:        "user-123",
				LabTemplateID: "template-1",
				StartedAt:     now.Add(-2 * time.Hour),
				EndedAt:       &endedAt,
				EarnedPoints:  80,
				MaxPoints:     100,
				Passed:        true,
			},
			{
				ID:            "session-2",
				UserID:        "user-123",
				LabTemplateID: "template-2",
				StartedAt:     now.Add(-3 * time.Hour),
				EndedAt:       &endedAt,
				EarnedPoints:  50,
				MaxPoints:     100,
				Passed:        false,
			},
		},
	}

	labTemplateRepo := &mockLabTemplateRepo{
		templates: []*models.LabTemplateRecord{
			{ID: "template-1", Name: "Security Lab"},
			{ID: "template-2", Name: "Network Lab"},
		},
	}

	svc := NewService(logger).
		WithSessionRepo(sessionRepo).
		WithLabTemplateRepo(labTemplateRepo)

	result, err := svc.GetActivity(t.Context(), "user-123", 10)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Activities, 2)
	assert.Equal(t, "lab_passed", result.Activities[0].Type)
	assert.Equal(t, "Security Lab", result.Activities[0].Title)
}

func TestService_GetActivity_WithAchievements(t *testing.T) {
	logger := testLogger()
	now := time.Now()

	achievementRepo := &mockAchievementRepo{
		achievements: []*models.Achievement{
			{ID: "ach-1", Name: "First Steps", Description: "Complete your first lab", Points: 50, Tier: models.AchievementTierBronze},
		},
		userAchievements: []*models.UserAchievement{
			{ID: "ua-1", UserID: "user-123", AchievementID: "ach-1", EarnedAt: now.Add(-24 * time.Hour)},
		},
	}

	svc := NewService(logger).WithAchievementRepo(achievementRepo)

	result, err := svc.GetActivity(t.Context(), "user-123", 10)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Activities, 1)
	assert.Equal(t, "achievement_earned", result.Activities[0].Type)
	assert.Equal(t, "First Steps", result.Activities[0].Title)
}

func TestService_GetActivity_SessionWithNoEndTime(t *testing.T) {
	logger := testLogger()
	now := time.Now()

	sessionRepo := &mockSessionRepo{
		sessions: []*models.Session{
			{
				ID:            "session-1",
				UserID:        "user-123",
				LabTemplateID: "template-1",
				StartedAt:     now,
				EndedAt:       nil, // Not ended yet
				EarnedPoints:  0,
				MaxPoints:     100,
				Passed:        false,
			},
		},
	}

	svc := NewService(logger).WithSessionRepo(sessionRepo)

	result, err := svc.GetActivity(t.Context(), "user-123", 10)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	// Session without EndedAt should be skipped
	assert.Empty(t, result.Activities)
}

func TestService_GetLeaderboard_WithData(t *testing.T) {
	logger := testLogger()
	now := time.Now()
	endedAt := now.Add(-time.Hour)

	achievementRepo := &mockAchievementRepo{
		achievements: []*models.Achievement{
			{ID: "ach-1", Name: "First Steps", Points: 50},
			{ID: "ach-2", Name: "Expert", Points: 200},
		},
		userAchievements: []*models.UserAchievement{
			{ID: "ua-1", UserID: "user-1", AchievementID: "ach-1"},
			{ID: "ua-2", UserID: "user-1", AchievementID: "ach-2"},
			{ID: "ua-3", UserID: "user-2", AchievementID: "ach-1"},
		},
	}

	sessionRepo := &mockSessionRepo{
		sessions: []*models.Session{
			{ID: "s1", UserID: "user-1", LabTemplateID: "t1", EndedAt: &endedAt, Passed: true},
			{ID: "s2", UserID: "user-1", LabTemplateID: "t2", EndedAt: &endedAt, Passed: true},
			{ID: "s3", UserID: "user-2", LabTemplateID: "t1", EndedAt: &endedAt, Passed: true},
		},
	}

	userRepo := &mockUserRepo{
		users: []*models.User{
			{ID: "user-1", DisplayName: "Alice"},
			{ID: "user-2", DisplayName: "Bob"},
		},
	}

	svc := NewService(logger).
		WithAchievementRepo(achievementRepo).
		WithSessionRepo(sessionRepo).
		WithUserRepo(userRepo)

	result, err := svc.GetLeaderboard(t.Context(), "user-1", 10)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 2, result.TotalUsers)
	assert.Len(t, result.Entries, 2)

	// User-1 should be first (250 points vs 50 points)
	assert.Equal(t, "Alice", result.Entries[0].DisplayName)
	assert.Equal(t, 250, result.Entries[0].TotalPoints)
	assert.Equal(t, 2, result.Entries[0].LabsCompleted)
	assert.True(t, result.Entries[0].IsCurrentUser)

	assert.Equal(t, "Bob", result.Entries[1].DisplayName)
	assert.Equal(t, 50, result.Entries[1].TotalPoints)

	assert.NotNil(t, result.CurrentUser)
	assert.Equal(t, "user-1", result.CurrentUser.UserID)
}

func TestService_GetLeaderboard_UserDisplayNameFallback(t *testing.T) {
	logger := testLogger()
	now := time.Now()
	endedAt := now.Add(-time.Hour)

	achievementRepo := &mockAchievementRepo{
		userAchievements: []*models.UserAchievement{
			{UserID: "user-with-long-id-12345678"},
			{UserID: "short"},
		},
	}

	sessionRepo := &mockSessionRepo{
		sessions: []*models.Session{
			{UserID: "user-with-long-id-12345678", EndedAt: &endedAt, Passed: true},
			{UserID: "short", EndedAt: &endedAt, Passed: true},
		},
	}

	// No users in the user repo, so fallback to "User <id>"
	userRepo := &mockUserRepo{users: []*models.User{}}

	svc := NewService(logger).
		WithAchievementRepo(achievementRepo).
		WithSessionRepo(sessionRepo).
		WithUserRepo(userRepo)

	result, err := svc.GetLeaderboard(t.Context(), "", 10)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 2, len(result.Entries))

	// Check fallback names
	foundLong := false
	foundShort := false
	for _, entry := range result.Entries {
		if entry.UserID == "user-with-long-id-12345678" {
			assert.Equal(t, "User user-wit", entry.DisplayName)
			foundLong = true
		}
		if entry.UserID == "short" {
			assert.Equal(t, "User short", entry.DisplayName)
			foundShort = true
		}
	}
	assert.True(t, foundLong)
	assert.True(t, foundShort)
}

func TestService_AddLabRecommendations(t *testing.T) {
	logger := testLogger()

	sessionRepo := &mockSessionRepo{
		sessions: []*models.Session{
			{ID: "s1", UserID: "user-123", LabTemplateID: "lab-1", Passed: true},
		},
		userStats: &repositories.UserSessionStats{
			TotalLabsCompleted: 3,
			AverageScore:       60,
		},
	}

	labTemplateRepo := &mockLabTemplateRepo{
		templates: []*models.LabTemplateRecord{
			{ID: "lab-1", Name: "Completed Lab", Slug: "completed", Difficulty: "beginner"},
			{ID: "lab-2", Name: "New Beginner Lab", Slug: "new-beginner", Difficulty: "beginner"},
			{ID: "lab-3", Name: "Intermediate Lab", Slug: "intermediate", Difficulty: "intermediate"},
		},
	}

	svc := NewService(logger).
		WithSessionRepo(sessionRepo).
		WithLabTemplateRepo(labTemplateRepo)

	recommendations := []Recommendation{}
	svc.addLabRecommendations(t.Context(), "user-123", &recommendations)

	// Should recommend lab-2 (beginner, not completed)
	assert.GreaterOrEqual(t, len(recommendations), 1)
	found := false
	for _, r := range recommendations {
		if r.LabID == "lab-2" {
			found = true
			assert.Equal(t, "try_lab", r.Type)
		}
	}
	assert.True(t, found, "Expected lab-2 in recommendations")
}

func TestService_AddLabRecommendations_NextDifficulty(t *testing.T) {
	logger := testLogger()

	sessionRepo := &mockSessionRepo{
		sessions: []*models.Session{},
		userStats: &repositories.UserSessionStats{
			TotalLabsCompleted: 5,
			AverageScore:       70,
		},
	}

	labTemplateRepo := &mockLabTemplateRepo{
		templates: []*models.LabTemplateRecord{
			{ID: "lab-1", Name: "Advanced Lab", Slug: "advanced", Difficulty: "advanced"},
		},
	}

	svc := NewService(logger).
		WithSessionRepo(sessionRepo).
		WithLabTemplateRepo(labTemplateRepo)

	recommendations := []Recommendation{}
	svc.addLabRecommendations(t.Context(), "user-123", &recommendations)

	// With 5 labs and 70% score, recommended difficulty is intermediate
	// lab-1 is advanced, so should appear as "next_difficulty" challenge
	found := false
	for _, r := range recommendations {
		if r.LabID == "lab-1" && r.Type == "next_difficulty" {
			found = true
			assert.Contains(t, r.Title, "Challenge")
		}
	}
	assert.True(t, found, "Expected advanced lab as challenge recommendation")
}

func TestService_GenerateRecommendations_EnrollPathway(t *testing.T) {
	logger := testLogger()

	pathwayRepo := &mockPathwayRepo{
		pathways: []*models.Pathway{
			{ID: "p1", Name: "Featured Pathway", Slug: "featured", Description: "A great featured pathway", IsFeatured: true, Status: models.PathwayStatusPublished},
		},
	}

	svc := NewService(logger).WithPathwayRepo(pathwayRepo)

	// Empty dashboard with no enrollments
	dashboard := &DashboardData{
		EnrolledPathways: []PathwayInfo{},
	}

	result := svc.generateRecommendations(t.Context(), "user-123", dashboard)

	// Should recommend enrolling in featured pathway
	assert.GreaterOrEqual(t, len(result), 1)
	found := false
	for _, r := range result {
		if r.Type == "enroll_pathway" && r.PathwayID == "p1" {
			found = true
			assert.Equal(t, "Start Featured Pathway", r.Title)
		}
	}
	assert.True(t, found, "Expected pathway enrollment recommendation")
}

func (m *mockSessionRepo) CountActive(ctx context.Context) (int64, error) {
	return 0, nil
}
