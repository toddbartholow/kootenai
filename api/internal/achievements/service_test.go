package achievements

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// Mock repositories for testing

type mockAchievementRepo struct {
	achievements             []*models.Achievement
	userAchievements         map[string]*models.UserAchievement
	achievementsWithProgress []*models.AchievementWithProgress
	progress                 map[string]*models.AchievementProgress
	awardedAchievements      []*models.UserAchievement
}

func newMockAchievementRepo() *mockAchievementRepo {
	return &mockAchievementRepo{
		achievements:        make([]*models.Achievement, 0),
		userAchievements:    make(map[string]*models.UserAchievement),
		progress:            make(map[string]*models.AchievementProgress),
		awardedAchievements: make([]*models.UserAchievement, 0),
	}
}

func (m *mockAchievementRepo) ListAchievements(ctx context.Context, filter repositories.AchievementFilter) ([]*models.Achievement, error) {
	return m.achievements, nil
}

func (m *mockAchievementRepo) GetUserAchievement(ctx context.Context, userID, achievementID string) (*models.UserAchievement, error) {
	key := userID + ":" + achievementID
	return m.userAchievements[key], nil
}

func (m *mockAchievementRepo) AwardAchievement(ctx context.Context, ua *models.UserAchievement) error {
	key := ua.UserID + ":" + ua.AchievementID
	m.userAchievements[key] = ua
	m.awardedAchievements = append(m.awardedAchievements, ua)
	return nil
}

func (m *mockAchievementRepo) UpdateProgress(ctx context.Context, progress *models.AchievementProgress) error {
	key := progress.UserID + ":" + progress.AchievementID
	m.progress[key] = progress
	return nil
}

func (m *mockAchievementRepo) ListAchievementsWithProgress(ctx context.Context, userID string) ([]*models.AchievementWithProgress, error) {
	return m.achievementsWithProgress, nil
}

func (m *mockAchievementRepo) GetUserAchievementSummary(ctx context.Context, userID string) (*models.UserAchievementSummary, error) {
	return &models.UserAchievementSummary{
		UserID:            userID,
		TotalEarned:       len(m.userAchievements),
		TotalPoints:       100,
		TotalAvailable:    len(m.achievements),
		CompletionPercent: 50.0,
	}, nil
}

// Additional methods required by the interface
func (m *mockAchievementRepo) GetAchievementByID(ctx context.Context, id string) (*models.Achievement, error) {
	for _, a := range m.achievements {
		if a.ID == id {
			return a, nil
		}
	}
	return nil, nil
}

func (m *mockAchievementRepo) GetAchievementsByIDs(ctx context.Context, ids []string) ([]*models.Achievement, error) {
	var result []*models.Achievement
	for _, id := range ids {
		for _, a := range m.achievements {
			if a.ID == id {
				result = append(result, a)
				break
			}
		}
	}
	return result, nil
}

func (m *mockAchievementRepo) CreateAchievement(ctx context.Context, a *models.Achievement) error {
	m.achievements = append(m.achievements, a)
	return nil
}

func (m *mockAchievementRepo) UpdateAchievement(ctx context.Context, a *models.Achievement) error {
	return nil
}

func (m *mockAchievementRepo) DeleteAchievement(ctx context.Context, id string) error {
	return nil
}

func (m *mockAchievementRepo) GetProgress(ctx context.Context, userID, achievementID string) (*models.AchievementProgress, error) {
	key := userID + ":" + achievementID
	return m.progress[key], nil
}

func (m *mockAchievementRepo) ListUserAchievements(ctx context.Context, userID string) ([]*models.UserAchievement, error) {
	var result []*models.UserAchievement
	for key, ua := range m.userAchievements {
		if ua.UserID == userID {
			_ = key
			result = append(result, ua)
		}
	}
	return result, nil
}

func (m *mockAchievementRepo) GetRecentAchievements(ctx context.Context, limit int) ([]*models.UserAchievement, error) {
	return nil, nil
}

func (m *mockAchievementRepo) MarkNotified(ctx context.Context, userAchievementID string) error {
	return nil
}

func (m *mockAchievementRepo) ListUserProgress(ctx context.Context, userID string) ([]*models.AchievementProgress, error) {
	var result []*models.AchievementProgress
	for key, p := range m.progress {
		if p.UserID == userID {
			_ = key
			result = append(result, p)
		}
	}
	return result, nil
}

func (m *mockAchievementRepo) ListAllUserAchievements(ctx context.Context) ([]*models.UserAchievement, error) {
	var result []*models.UserAchievement
	for _, ua := range m.userAchievements {
		result = append(result, ua)
	}
	return result, nil
}

func (m *mockAchievementRepo) GetLeaderboard(ctx context.Context, limit int, timeRange string) (*models.Leaderboard, error) {
	return &models.Leaderboard{
		Entries:    []*models.LeaderboardEntry{},
		TotalUsers: 0,
		UpdatedAt:  time.Now(),
		TimeRange:  timeRange,
	}, nil
}

type mockSessionRepo struct {
	sessions []*models.Session
}

func newMockSessionRepo() *mockSessionRepo {
	return &mockSessionRepo{
		sessions: make([]*models.Session, 0),
	}
}

func (m *mockSessionRepo) List(ctx context.Context, filter repositories.SessionFilter) ([]*models.Session, error) {
	if filter.UserID != "" {
		var result []*models.Session
		for _, s := range m.sessions {
			if s.UserID == filter.UserID {
				result = append(result, s)
			}
		}
		return result, nil
	}
	return m.sessions, nil
}

func (m *mockSessionRepo) Create(ctx context.Context, s *models.Session) error {
	m.sessions = append(m.sessions, s)
	return nil
}

func (m *mockSessionRepo) GetByID(ctx context.Context, id string) (*models.Session, error) {
	for _, s := range m.sessions {
		if s.ID == id {
			return s, nil
		}
	}
	return nil, nil
}

func (m *mockSessionRepo) GetByPodID(ctx context.Context, podID string) ([]*models.Session, error) {
	return nil, nil
}

func (m *mockSessionRepo) GetActiveByUserID(ctx context.Context, userID string) ([]*models.Session, error) {
	return nil, nil
}

func (m *mockSessionRepo) Update(ctx context.Context, s *models.Session) error {
	return nil
}

func (m *mockSessionRepo) End(ctx context.Context, id string) error {
	return nil
}

func (m *mockSessionRepo) UpdateGrade(ctx context.Context, id string, earned int, passed bool) error {
	return nil
}

func (m *mockSessionRepo) MarkGradeSynced(ctx context.Context, id string, syncedAt time.Time) error {
	return nil
}

func (m *mockSessionRepo) MarkGradeSyncFailed(ctx context.Context, id string, errorMsg string) error {
	return nil
}

func (m *mockSessionRepo) GetUserID(ctx context.Context, id string) (string, error) {
	for _, s := range m.sessions {
		if s.ID == id {
			return s.UserID, nil
		}
	}
	return "", nil
}

func (m *mockSessionRepo) IsOwner(ctx context.Context, id, userID string) (bool, error) {
	for _, s := range m.sessions {
		if s.ID == id && s.UserID == userID {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockSessionRepo) GetOrganizationID(ctx context.Context, id string) (*string, error) {
	return nil, nil
}

func (m *mockSessionRepo) ListAll(ctx context.Context) ([]*models.Session, error) {
	return m.sessions, nil
}

func (m *mockSessionRepo) EndStaleSessions(ctx context.Context, maxAge time.Duration) (int64, error) {
	return 0, nil
}

func (m *mockSessionRepo) DeleteEndedBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return 0, nil
}

func (m *mockSessionRepo) Delete(ctx context.Context, id string) error {
	for i, s := range m.sessions {
		if s.ID == id {
			m.sessions = append(m.sessions[:i], m.sessions[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockSessionRepo) GetUserStats(ctx context.Context, userID string) (*repositories.UserSessionStats, error) {
	return &repositories.UserSessionStats{}, nil
}

func (m *mockSessionRepo) ListWithLabNames(ctx context.Context, filter repositories.SessionFilter) ([]*repositories.SessionWithLabName, error) {
	return nil, nil
}

func (m *mockSessionRepo) CountCompletedLabsByUser(ctx context.Context) (map[string]int, error) {
	return nil, nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestNewService(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()

	svc := NewService(achievementRepo, sessionRepo, logger)
	if svc == nil {
		t.Fatal("expected service to be created")
	}
}

func TestCheckAndAwardAchievements_NoAchievements(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()

	svc := NewService(achievementRepo, sessionRepo, logger)

	session := &models.Session{
		ID:     "session-1",
		UserID: "user-1",
		Passed: true,
	}

	awarded, err := svc.CheckAndAwardAchievements(context.Background(), session, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(awarded) != 0 {
		t.Errorf("expected no awards, got %d", len(awarded))
	}
}

func TestCheckAndAwardAchievements_AlreadyEarned(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()

	// Add an achievement
	achievement := &models.Achievement{
		ID:       "ach-1",
		Name:     "First Lab",
		Type:     models.AchievementTypeLabCompletion,
		IsActive: true,
		Criteria: models.AchievementCriteria{TotalLabs: 1},
	}
	achievementRepo.achievements = append(achievementRepo.achievements, achievement)

	// Mark as already earned
	achievementRepo.userAchievements["user-1:ach-1"] = &models.UserAchievement{
		UserID:        "user-1",
		AchievementID: "ach-1",
		EarnedAt:      time.Now(),
	}

	svc := NewService(achievementRepo, sessionRepo, logger)

	session := &models.Session{
		ID:     "session-1",
		UserID: "user-1",
		Passed: true,
	}

	awarded, err := svc.CheckAndAwardAchievements(context.Background(), session, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(awarded) != 0 {
		t.Errorf("expected no new awards (already earned), got %d", len(awarded))
	}
}

func TestCheckAndAwardAchievements_LabCompletion(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()

	// Add achievement requiring 1 completed lab
	achievement := &models.Achievement{
		ID:       "ach-1",
		Name:     "First Lab",
		Type:     models.AchievementTypeLabCompletion,
		IsActive: true,
		Criteria: models.AchievementCriteria{TotalLabs: 1},
	}
	achievementRepo.achievements = append(achievementRepo.achievements, achievement)

	// Add a completed session for the user
	endedAt := time.Now()
	sessionRepo.sessions = append(sessionRepo.sessions, &models.Session{
		ID:      "session-1",
		UserID:  "user-1",
		Passed:  true,
		EndedAt: &endedAt,
	})

	svc := NewService(achievementRepo, sessionRepo, logger)

	session := &models.Session{
		ID:      "session-1",
		UserID:  "user-1",
		Passed:  true,
		EndedAt: &endedAt,
	}

	awarded, err := svc.CheckAndAwardAchievements(context.Background(), session, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(awarded) != 1 {
		t.Errorf("expected 1 award, got %d", len(awarded))
	}
	if len(awarded) > 0 && awarded[0].AchievementID != "ach-1" {
		t.Errorf("expected achievement ach-1, got %s", awarded[0].AchievementID)
	}
}

func TestCheckAndAwardAchievements_PerfectScore(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()

	// Add perfect score achievement
	achievement := &models.Achievement{
		ID:       "ach-perfect",
		Name:     "Perfectionist",
		Type:     models.AchievementTypePerfectScore,
		IsActive: true,
		Criteria: models.AchievementCriteria{RequirePerfect: true},
	}
	achievementRepo.achievements = append(achievementRepo.achievements, achievement)

	svc := NewService(achievementRepo, sessionRepo, logger)

	session := &models.Session{
		ID:         "session-1",
		UserID:     "user-1",
		Percentage: 100.0,
		Passed:     true,
	}

	awarded, err := svc.CheckAndAwardAchievements(context.Background(), session, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(awarded) != 1 {
		t.Errorf("expected 1 award for perfect score, got %d", len(awarded))
	}
}

func TestCheckAndAwardAchievements_PerfectScore_NotPerfect(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()

	// Add perfect score achievement
	achievement := &models.Achievement{
		ID:       "ach-perfect",
		Name:     "Perfectionist",
		Type:     models.AchievementTypePerfectScore,
		IsActive: true,
		Criteria: models.AchievementCriteria{RequirePerfect: true},
	}
	achievementRepo.achievements = append(achievementRepo.achievements, achievement)

	svc := NewService(achievementRepo, sessionRepo, logger)

	session := &models.Session{
		ID:         "session-1",
		UserID:     "user-1",
		Percentage: 90.0, // Not perfect
		Passed:     true,
	}

	awarded, err := svc.CheckAndAwardAchievements(context.Background(), session, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(awarded) != 0 {
		t.Errorf("expected no award for non-perfect score, got %d", len(awarded))
	}
}

func TestCheckAndAwardAchievements_Speed(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()

	// Add speed achievement (complete in under 10 minutes)
	achievement := &models.Achievement{
		ID:       "ach-speed",
		Name:     "Speed Demon",
		Type:     models.AchievementTypeSpeed,
		IsActive: true,
		Criteria: models.AchievementCriteria{MaxDurationMins: 10},
	}
	achievementRepo.achievements = append(achievementRepo.achievements, achievement)

	svc := NewService(achievementRepo, sessionRepo, logger)

	startedAt := time.Now().Add(-5 * time.Minute)
	endedAt := time.Now()
	session := &models.Session{
		ID:        "session-1",
		UserID:    "user-1",
		StartedAt: startedAt,
		EndedAt:   &endedAt,
		Passed:    true,
	}

	awarded, err := svc.CheckAndAwardAchievements(context.Background(), session, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(awarded) != 1 {
		t.Errorf("expected 1 award for speed, got %d", len(awarded))
	}
}

func TestCheckAndAwardAchievements_Speed_TooSlow(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()

	// Add speed achievement (complete in under 10 minutes)
	achievement := &models.Achievement{
		ID:       "ach-speed",
		Name:     "Speed Demon",
		Type:     models.AchievementTypeSpeed,
		IsActive: true,
		Criteria: models.AchievementCriteria{MaxDurationMins: 10},
	}
	achievementRepo.achievements = append(achievementRepo.achievements, achievement)

	svc := NewService(achievementRepo, sessionRepo, logger)

	startedAt := time.Now().Add(-20 * time.Minute) // 20 minutes ago - too slow
	endedAt := time.Now()
	session := &models.Session{
		ID:        "session-1",
		UserID:    "user-1",
		StartedAt: startedAt,
		EndedAt:   &endedAt,
		Passed:    true,
	}

	awarded, err := svc.CheckAndAwardAchievements(context.Background(), session, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(awarded) != 0 {
		t.Errorf("expected no award for being too slow, got %d", len(awarded))
	}
}

func TestCheckAndAwardAchievements_Streak(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()

	// Add streak achievement (3 passed in a row)
	achievement := &models.Achievement{
		ID:       "ach-streak",
		Name:     "Hat Trick",
		Type:     models.AchievementTypeStreak,
		IsActive: true,
		Criteria: models.AchievementCriteria{StreakCount: 3},
	}
	achievementRepo.achievements = append(achievementRepo.achievements, achievement)

	// Add 3 passed sessions
	for i := 0; i < 3; i++ {
		endedAt := time.Now().Add(time.Duration(-i) * time.Hour)
		sessionRepo.sessions = append(sessionRepo.sessions, &models.Session{
			ID:      "session-" + string(rune('1'+i)),
			UserID:  "user-1",
			Passed:  true,
			EndedAt: &endedAt,
		})
	}

	svc := NewService(achievementRepo, sessionRepo, logger)

	endedAt := time.Now()
	session := &models.Session{
		ID:      "session-3",
		UserID:  "user-1",
		Passed:  true,
		EndedAt: &endedAt,
	}

	awarded, err := svc.CheckAndAwardAchievements(context.Background(), session, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(awarded) != 1 {
		t.Errorf("expected 1 award for streak, got %d", len(awarded))
	}
}

func TestCheckAndAwardAchievements_SpecialEarlyMorning(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()

	// Add early morning achievement
	achievement := &models.Achievement{
		ID:       "ach-early",
		Name:     "Early Bird",
		Type:     models.AchievementTypeSpecial,
		IsActive: true,
		Criteria: models.AchievementCriteria{CustomRule: "early_morning"},
	}
	achievementRepo.achievements = append(achievementRepo.achievements, achievement)

	svc := NewService(achievementRepo, sessionRepo, logger)

	// Session completed at 5 AM
	endedAt := time.Date(2024, 1, 15, 5, 0, 0, 0, time.UTC)
	session := &models.Session{
		ID:      "session-1",
		UserID:  "user-1",
		EndedAt: &endedAt,
		Passed:  true,
	}

	awarded, err := svc.CheckAndAwardAchievements(context.Background(), session, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(awarded) != 1 {
		t.Errorf("expected 1 award for early morning, got %d", len(awarded))
	}
}

func TestCheckAndAwardAchievements_SpecialLateNight(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()

	// Add late night achievement
	achievement := &models.Achievement{
		ID:       "ach-late",
		Name:     "Night Owl",
		Type:     models.AchievementTypeSpecial,
		IsActive: true,
		Criteria: models.AchievementCriteria{CustomRule: "late_night"},
	}
	achievementRepo.achievements = append(achievementRepo.achievements, achievement)

	svc := NewService(achievementRepo, sessionRepo, logger)

	// Session completed at 11 PM
	endedAt := time.Date(2024, 1, 15, 23, 0, 0, 0, time.UTC)
	session := &models.Session{
		ID:      "session-1",
		UserID:  "user-1",
		EndedAt: &endedAt,
		Passed:  true,
	}

	awarded, err := svc.CheckAndAwardAchievements(context.Background(), session, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(awarded) != 1 {
		t.Errorf("expected 1 award for late night, got %d", len(awarded))
	}
}

func TestGetUserAchievements(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()

	achievementRepo.achievementsWithProgress = []*models.AchievementWithProgress{
		{
			Achievement: models.Achievement{ID: "ach-1", Name: "Test"},
			Earned:      true,
		},
	}

	svc := NewService(achievementRepo, sessionRepo, logger)

	achievements, err := svc.GetUserAchievements(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(achievements) != 1 {
		t.Errorf("expected 1 achievement, got %d", len(achievements))
	}
}

func TestGetUserSummary(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()

	achievementRepo.achievements = []*models.Achievement{
		{ID: "ach-1"},
		{ID: "ach-2"},
	}

	svc := NewService(achievementRepo, sessionRepo, logger)

	summary, err := svc.GetUserSummary(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary == nil {
		t.Fatal("expected summary, got nil")
	}
	if summary.TotalAvailable != 2 {
		t.Errorf("expected 2 total available achievements, got %d", summary.TotalAvailable)
	}
}

// Mock LabTemplateRepository for category tests
type mockLabTemplateRepo struct {
	templates []*models.LabTemplateRecord
}

func newMockLabTemplateRepo() *mockLabTemplateRepo {
	return &mockLabTemplateRepo{
		templates: make([]*models.LabTemplateRecord, 0),
	}
}

func (m *mockLabTemplateRepo) Create(ctx context.Context, record *models.LabTemplateRecord) error {
	m.templates = append(m.templates, record)
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
	var result []*models.LabTemplateRecord
	for _, id := range ids {
		for _, t := range m.templates {
			if t.ID == id {
				result = append(result, t)
				break
			}
		}
	}
	return result, nil
}

func (m *mockLabTemplateRepo) GetByName(ctx context.Context, name string) (*models.LabTemplateRecord, error) {
	for _, t := range m.templates {
		if t.Name == name {
			return t, nil
		}
	}
	return nil, nil
}

func (m *mockLabTemplateRepo) List(ctx context.Context, filter repositories.LabTemplateFilter) ([]*models.LabTemplateRecord, error) {
	var result []*models.LabTemplateRecord
	for _, t := range m.templates {
		if filter.Category != "" && t.Category != filter.Category {
			continue
		}
		if filter.Active != nil && t.IsActive != *filter.Active {
			continue
		}
		result = append(result, t)
	}
	return result, nil
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

// Mock EnrollmentRepository for pathway tests
type mockEnrollmentRepo struct {
	enrollments map[string]*models.PathwayEnrollment
}

func newMockEnrollmentRepo() *mockEnrollmentRepo {
	return &mockEnrollmentRepo{
		enrollments: make(map[string]*models.PathwayEnrollment),
	}
}

func (m *mockEnrollmentRepo) Create(ctx context.Context, enrollment *models.PathwayEnrollment) error {
	m.enrollments[enrollment.ID] = enrollment
	return nil
}

func (m *mockEnrollmentRepo) GetByID(ctx context.Context, id string) (*models.PathwayEnrollment, error) {
	return m.enrollments[id], nil
}

func (m *mockEnrollmentRepo) GetByUserAndPathway(ctx context.Context, userID, pathwayID string) (*models.PathwayEnrollment, error) {
	for _, e := range m.enrollments {
		if e.UserID == userID && e.PathwayID == pathwayID {
			return e, nil
		}
	}
	return nil, nil
}

func (m *mockEnrollmentRepo) List(ctx context.Context, opts models.EnrollmentListOptions) ([]*models.PathwayEnrollment, error) {
	var result []*models.PathwayEnrollment
	for _, e := range m.enrollments {
		result = append(result, e)
	}
	return result, nil
}

func (m *mockEnrollmentRepo) Update(ctx context.Context, enrollment *models.PathwayEnrollment) error {
	return nil
}

func (m *mockEnrollmentRepo) Delete(ctx context.Context, id string) error {
	delete(m.enrollments, id)
	return nil
}

func (m *mockEnrollmentRepo) UpdateStatus(ctx context.Context, id string, status models.EnrollmentStatus) error {
	return nil
}

func (m *mockEnrollmentRepo) GetWithProgress(ctx context.Context, id string) (*models.PathwayEnrollment, error) {
	return m.enrollments[id], nil
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

// Mock PathwayRepository for pathway tests
type mockPathwayRepo struct {
	pathways map[string]*models.Pathway
}

func newMockPathwayRepo() *mockPathwayRepo {
	return &mockPathwayRepo{
		pathways: make(map[string]*models.Pathway),
	}
}

func (m *mockPathwayRepo) Create(ctx context.Context, pathway *models.Pathway) error {
	m.pathways[pathway.ID] = pathway
	return nil
}

func (m *mockPathwayRepo) GetByID(ctx context.Context, id string) (*models.Pathway, error) {
	return m.pathways[id], nil
}

func (m *mockPathwayRepo) GetByIDs(ctx context.Context, ids []string) ([]*models.Pathway, error) {
	var result []*models.Pathway
	for _, id := range ids {
		if p, ok := m.pathways[id]; ok {
			result = append(result, p)
		}
	}
	return result, nil
}

func (m *mockPathwayRepo) GetBySlug(ctx context.Context, slug string) (*models.Pathway, error) {
	for _, p := range m.pathways {
		if p.Slug == slug {
			return p, nil
		}
	}
	return nil, nil
}

func (m *mockPathwayRepo) List(ctx context.Context, opts models.PathwayListOptions) ([]*models.Pathway, error) {
	var result []*models.Pathway
	for _, p := range m.pathways {
		result = append(result, p)
	}
	return result, nil
}

func (m *mockPathwayRepo) Update(ctx context.Context, pathway *models.Pathway) error {
	return nil
}

func (m *mockPathwayRepo) Delete(ctx context.Context, id string) error {
	delete(m.pathways, id)
	return nil
}

func (m *mockPathwayRepo) UpdateStatus(ctx context.Context, id string, status models.PathwayStatus) error {
	return nil
}

func (m *mockPathwayRepo) GetWithModules(ctx context.Context, id string) (*models.Pathway, error) {
	return m.pathways[id], nil
}

func (m *mockPathwayRepo) GetWithModulesBySlug(ctx context.Context, slug string) (*models.Pathway, error) {
	for _, p := range m.pathways {
		if p.Slug == slug {
			return p, nil
		}
	}
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

// Tests for WithXxx methods

func TestWithLabTemplateRepo(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()
	labTemplateRepo := newMockLabTemplateRepo()

	svc := NewService(achievementRepo, sessionRepo, logger)
	result := svc.WithLabTemplateRepo(labTemplateRepo)

	// Should return same service for chaining
	if result != svc {
		t.Error("WithLabTemplateRepo should return the same service instance")
	}

	// Verify the repo was set
	if svc.labTemplateRepo == nil {
		t.Error("labTemplateRepo should be set")
	}
}

func TestWithEnrollmentRepo(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()
	enrollmentRepo := newMockEnrollmentRepo()

	svc := NewService(achievementRepo, sessionRepo, logger)
	result := svc.WithEnrollmentRepo(enrollmentRepo)

	// Should return same service for chaining
	if result != svc {
		t.Error("WithEnrollmentRepo should return the same service instance")
	}

	// Verify the repo was set
	if svc.enrollmentRepo == nil {
		t.Error("enrollmentRepo should be set")
	}
}

func TestWithPathwayRepo(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()
	pathwayRepo := newMockPathwayRepo()

	svc := NewService(achievementRepo, sessionRepo, logger)
	result := svc.WithPathwayRepo(pathwayRepo)

	// Should return same service for chaining
	if result != svc {
		t.Error("WithPathwayRepo should return the same service instance")
	}

	// Verify the repo was set
	if svc.pathwayRepo == nil {
		t.Error("pathwayRepo should be set")
	}
}

// Tests for checkCategoryCriteria

func TestCheckCategoryCriteria_NoCategory(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()

	achievement := &models.Achievement{
		ID:       "ach-category",
		Name:     "Category Master",
		Type:     models.AchievementTypeCategory,
		IsActive: true,
		Criteria: models.AchievementCriteria{LabCategory: ""}, // Empty category
	}
	achievementRepo.achievements = append(achievementRepo.achievements, achievement)

	svc := NewService(achievementRepo, sessionRepo, logger)

	session := &models.Session{
		ID:     "session-1",
		UserID: "user-1",
		Passed: true,
	}

	awarded, err := svc.CheckAndAwardAchievements(context.Background(), session, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(awarded) != 0 {
		t.Errorf("expected no awards for empty category, got %d", len(awarded))
	}
}

func TestCheckCategoryCriteria_NoLabTemplateRepo(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()

	achievement := &models.Achievement{
		ID:       "ach-category",
		Name:     "Category Master",
		Type:     models.AchievementTypeCategory,
		IsActive: true,
		Criteria: models.AchievementCriteria{LabCategory: "security"},
	}
	achievementRepo.achievements = append(achievementRepo.achievements, achievement)

	// Don't add labTemplateRepo
	svc := NewService(achievementRepo, sessionRepo, logger)

	session := &models.Session{
		ID:     "session-1",
		UserID: "user-1",
		Passed: true,
	}

	awarded, err := svc.CheckAndAwardAchievements(context.Background(), session, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(awarded) != 0 {
		t.Errorf("expected no awards without labTemplateRepo, got %d", len(awarded))
	}
}

func TestCheckCategoryCriteria_NoTemplatesInCategory(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()
	labTemplateRepo := newMockLabTemplateRepo()

	achievement := &models.Achievement{
		ID:       "ach-category",
		Name:     "Category Master",
		Type:     models.AchievementTypeCategory,
		IsActive: true,
		Criteria: models.AchievementCriteria{LabCategory: "security"},
	}
	achievementRepo.achievements = append(achievementRepo.achievements, achievement)

	// No templates in category
	svc := NewService(achievementRepo, sessionRepo, logger).
		WithLabTemplateRepo(labTemplateRepo)

	session := &models.Session{
		ID:     "session-1",
		UserID: "user-1",
		Passed: true,
	}

	awarded, err := svc.CheckAndAwardAchievements(context.Background(), session, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(awarded) != 0 {
		t.Errorf("expected no awards with no templates in category, got %d", len(awarded))
	}
}

func TestCheckCategoryCriteria_PartialCompletion(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()
	labTemplateRepo := newMockLabTemplateRepo()

	achievement := &models.Achievement{
		ID:       "ach-category",
		Name:     "Category Master",
		Type:     models.AchievementTypeCategory,
		IsActive: true,
		Criteria: models.AchievementCriteria{LabCategory: "security"},
	}
	achievementRepo.achievements = append(achievementRepo.achievements, achievement)

	// Add 2 templates in security category
	labTemplateRepo.templates = []*models.LabTemplateRecord{
		{ID: "lab-1", Name: "Lab 1", Category: "security", IsActive: true},
		{ID: "lab-2", Name: "Lab 2", Category: "security", IsActive: true},
	}

	// User completed only 1 of 2
	endedAt := time.Now()
	sessionRepo.sessions = append(sessionRepo.sessions, &models.Session{
		ID:            "session-1",
		UserID:        "user-1",
		LabTemplateID: "lab-1",
		Passed:        true,
		EndedAt:       &endedAt,
	})

	svc := NewService(achievementRepo, sessionRepo, logger).
		WithLabTemplateRepo(labTemplateRepo)

	session := &models.Session{
		ID:            "session-1",
		UserID:        "user-1",
		LabTemplateID: "lab-1",
		Passed:        true,
		EndedAt:       &endedAt,
	}

	awarded, err := svc.CheckAndAwardAchievements(context.Background(), session, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(awarded) != 0 {
		t.Errorf("expected no awards for partial completion, got %d", len(awarded))
	}

	// Check progress was recorded
	progressKey := "user-1:ach-category"
	progress := achievementRepo.progress[progressKey]
	if progress == nil {
		t.Fatal("expected progress to be recorded")
	}
	if progress.Current != 1 {
		t.Errorf("expected progress.Current = 1, got %d", progress.Current)
	}
	if progress.Required != 2 {
		t.Errorf("expected progress.Required = 2, got %d", progress.Required)
	}
	if progress.Earned {
		t.Error("expected progress.Earned = false for partial completion")
	}
}

func TestCheckCategoryCriteria_FullCompletion(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()
	labTemplateRepo := newMockLabTemplateRepo()

	achievement := &models.Achievement{
		ID:       "ach-category",
		Name:     "Category Master",
		Type:     models.AchievementTypeCategory,
		IsActive: true,
		Criteria: models.AchievementCriteria{LabCategory: "security"},
	}
	achievementRepo.achievements = append(achievementRepo.achievements, achievement)

	// Add 2 templates in security category
	labTemplateRepo.templates = []*models.LabTemplateRecord{
		{ID: "lab-1", Name: "Lab 1", Category: "security", IsActive: true},
		{ID: "lab-2", Name: "Lab 2", Category: "security", IsActive: true},
	}

	// User completed both
	endedAt := time.Now()
	sessionRepo.sessions = []*models.Session{
		{ID: "session-1", UserID: "user-1", LabTemplateID: "lab-1", Passed: true, EndedAt: &endedAt},
		{ID: "session-2", UserID: "user-1", LabTemplateID: "lab-2", Passed: true, EndedAt: &endedAt},
	}

	svc := NewService(achievementRepo, sessionRepo, logger).
		WithLabTemplateRepo(labTemplateRepo)

	session := &models.Session{
		ID:            "session-2",
		UserID:        "user-1",
		LabTemplateID: "lab-2",
		Passed:        true,
		EndedAt:       &endedAt,
	}

	awarded, err := svc.CheckAndAwardAchievements(context.Background(), session, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(awarded) != 1 {
		t.Errorf("expected 1 award for full category completion, got %d", len(awarded))
	}
}

// Tests for checkPathwayCriteria

func TestCheckPathwayCriteria_NoRepos(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()

	achievement := &models.Achievement{
		ID:       "ach-pathway",
		Name:     "Pathway Progress",
		Type:     models.AchievementTypePathway,
		IsActive: true,
		Criteria: models.AchievementCriteria{
			Type:            "pathway_modules",
			PathwaySlug:     "intro-security",
			ModulesRequired: 3,
		},
	}
	achievementRepo.achievements = append(achievementRepo.achievements, achievement)

	// Don't add enrollment or pathway repos
	svc := NewService(achievementRepo, sessionRepo, logger)

	session := &models.Session{ID: "session-1", UserID: "user-1", Passed: true}

	awarded, err := svc.CheckAndAwardAchievements(context.Background(), session, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(awarded) != 0 {
		t.Errorf("expected no awards without repos, got %d", len(awarded))
	}
}

func TestCheckPathwayCriteria_PathwayNotFound(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()
	enrollmentRepo := newMockEnrollmentRepo()
	pathwayRepo := newMockPathwayRepo()

	achievement := &models.Achievement{
		ID:       "ach-pathway",
		Name:     "Pathway Progress",
		Type:     models.AchievementTypePathway,
		IsActive: true,
		Criteria: models.AchievementCriteria{
			Type:            "pathway_modules",
			PathwaySlug:     "non-existent",
			ModulesRequired: 3,
		},
	}
	achievementRepo.achievements = append(achievementRepo.achievements, achievement)

	svc := NewService(achievementRepo, sessionRepo, logger).
		WithEnrollmentRepo(enrollmentRepo).
		WithPathwayRepo(pathwayRepo)

	session := &models.Session{ID: "session-1", UserID: "user-1", Passed: true}

	awarded, err := svc.CheckAndAwardAchievements(context.Background(), session, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(awarded) != 0 {
		t.Errorf("expected no awards for non-existent pathway, got %d", len(awarded))
	}
}

func TestCheckPathwayCriteria_NotEnrolled(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()
	enrollmentRepo := newMockEnrollmentRepo()
	pathwayRepo := newMockPathwayRepo()

	// Add pathway
	pathwayRepo.pathways["pathway-1"] = &models.Pathway{
		ID:   "pathway-1",
		Slug: "intro-security",
		Name: "Introduction to Security",
	}

	achievement := &models.Achievement{
		ID:       "ach-pathway",
		Name:     "Pathway Progress",
		Type:     models.AchievementTypePathway,
		IsActive: true,
		Criteria: models.AchievementCriteria{
			Type:            "pathway_modules",
			PathwaySlug:     "intro-security",
			ModulesRequired: 3,
		},
	}
	achievementRepo.achievements = append(achievementRepo.achievements, achievement)

	// User not enrolled
	svc := NewService(achievementRepo, sessionRepo, logger).
		WithEnrollmentRepo(enrollmentRepo).
		WithPathwayRepo(pathwayRepo)

	session := &models.Session{ID: "session-1", UserID: "user-1", Passed: true}

	awarded, err := svc.CheckAndAwardAchievements(context.Background(), session, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(awarded) != 0 {
		t.Errorf("expected no awards when not enrolled, got %d", len(awarded))
	}
}

func TestCheckPathwayCriteria_PartialModules(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()
	enrollmentRepo := newMockEnrollmentRepo()
	pathwayRepo := newMockPathwayRepo()

	// Add pathway
	pathwayRepo.pathways["pathway-1"] = &models.Pathway{
		ID:   "pathway-1",
		Slug: "intro-security",
		Name: "Introduction to Security",
	}

	// Add enrollment with partial progress
	enrollmentRepo.enrollments["enroll-1"] = &models.PathwayEnrollment{
		ID:               "enroll-1",
		UserID:           "user-1",
		PathwayID:        "pathway-1",
		CompletedModules: 2, // Only 2 of 3 required
		TotalModules:     5,
	}

	achievement := &models.Achievement{
		ID:       "ach-pathway",
		Name:     "Pathway Progress",
		Type:     models.AchievementTypePathway,
		IsActive: true,
		Criteria: models.AchievementCriteria{
			Type:            "pathway_modules",
			PathwaySlug:     "intro-security",
			ModulesRequired: 3,
		},
	}
	achievementRepo.achievements = append(achievementRepo.achievements, achievement)

	svc := NewService(achievementRepo, sessionRepo, logger).
		WithEnrollmentRepo(enrollmentRepo).
		WithPathwayRepo(pathwayRepo)

	session := &models.Session{ID: "session-1", UserID: "user-1", Passed: true}

	awarded, err := svc.CheckAndAwardAchievements(context.Background(), session, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(awarded) != 0 {
		t.Errorf("expected no awards for partial modules, got %d", len(awarded))
	}

	// Check progress was recorded
	progressKey := "user-1:ach-pathway"
	progress := achievementRepo.progress[progressKey]
	if progress == nil {
		t.Fatal("expected progress to be recorded")
	}
	if progress.Current != 2 {
		t.Errorf("expected progress.Current = 2, got %d", progress.Current)
	}
}

func TestCheckPathwayCriteria_AchieveModulesRequired(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()
	enrollmentRepo := newMockEnrollmentRepo()
	pathwayRepo := newMockPathwayRepo()

	// Add pathway
	pathwayRepo.pathways["pathway-1"] = &models.Pathway{
		ID:   "pathway-1",
		Slug: "intro-security",
		Name: "Introduction to Security",
	}

	// Add enrollment with enough progress
	enrollmentRepo.enrollments["enroll-1"] = &models.PathwayEnrollment{
		ID:               "enroll-1",
		UserID:           "user-1",
		PathwayID:        "pathway-1",
		CompletedModules: 3, // Matches required
		TotalModules:     5,
	}

	achievement := &models.Achievement{
		ID:       "ach-pathway",
		Name:     "Pathway Progress",
		Type:     models.AchievementTypePathway,
		IsActive: true,
		Criteria: models.AchievementCriteria{
			Type:            "pathway_modules",
			PathwaySlug:     "intro-security",
			ModulesRequired: 3,
		},
	}
	achievementRepo.achievements = append(achievementRepo.achievements, achievement)

	svc := NewService(achievementRepo, sessionRepo, logger).
		WithEnrollmentRepo(enrollmentRepo).
		WithPathwayRepo(pathwayRepo)

	session := &models.Session{ID: "session-1", UserID: "user-1", Passed: true}

	awarded, err := svc.CheckAndAwardAchievements(context.Background(), session, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(awarded) != 1 {
		t.Errorf("expected 1 award for meeting modules required, got %d", len(awarded))
	}
}

func TestCheckPathwayCriteria_ByPathwayID(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()
	enrollmentRepo := newMockEnrollmentRepo()
	pathwayRepo := newMockPathwayRepo()

	// Add pathway
	pathwayRepo.pathways["pathway-1"] = &models.Pathway{
		ID:   "pathway-1",
		Slug: "intro-security",
		Name: "Introduction to Security",
	}

	// Add enrollment with enough progress
	enrollmentRepo.enrollments["enroll-1"] = &models.PathwayEnrollment{
		ID:               "enroll-1",
		UserID:           "user-1",
		PathwayID:        "pathway-1",
		CompletedModules: 3,
		TotalModules:     5,
	}

	achievement := &models.Achievement{
		ID:       "ach-pathway",
		Name:     "Pathway Progress",
		Type:     models.AchievementTypePathway,
		IsActive: true,
		Criteria: models.AchievementCriteria{
			Type:            "pathway_modules",
			PathwayID:       "pathway-1", // Use ID instead of slug
			ModulesRequired: 3,
		},
	}
	achievementRepo.achievements = append(achievementRepo.achievements, achievement)

	svc := NewService(achievementRepo, sessionRepo, logger).
		WithEnrollmentRepo(enrollmentRepo).
		WithPathwayRepo(pathwayRepo)

	session := &models.Session{ID: "session-1", UserID: "user-1", Passed: true}

	awarded, err := svc.CheckAndAwardAchievements(context.Background(), session, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(awarded) != 1 {
		t.Errorf("expected 1 award for meeting modules required (by ID), got %d", len(awarded))
	}
}

// Tests for checkPathwayCompleteCriteria

func TestCheckPathwayCompleteCriteria_NoRepos(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()

	achievement := &models.Achievement{
		ID:       "ach-complete",
		Name:     "Pathway Graduate",
		Type:     models.AchievementTypePathwayComplete,
		IsActive: true,
		Criteria: models.AchievementCriteria{
			Type:        "pathway_complete",
			PathwaySlug: "intro-security",
		},
	}
	achievementRepo.achievements = append(achievementRepo.achievements, achievement)

	// Don't add repos
	svc := NewService(achievementRepo, sessionRepo, logger)

	session := &models.Session{ID: "session-1", UserID: "user-1", Passed: true}

	awarded, err := svc.CheckAndAwardAchievements(context.Background(), session, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(awarded) != 0 {
		t.Errorf("expected no awards without repos, got %d", len(awarded))
	}
}

func TestCheckPathwayCompleteCriteria_NotComplete(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()
	enrollmentRepo := newMockEnrollmentRepo()
	pathwayRepo := newMockPathwayRepo()

	// Add pathway
	pathwayRepo.pathways["pathway-1"] = &models.Pathway{
		ID:   "pathway-1",
		Slug: "intro-security",
		Name: "Introduction to Security",
	}

	// Add incomplete enrollment
	enrollmentRepo.enrollments["enroll-1"] = &models.PathwayEnrollment{
		ID:               "enroll-1",
		UserID:           "user-1",
		PathwayID:        "pathway-1",
		Status:           models.EnrollmentStatusInProgress,
		CompletedModules: 3,
		TotalModules:     5,
		Percentage:       60.0,
	}

	achievement := &models.Achievement{
		ID:       "ach-complete",
		Name:     "Pathway Graduate",
		Type:     models.AchievementTypePathwayComplete,
		IsActive: true,
		Criteria: models.AchievementCriteria{
			Type:        "pathway_complete",
			PathwaySlug: "intro-security",
		},
	}
	achievementRepo.achievements = append(achievementRepo.achievements, achievement)

	svc := NewService(achievementRepo, sessionRepo, logger).
		WithEnrollmentRepo(enrollmentRepo).
		WithPathwayRepo(pathwayRepo)

	session := &models.Session{ID: "session-1", UserID: "user-1", Passed: true}

	awarded, err := svc.CheckAndAwardAchievements(context.Background(), session, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(awarded) != 0 {
		t.Errorf("expected no awards for incomplete pathway, got %d", len(awarded))
	}
}

func TestCheckPathwayCompleteCriteria_CompletedByStatus(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()
	enrollmentRepo := newMockEnrollmentRepo()
	pathwayRepo := newMockPathwayRepo()

	// Add pathway
	pathwayRepo.pathways["pathway-1"] = &models.Pathway{
		ID:   "pathway-1",
		Slug: "intro-security",
		Name: "Introduction to Security",
	}

	// Add completed enrollment by status
	enrollmentRepo.enrollments["enroll-1"] = &models.PathwayEnrollment{
		ID:               "enroll-1",
		UserID:           "user-1",
		PathwayID:        "pathway-1",
		Status:           models.EnrollmentStatusCompleted,
		CompletedModules: 5,
		TotalModules:     5,
		Percentage:       100.0,
	}

	achievement := &models.Achievement{
		ID:       "ach-complete",
		Name:     "Pathway Graduate",
		Type:     models.AchievementTypePathwayComplete,
		IsActive: true,
		Criteria: models.AchievementCriteria{
			Type:        "pathway_complete",
			PathwaySlug: "intro-security",
		},
	}
	achievementRepo.achievements = append(achievementRepo.achievements, achievement)

	svc := NewService(achievementRepo, sessionRepo, logger).
		WithEnrollmentRepo(enrollmentRepo).
		WithPathwayRepo(pathwayRepo)

	session := &models.Session{ID: "session-1", UserID: "user-1", Passed: true}

	awarded, err := svc.CheckAndAwardAchievements(context.Background(), session, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(awarded) != 1 {
		t.Errorf("expected 1 award for completed pathway, got %d", len(awarded))
	}
}

func TestCheckPathwayCompleteCriteria_CompletedByModuleCount(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()
	enrollmentRepo := newMockEnrollmentRepo()
	pathwayRepo := newMockPathwayRepo()

	// Add pathway
	pathwayRepo.pathways["pathway-1"] = &models.Pathway{
		ID:   "pathway-1",
		Slug: "intro-security",
		Name: "Introduction to Security",
	}

	// Add enrollment - completed by module count (status not set to completed yet)
	enrollmentRepo.enrollments["enroll-1"] = &models.PathwayEnrollment{
		ID:               "enroll-1",
		UserID:           "user-1",
		PathwayID:        "pathway-1",
		Status:           models.EnrollmentStatusInProgress,
		CompletedModules: 5,
		TotalModules:     5, // All modules completed
		Percentage:       100.0,
	}

	achievement := &models.Achievement{
		ID:       "ach-complete",
		Name:     "Pathway Graduate",
		Type:     models.AchievementTypePathwayComplete,
		IsActive: true,
		Criteria: models.AchievementCriteria{
			Type:        "pathway_complete",
			PathwaySlug: "intro-security",
		},
	}
	achievementRepo.achievements = append(achievementRepo.achievements, achievement)

	svc := NewService(achievementRepo, sessionRepo, logger).
		WithEnrollmentRepo(enrollmentRepo).
		WithPathwayRepo(pathwayRepo)

	session := &models.Session{ID: "session-1", UserID: "user-1", Passed: true}

	awarded, err := svc.CheckAndAwardAchievements(context.Background(), session, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(awarded) != 1 {
		t.Errorf("expected 1 award for completed pathway by module count, got %d", len(awarded))
	}
}

func TestCheckPathwayCompleteCriteria_ByPathwayID(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()
	enrollmentRepo := newMockEnrollmentRepo()
	pathwayRepo := newMockPathwayRepo()

	// Add pathway
	pathwayRepo.pathways["pathway-1"] = &models.Pathway{
		ID:   "pathway-1",
		Slug: "intro-security",
		Name: "Introduction to Security",
	}

	// Add completed enrollment
	enrollmentRepo.enrollments["enroll-1"] = &models.PathwayEnrollment{
		ID:               "enroll-1",
		UserID:           "user-1",
		PathwayID:        "pathway-1",
		Status:           models.EnrollmentStatusCompleted,
		CompletedModules: 5,
		TotalModules:     5,
	}

	achievement := &models.Achievement{
		ID:       "ach-complete",
		Name:     "Pathway Graduate",
		Type:     models.AchievementTypePathwayComplete,
		IsActive: true,
		Criteria: models.AchievementCriteria{
			Type:      "pathway_complete",
			PathwayID: "pathway-1", // Use ID instead of slug
		},
	}
	achievementRepo.achievements = append(achievementRepo.achievements, achievement)

	svc := NewService(achievementRepo, sessionRepo, logger).
		WithEnrollmentRepo(enrollmentRepo).
		WithPathwayRepo(pathwayRepo)

	session := &models.Session{ID: "session-1", UserID: "user-1", Passed: true}

	awarded, err := svc.CheckAndAwardAchievements(context.Background(), session, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(awarded) != 1 {
		t.Errorf("expected 1 award for completed pathway (by ID), got %d", len(awarded))
	}
}

// Additional edge case tests

func TestCheckPerfectScoreCriteria_MultiplePerfectScores(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()

	// Achievement requiring 3 perfect scores
	achievement := &models.Achievement{
		ID:       "ach-multi-perfect",
		Name:     "Triple Perfectionist",
		Type:     models.AchievementTypePerfectScore,
		IsActive: true,
		Criteria: models.AchievementCriteria{
			RequirePerfect: true,
			TotalLabs:      3,
		},
	}
	achievementRepo.achievements = append(achievementRepo.achievements, achievement)

	// Add 3 perfect sessions
	for i := 0; i < 3; i++ {
		sessionRepo.sessions = append(sessionRepo.sessions, &models.Session{
			ID:         "session-" + string(rune('1'+i)),
			UserID:     "user-1",
			Percentage: 100.0,
			Passed:     true,
		})
	}

	svc := NewService(achievementRepo, sessionRepo, logger)

	session := &models.Session{
		ID:         "session-3",
		UserID:     "user-1",
		Percentage: 100.0,
		Passed:     true,
	}

	awarded, err := svc.CheckAndAwardAchievements(context.Background(), session, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(awarded) != 1 {
		t.Errorf("expected 1 award for 3 perfect scores, got %d", len(awarded))
	}
}

func TestCheckPerfectScoreCriteria_NotEnoughPerfectScores(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()

	// Achievement requiring 3 perfect scores
	achievement := &models.Achievement{
		ID:       "ach-multi-perfect",
		Name:     "Triple Perfectionist",
		Type:     models.AchievementTypePerfectScore,
		IsActive: true,
		Criteria: models.AchievementCriteria{
			RequirePerfect: true,
			TotalLabs:      3,
		},
	}
	achievementRepo.achievements = append(achievementRepo.achievements, achievement)

	// Add 2 perfect sessions and 1 non-perfect
	sessionRepo.sessions = []*models.Session{
		{ID: "session-1", UserID: "user-1", Percentage: 100.0, Passed: true},
		{ID: "session-2", UserID: "user-1", Percentage: 100.0, Passed: true},
		{ID: "session-3", UserID: "user-1", Percentage: 90.0, Passed: true},
	}

	svc := NewService(achievementRepo, sessionRepo, logger)

	// Current session is perfect but still only 2 perfect total
	session := &models.Session{
		ID:         "session-1",
		UserID:     "user-1",
		Percentage: 100.0,
		Passed:     true,
	}

	awarded, err := svc.CheckAndAwardAchievements(context.Background(), session, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(awarded) != 0 {
		t.Errorf("expected no awards for only 2 perfect scores, got %d", len(awarded))
	}
}

func TestCheckStreakCriteria_BrokenStreak(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()

	achievement := &models.Achievement{
		ID:       "ach-streak",
		Name:     "Hat Trick",
		Type:     models.AchievementTypeStreak,
		IsActive: true,
		Criteria: models.AchievementCriteria{StreakCount: 3},
	}
	achievementRepo.achievements = append(achievementRepo.achievements, achievement)

	// Add 2 passed, then 1 failed, then 1 passed - streak is 1
	endedAt := time.Now()
	sessionRepo.sessions = []*models.Session{
		{ID: "session-1", UserID: "user-1", Passed: true, EndedAt: &endedAt},
		{ID: "session-2", UserID: "user-1", Passed: true, EndedAt: &endedAt},
		{ID: "session-3", UserID: "user-1", Passed: false, EndedAt: &endedAt}, // Breaks streak
		{ID: "session-4", UserID: "user-1", Passed: true, EndedAt: &endedAt},  // New streak of 1
	}

	svc := NewService(achievementRepo, sessionRepo, logger)

	session := &models.Session{
		ID:      "session-4",
		UserID:  "user-1",
		Passed:  true,
		EndedAt: &endedAt,
	}

	awarded, err := svc.CheckAndAwardAchievements(context.Background(), session, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(awarded) != 0 {
		t.Errorf("expected no awards with broken streak, got %d", len(awarded))
	}
}

func TestCheckSpeedCriteria_SessionNotEnded(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()

	achievement := &models.Achievement{
		ID:       "ach-speed",
		Name:     "Speed Demon",
		Type:     models.AchievementTypeSpeed,
		IsActive: true,
		Criteria: models.AchievementCriteria{MaxDurationMins: 10},
	}
	achievementRepo.achievements = append(achievementRepo.achievements, achievement)

	svc := NewService(achievementRepo, sessionRepo, logger)

	// Session has no EndedAt
	session := &models.Session{
		ID:        "session-1",
		UserID:    "user-1",
		StartedAt: time.Now().Add(-5 * time.Minute),
		EndedAt:   nil, // Not ended
		Passed:    true,
	}

	awarded, err := svc.CheckAndAwardAchievements(context.Background(), session, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(awarded) != 0 {
		t.Errorf("expected no awards for unended session, got %d", len(awarded))
	}
}

func TestCheckCriteria_UnknownType(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()

	// Achievement with unknown type
	achievement := &models.Achievement{
		ID:       "ach-unknown",
		Name:     "Unknown",
		Type:     models.AchievementType("unknown_type"),
		IsActive: true,
		Criteria: models.AchievementCriteria{},
	}
	achievementRepo.achievements = append(achievementRepo.achievements, achievement)

	svc := NewService(achievementRepo, sessionRepo, logger)

	session := &models.Session{
		ID:     "session-1",
		UserID: "user-1",
		Passed: true,
	}

	awarded, err := svc.CheckAndAwardAchievements(context.Background(), session, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(awarded) != 0 {
		t.Errorf("expected no awards for unknown type, got %d", len(awarded))
	}
}

func TestCheckSpecialCriteria_UnknownCustomRule(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()

	achievement := &models.Achievement{
		ID:       "ach-special",
		Name:     "Unknown Special",
		Type:     models.AchievementTypeSpecial,
		IsActive: true,
		Criteria: models.AchievementCriteria{CustomRule: "unknown_rule"},
	}
	achievementRepo.achievements = append(achievementRepo.achievements, achievement)

	svc := NewService(achievementRepo, sessionRepo, logger)

	endedAt := time.Now()
	session := &models.Session{
		ID:      "session-1",
		UserID:  "user-1",
		EndedAt: &endedAt,
		Passed:  true,
	}

	awarded, err := svc.CheckAndAwardAchievements(context.Background(), session, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(awarded) != 0 {
		t.Errorf("expected no awards for unknown custom rule, got %d", len(awarded))
	}
}

func TestCheckLabCompletionCriteria_ProgressOver100(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()

	// Achievement requiring 2 completed labs
	achievement := &models.Achievement{
		ID:       "ach-labs",
		Name:     "Lab Master",
		Type:     models.AchievementTypeLabCompletion,
		IsActive: true,
		Criteria: models.AchievementCriteria{TotalLabs: 2},
	}
	achievementRepo.achievements = append(achievementRepo.achievements, achievement)

	// Add 3 completed sessions (more than required)
	endedAt := time.Now()
	for i := 0; i < 3; i++ {
		sessionRepo.sessions = append(sessionRepo.sessions, &models.Session{
			ID:      "session-" + string(rune('1'+i)),
			UserID:  "user-1",
			Passed:  true,
			EndedAt: &endedAt,
		})
	}

	svc := NewService(achievementRepo, sessionRepo, logger)

	session := &models.Session{
		ID:      "session-3",
		UserID:  "user-1",
		Passed:  true,
		EndedAt: &endedAt,
	}

	awarded, err := svc.CheckAndAwardAchievements(context.Background(), session, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(awarded) != 1 {
		t.Errorf("expected 1 award, got %d", len(awarded))
	}

	// Progress should be capped at 100
	progressKey := "user-1:ach-labs"
	progress := achievementRepo.progress[progressKey]
	if progress != nil && progress.Progress > 100 {
		t.Errorf("expected progress capped at 100, got %f", progress.Progress)
	}
}

func TestCheckStreakCriteria_ProgressOver100(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()

	achievement := &models.Achievement{
		ID:       "ach-streak",
		Name:     "Hat Trick",
		Type:     models.AchievementTypeStreak,
		IsActive: true,
		Criteria: models.AchievementCriteria{StreakCount: 2},
	}
	achievementRepo.achievements = append(achievementRepo.achievements, achievement)

	// Add 5 passed sessions (more than required streak)
	endedAt := time.Now()
	for i := 0; i < 5; i++ {
		sessionRepo.sessions = append(sessionRepo.sessions, &models.Session{
			ID:      "session-" + string(rune('1'+i)),
			UserID:  "user-1",
			Passed:  true,
			EndedAt: &endedAt,
		})
	}

	svc := NewService(achievementRepo, sessionRepo, logger)

	session := &models.Session{
		ID:      "session-5",
		UserID:  "user-1",
		Passed:  true,
		EndedAt: &endedAt,
	}

	awarded, err := svc.CheckAndAwardAchievements(context.Background(), session, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(awarded) != 1 {
		t.Errorf("expected 1 award, got %d", len(awarded))
	}

	// Progress should be capped at 100
	progressKey := "user-1:ach-streak"
	progress := achievementRepo.progress[progressKey]
	if progress != nil && progress.Progress > 100 {
		t.Errorf("expected progress capped at 100, got %f", progress.Progress)
	}
}

func TestCheckPathwayCriteria_ProgressOver100(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()
	enrollmentRepo := newMockEnrollmentRepo()
	pathwayRepo := newMockPathwayRepo()

	// Add pathway
	pathwayRepo.pathways["pathway-1"] = &models.Pathway{
		ID:   "pathway-1",
		Slug: "intro-security",
		Name: "Introduction to Security",
	}

	// Add enrollment with more modules than required
	enrollmentRepo.enrollments["enroll-1"] = &models.PathwayEnrollment{
		ID:               "enroll-1",
		UserID:           "user-1",
		PathwayID:        "pathway-1",
		CompletedModules: 10, // Way more than required
		TotalModules:     15,
	}

	achievement := &models.Achievement{
		ID:       "ach-pathway",
		Name:     "Pathway Progress",
		Type:     models.AchievementTypePathway,
		IsActive: true,
		Criteria: models.AchievementCriteria{
			Type:            "pathway_modules",
			PathwaySlug:     "intro-security",
			ModulesRequired: 3,
		},
	}
	achievementRepo.achievements = append(achievementRepo.achievements, achievement)

	svc := NewService(achievementRepo, sessionRepo, logger).
		WithEnrollmentRepo(enrollmentRepo).
		WithPathwayRepo(pathwayRepo)

	session := &models.Session{ID: "session-1", UserID: "user-1", Passed: true}

	awarded, err := svc.CheckAndAwardAchievements(context.Background(), session, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(awarded) != 1 {
		t.Errorf("expected 1 award, got %d", len(awarded))
	}

	// Progress should be capped at 100
	progressKey := "user-1:ach-pathway"
	progress := achievementRepo.progress[progressKey]
	if progress != nil && progress.Progress > 100 {
		t.Errorf("expected progress capped at 100, got %f", progress.Progress)
	}
}

func TestCheckPathwayCriteria_ZeroModulesRequired(t *testing.T) {
	achievementRepo := newMockAchievementRepo()
	sessionRepo := newMockSessionRepo()
	logger := testLogger()
	enrollmentRepo := newMockEnrollmentRepo()
	pathwayRepo := newMockPathwayRepo()

	// Add pathway
	pathwayRepo.pathways["pathway-1"] = &models.Pathway{
		ID:   "pathway-1",
		Slug: "intro-security",
		Name: "Introduction to Security",
	}

	// Add enrollment with all modules completed
	enrollmentRepo.enrollments["enroll-1"] = &models.PathwayEnrollment{
		ID:               "enroll-1",
		UserID:           "user-1",
		PathwayID:        "pathway-1",
		CompletedModules: 5,
		TotalModules:     5,
	}

	// ModulesRequired = 0 means use TotalModules from enrollment
	achievement := &models.Achievement{
		ID:       "ach-pathway",
		Name:     "Pathway Progress",
		Type:     models.AchievementTypePathway,
		IsActive: true,
		Criteria: models.AchievementCriteria{
			Type:            "pathway_modules",
			PathwaySlug:     "intro-security",
			ModulesRequired: 0, // Zero means use total from enrollment
		},
	}
	achievementRepo.achievements = append(achievementRepo.achievements, achievement)

	svc := NewService(achievementRepo, sessionRepo, logger).
		WithEnrollmentRepo(enrollmentRepo).
		WithPathwayRepo(pathwayRepo)

	session := &models.Session{ID: "session-1", UserID: "user-1", Passed: true}

	awarded, err := svc.CheckAndAwardAchievements(context.Background(), session, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(awarded) != 1 {
		t.Errorf("expected 1 award for completing all modules, got %d", len(awarded))
	}
}

func (m *mockSessionRepo) CountActive(ctx context.Context) (int64, error) {
	return 0, nil
}
