package achievements

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// =============================================================================
// Mock Repositories for Benchmarking
// =============================================================================

type benchAchievementRepo struct {
	achievements     []*models.Achievement
	userAchievements map[string]*models.UserAchievement
	mu               sync.RWMutex
}

func newBenchAchievementRepo(numAchievements int) *benchAchievementRepo {
	achievements := make([]*models.Achievement, numAchievements)
	for i := 0; i < numAchievements; i++ {
		achievements[i] = &models.Achievement{
			ID:       fmt.Sprintf("ach-%d", i),
			Name:     fmt.Sprintf("Achievement %d", i),
			Type:     models.AchievementType([]string{"lab_completion", "perfect_score", "speed", "streak"}[i%4]),
			Tier:     models.AchievementTier([]string{"bronze", "silver", "gold", "platinum"}[i%4]),
			Points:   (i%5 + 1) * 10,
			IsActive: true,
			Criteria: models.AchievementCriteria{
				TotalLabs:       i%10 + 1,
				RequirePerfect:  i%4 == 1,
				MaxDurationMins: (i%3 + 1) * 5,
				StreakCount:     i%5 + 1,
			},
		}
	}
	return &benchAchievementRepo{
		achievements:     achievements,
		userAchievements: make(map[string]*models.UserAchievement),
	}
}

func (r *benchAchievementRepo) CreateAchievement(ctx context.Context, a *models.Achievement) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.achievements = append(r.achievements, a)
	return nil
}

func (r *benchAchievementRepo) GetAchievementByID(ctx context.Context, id string) (*models.Achievement, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, a := range r.achievements {
		if a.ID == id {
			return a, nil
		}
	}
	return nil, nil
}

func (r *benchAchievementRepo) GetAchievementsByIDs(ctx context.Context, ids []string) ([]*models.Achievement, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*models.Achievement
	for _, id := range ids {
		for _, a := range r.achievements {
			if a.ID == id {
				result = append(result, a)
				break
			}
		}
	}
	return result, nil
}

func (r *benchAchievementRepo) ListAchievements(ctx context.Context, filter repositories.AchievementFilter) ([]*models.Achievement, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if filter.IsActive != nil && *filter.IsActive {
		var active []*models.Achievement
		for _, a := range r.achievements {
			if a.IsActive {
				active = append(active, a)
			}
		}
		return active, nil
	}
	return r.achievements, nil
}

func (r *benchAchievementRepo) UpdateAchievement(ctx context.Context, a *models.Achievement) error {
	return nil
}

func (r *benchAchievementRepo) DeleteAchievement(ctx context.Context, id string) error {
	return nil
}

func (r *benchAchievementRepo) AwardAchievement(ctx context.Context, ua *models.UserAchievement) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := ua.UserID + ":" + ua.AchievementID
	r.userAchievements[key] = ua
	return nil
}

func (r *benchAchievementRepo) GetUserAchievement(ctx context.Context, userID, achievementID string) (*models.UserAchievement, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	key := userID + ":" + achievementID
	return r.userAchievements[key], nil
}

func (r *benchAchievementRepo) ListUserAchievements(ctx context.Context, userID string) ([]*models.UserAchievement, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*models.UserAchievement
	for key, ua := range r.userAchievements {
		if len(key) > len(userID) && key[:len(userID)] == userID {
			result = append(result, ua)
		}
	}
	return result, nil
}

func (r *benchAchievementRepo) GetUserAchievementSummary(ctx context.Context, userID string) (*models.UserAchievementSummary, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := 0
	for key := range r.userAchievements {
		if len(key) > len(userID) && key[:len(userID)] == userID {
			count++
		}
	}
	return &models.UserAchievementSummary{
		UserID:         userID,
		TotalEarned:    count,
		TotalAvailable: len(r.achievements),
	}, nil
}

func (r *benchAchievementRepo) GetRecentAchievements(ctx context.Context, limit int) ([]*models.UserAchievement, error) {
	return nil, nil
}

func (r *benchAchievementRepo) MarkNotified(ctx context.Context, userAchievementID string) error {
	return nil
}

func (r *benchAchievementRepo) UpdateProgress(ctx context.Context, progress *models.AchievementProgress) error {
	return nil
}

func (r *benchAchievementRepo) GetProgress(ctx context.Context, userID, achievementID string) (*models.AchievementProgress, error) {
	return nil, nil
}

func (r *benchAchievementRepo) ListUserProgress(ctx context.Context, userID string) ([]*models.AchievementProgress, error) {
	return nil, nil
}

func (r *benchAchievementRepo) ListAchievementsWithProgress(ctx context.Context, userID string) ([]*models.AchievementWithProgress, error) {
	return nil, nil
}

func (r *benchAchievementRepo) ListAllUserAchievements(ctx context.Context) ([]*models.UserAchievement, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*models.UserAchievement
	for _, ua := range r.userAchievements {
		result = append(result, ua)
	}
	return result, nil
}

func (r *benchAchievementRepo) GetLeaderboard(ctx context.Context, limit int, timeRange string) (*models.Leaderboard, error) {
	return &models.Leaderboard{
		Entries:    []*models.LeaderboardEntry{},
		TotalUsers: 0,
		UpdatedAt:  time.Now(),
		TimeRange:  timeRange,
	}, nil
}

type benchSessionRepo struct {
	sessions []*models.Session
	mu       sync.RWMutex
}

func newBenchSessionRepo(numSessions int, userID string) *benchSessionRepo {
	sessions := make([]*models.Session, numSessions)
	for i := 0; i < numSessions; i++ {
		endedAt := time.Now().Add(-time.Duration(i) * time.Hour)
		sessions[i] = &models.Session{
			ID:         fmt.Sprintf("session-%d", i),
			UserID:     userID,
			Passed:     i%3 != 0, // 2/3 pass rate
			Percentage: float64(70 + i%31),
			EndedAt:    &endedAt,
			StartedAt:  endedAt.Add(-30 * time.Minute),
		}
	}
	return &benchSessionRepo{sessions: sessions}
}

func (r *benchSessionRepo) List(ctx context.Context, filter repositories.SessionFilter) ([]*models.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if filter.UserID != "" {
		var result []*models.Session
		for _, s := range r.sessions {
			if s.UserID == filter.UserID {
				result = append(result, s)
			}
		}
		return result, nil
	}
	return r.sessions, nil
}

func (r *benchSessionRepo) Create(ctx context.Context, s *models.Session) error { return nil }
func (r *benchSessionRepo) GetByID(ctx context.Context, id string) (*models.Session, error) {
	return nil, nil
}
func (r *benchSessionRepo) GetByPodID(ctx context.Context, podID string) ([]*models.Session, error) {
	return nil, nil
}
func (r *benchSessionRepo) GetActiveByUserID(ctx context.Context, userID string) ([]*models.Session, error) {
	return nil, nil
}
func (r *benchSessionRepo) Update(ctx context.Context, s *models.Session) error { return nil }
func (r *benchSessionRepo) End(ctx context.Context, id string) error            { return nil }
func (r *benchSessionRepo) UpdateGrade(ctx context.Context, id string, earned int, passed bool) error {
	return nil
}
func (r *benchSessionRepo) MarkGradeSynced(ctx context.Context, id string, syncedAt time.Time) error {
	return nil
}
func (r *benchSessionRepo) MarkGradeSyncFailed(ctx context.Context, id string, errorMsg string) error {
	return nil
}
func (r *benchSessionRepo) GetUserID(ctx context.Context, id string) (string, error) { return "", nil }
func (r *benchSessionRepo) IsOwner(ctx context.Context, id, userID string) (bool, error) {
	return false, nil
}
func (r *benchSessionRepo) GetOrganizationID(ctx context.Context, id string) (*string, error) {
	return nil, nil
}
func (r *benchSessionRepo) ListAll(ctx context.Context) ([]*models.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sessions, nil
}

func (r *benchSessionRepo) EndStaleSessions(ctx context.Context, maxAge time.Duration) (int64, error) {
	return 0, nil
}

func (r *benchSessionRepo) DeleteEndedBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return 0, nil
}

func (r *benchSessionRepo) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, s := range r.sessions {
		if s.ID == id {
			r.sessions = append(r.sessions[:i], r.sessions[i+1:]...)
			return nil
		}
	}
	return nil
}

func (r *benchSessionRepo) GetUserStats(ctx context.Context, userID string) (*repositories.UserSessionStats, error) {
	return &repositories.UserSessionStats{}, nil
}

func (r *benchSessionRepo) ListWithLabNames(ctx context.Context, filter repositories.SessionFilter) ([]*repositories.SessionWithLabName, error) {
	return nil, nil
}

func (r *benchSessionRepo) CountCompletedLabsByUser(ctx context.Context) (map[string]int, error) {
	return nil, nil
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkCheckAndAwardAchievements_10Achievements(b *testing.B) {
	benchmarkCheckAchievements(b, 10, 5)
}

func BenchmarkCheckAndAwardAchievements_50Achievements(b *testing.B) {
	benchmarkCheckAchievements(b, 50, 10)
}

func BenchmarkCheckAndAwardAchievements_100Achievements(b *testing.B) {
	benchmarkCheckAchievements(b, 100, 20)
}

func BenchmarkCheckAndAwardAchievements_500Achievements(b *testing.B) {
	benchmarkCheckAchievements(b, 500, 50)
}

func benchmarkCheckAchievements(b *testing.B, numAchievements, numSessions int) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	achievementRepo := newBenchAchievementRepo(numAchievements)
	sessionRepo := newBenchSessionRepo(numSessions, "user-bench")
	service := NewService(achievementRepo, sessionRepo, logger)

	session := &models.Session{
		ID:         "session-current",
		UserID:     "user-bench",
		Passed:     true,
		Percentage: 100.0,
		StartedAt:  time.Now().Add(-10 * time.Minute),
	}
	endedAt := time.Now()
	session.EndedAt = &endedAt

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.CheckAndAwardAchievements(ctx, session, "user-bench")
	}
}

// BenchmarkConcurrentAchievementChecks tests concurrent achievement evaluation
func BenchmarkConcurrentAchievementChecks_10Users(b *testing.B) {
	benchmarkConcurrentChecks(b, 10, 50, 10)
}

func BenchmarkConcurrentAchievementChecks_50Users(b *testing.B) {
	benchmarkConcurrentChecks(b, 50, 50, 10)
}

func BenchmarkConcurrentAchievementChecks_100Users(b *testing.B) {
	benchmarkConcurrentChecks(b, 100, 50, 10)
}

func benchmarkConcurrentChecks(b *testing.B, numUsers, numAchievements, numSessionsPerUser int) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	achievementRepo := newBenchAchievementRepo(numAchievements)

	// Create a combined session repo with sessions for all users
	allSessions := make([]*models.Session, 0, numUsers*numSessionsPerUser)
	for u := 0; u < numUsers; u++ {
		userID := fmt.Sprintf("user-%d", u)
		for s := 0; s < numSessionsPerUser; s++ {
			endedAt := time.Now().Add(-time.Duration(s) * time.Hour)
			allSessions = append(allSessions, &models.Session{
				ID:         fmt.Sprintf("session-%d-%d", u, s),
				UserID:     userID,
				Passed:     s%3 != 0,
				Percentage: float64(70 + s%31),
				EndedAt:    &endedAt,
				StartedAt:  endedAt.Add(-30 * time.Minute),
			})
		}
	}
	sessionRepo := &benchSessionRepo{sessions: allSessions}

	service := NewService(achievementRepo, sessionRepo, logger)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		wg.Add(numUsers)

		for u := 0; u < numUsers; u++ {
			go func(userNum int) {
				defer wg.Done()
				userID := fmt.Sprintf("user-%d", userNum)
				session := &models.Session{
					ID:         fmt.Sprintf("session-current-%d", userNum),
					UserID:     userID,
					Passed:     true,
					Percentage: 100.0,
					StartedAt:  time.Now().Add(-10 * time.Minute),
				}
				endedAt := time.Now()
				session.EndedAt = &endedAt
				_, _ = service.CheckAndAwardAchievements(ctx, session, userID)
			}(u)
		}

		wg.Wait()
	}
}

// BenchmarkGetUserSummary tests user achievement summary retrieval
func BenchmarkGetUserSummary(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	achievementRepo := newBenchAchievementRepo(100)
	sessionRepo := newBenchSessionRepo(50, "user-bench")
	service := NewService(achievementRepo, sessionRepo, logger)

	// Award some achievements first
	ctx := context.Background()
	for i := 0; i < 25; i++ {
		_ = achievementRepo.AwardAchievement(ctx, &models.UserAchievement{
			ID:            fmt.Sprintf("ua-%d", i),
			UserID:        "user-bench",
			AchievementID: fmt.Sprintf("ach-%d", i),
			EarnedAt:      time.Now(),
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.GetUserSummary(ctx, "user-bench")
	}
}

// BenchmarkGetUserAchievements tests getting user achievements with progress
func BenchmarkGetUserAchievements_100(b *testing.B) {
	benchmarkGetUserAchievements(b, 100)
}

func BenchmarkGetUserAchievements_500(b *testing.B) {
	benchmarkGetUserAchievements(b, 500)
}

func BenchmarkGetUserAchievements_1000(b *testing.B) {
	benchmarkGetUserAchievements(b, 1000)
}

func benchmarkGetUserAchievements(b *testing.B, numAchievements int) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	achievementRepo := newBenchAchievementRepo(numAchievements)
	sessionRepo := newBenchSessionRepo(10, "user-bench")
	service := NewService(achievementRepo, sessionRepo, logger)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.GetUserAchievements(ctx, "user-bench")
	}
}

// =============================================================================
// Load Tests (run with -count=1 to avoid caching)
// =============================================================================

func TestLoadTest_ConcurrentSubmissions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	achievementRepo := newBenchAchievementRepo(50)
	sessionRepo := newBenchSessionRepo(100, "user-load")
	service := NewService(achievementRepo, sessionRepo, logger)

	ctx := context.Background()
	numConcurrent := 100
	numIterations := 10

	var wg sync.WaitGroup
	errors := make(chan error, numConcurrent*numIterations)
	durations := make(chan time.Duration, numConcurrent*numIterations)

	start := time.Now()

	for iter := 0; iter < numIterations; iter++ {
		wg.Add(numConcurrent)
		for i := 0; i < numConcurrent; i++ {
			go func(userNum, iteration int) {
				defer wg.Done()

				userID := fmt.Sprintf("user-%d", userNum)
				session := &models.Session{
					ID:         fmt.Sprintf("session-%d-%d", userNum, iteration),
					UserID:     userID,
					Passed:     true,
					Percentage: float64(70 + userNum%31),
					StartedAt:  time.Now().Add(-10 * time.Minute),
				}
				endedAt := time.Now()
				session.EndedAt = &endedAt

				opStart := time.Now()
				_, err := service.CheckAndAwardAchievements(ctx, session, userID)
				durations <- time.Since(opStart)

				if err != nil {
					errors <- err
				}
			}(i, iter)
		}
		wg.Wait()
	}

	close(errors)
	close(durations)

	totalDuration := time.Since(start)
	totalOps := numConcurrent * numIterations

	// Calculate statistics
	var totalOpDuration time.Duration
	var maxDuration time.Duration
	var minDuration = time.Hour
	opCount := 0

	for d := range durations {
		totalOpDuration += d
		opCount++
		if d > maxDuration {
			maxDuration = d
		}
		if d < minDuration {
			minDuration = d
		}
	}

	var errorCount int
	for range errors {
		errorCount++
	}

	avgDuration := totalOpDuration / time.Duration(opCount)
	opsPerSecond := float64(totalOps) / totalDuration.Seconds()

	t.Logf("=== Load Test Results ===")
	t.Logf("Total operations: %d", totalOps)
	t.Logf("Concurrent users: %d", numConcurrent)
	t.Logf("Iterations: %d", numIterations)
	t.Logf("Total duration: %v", totalDuration)
	t.Logf("Operations/second: %.2f", opsPerSecond)
	t.Logf("Avg operation time: %v", avgDuration)
	t.Logf("Min operation time: %v", minDuration)
	t.Logf("Max operation time: %v", maxDuration)
	t.Logf("Errors: %d", errorCount)
	t.Logf("=========================")

	// Assertions
	if errorCount > 0 {
		t.Errorf("Expected 0 errors, got %d", errorCount)
	}
	if opsPerSecond < 100 {
		t.Errorf("Expected at least 100 ops/sec, got %.2f", opsPerSecond)
	}
	if avgDuration > 100*time.Millisecond {
		t.Errorf("Expected avg duration < 100ms, got %v", avgDuration)
	}
}

func TestLoadTest_HighVolumeSessions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Simulate a user with many past sessions
	numPastSessions := 1000
	achievementRepo := newBenchAchievementRepo(100)
	sessionRepo := newBenchSessionRepo(numPastSessions, "user-highvol")
	service := NewService(achievementRepo, sessionRepo, logger)

	ctx := context.Background()

	session := &models.Session{
		ID:         "session-current",
		UserID:     "user-highvol",
		Passed:     true,
		Percentage: 100.0,
		StartedAt:  time.Now().Add(-10 * time.Minute),
	}
	endedAt := time.Now()
	session.EndedAt = &endedAt

	// Measure time for achievement check with high session history
	start := time.Now()
	numChecks := 100
	for i := 0; i < numChecks; i++ {
		_, err := service.CheckAndAwardAchievements(ctx, session, "user-highvol")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	}
	duration := time.Since(start)

	avgDuration := duration / time.Duration(numChecks)
	t.Logf("=== High Volume Session Test ===")
	t.Logf("Past sessions: %d", numPastSessions)
	t.Logf("Achievement checks: %d", numChecks)
	t.Logf("Total duration: %v", duration)
	t.Logf("Avg check time: %v", avgDuration)
	t.Logf("================================")

	// Should still be fast even with many past sessions
	if avgDuration > 50*time.Millisecond {
		t.Errorf("Expected avg duration < 50ms with %d sessions, got %v", numPastSessions, avgDuration)
	}
}

func (r *benchSessionRepo) CountActive(ctx context.Context) (int64, error) {
	return 0, nil
}
