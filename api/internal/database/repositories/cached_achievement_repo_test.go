package repositories

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// mockAchievementRepo is a mock implementation of AchievementRepository
type mockAchievementRepo struct {
	createAchievementFn            func(ctx context.Context, achievement *models.Achievement) error
	getAchievementByIDFn           func(ctx context.Context, id string) (*models.Achievement, error)
	listAchievementsFn             func(ctx context.Context, filter AchievementFilter) ([]*models.Achievement, error)
	updateAchievementFn            func(ctx context.Context, achievement *models.Achievement) error
	deleteAchievementFn            func(ctx context.Context, id string) error
	awardAchievementFn             func(ctx context.Context, userAchievement *models.UserAchievement) error
	getUserAchievementFn           func(ctx context.Context, userID, achievementID string) (*models.UserAchievement, error)
	listUserAchievementsFn         func(ctx context.Context, userID string) ([]*models.UserAchievement, error)
	getUserAchievementSummaryFn    func(ctx context.Context, userID string) (*models.UserAchievementSummary, error)
	getRecentAchievementsFn        func(ctx context.Context, limit int) ([]*models.UserAchievement, error)
	markNotifiedFn                 func(ctx context.Context, userAchievementID string) error
	updateProgressFn               func(ctx context.Context, progress *models.AchievementProgress) error
	getProgressFn                  func(ctx context.Context, userID, achievementID string) (*models.AchievementProgress, error)
	listUserProgressFn             func(ctx context.Context, userID string) ([]*models.AchievementProgress, error)
	listAchievementsWithProgressFn func(ctx context.Context, userID string) ([]*models.AchievementWithProgress, error)
	listAllUserAchievementsFn      func(ctx context.Context) ([]*models.UserAchievement, error)
	getLeaderboardFn               func(ctx context.Context, limit int, timeRange string) (*models.Leaderboard, error)
}

func (m *mockAchievementRepo) CreateAchievement(ctx context.Context, achievement *models.Achievement) error {
	if m.createAchievementFn != nil {
		return m.createAchievementFn(ctx, achievement)
	}
	return nil
}

func (m *mockAchievementRepo) GetAchievementByID(ctx context.Context, id string) (*models.Achievement, error) {
	if m.getAchievementByIDFn != nil {
		return m.getAchievementByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockAchievementRepo) GetAchievementsByIDs(ctx context.Context, ids []string) ([]*models.Achievement, error) {
	return nil, nil
}

func (m *mockAchievementRepo) ListAchievements(ctx context.Context, filter AchievementFilter) ([]*models.Achievement, error) {
	if m.listAchievementsFn != nil {
		return m.listAchievementsFn(ctx, filter)
	}
	return nil, nil
}

func (m *mockAchievementRepo) UpdateAchievement(ctx context.Context, achievement *models.Achievement) error {
	if m.updateAchievementFn != nil {
		return m.updateAchievementFn(ctx, achievement)
	}
	return nil
}

func (m *mockAchievementRepo) DeleteAchievement(ctx context.Context, id string) error {
	if m.deleteAchievementFn != nil {
		return m.deleteAchievementFn(ctx, id)
	}
	return nil
}

func (m *mockAchievementRepo) AwardAchievement(ctx context.Context, userAchievement *models.UserAchievement) error {
	if m.awardAchievementFn != nil {
		return m.awardAchievementFn(ctx, userAchievement)
	}
	return nil
}

func (m *mockAchievementRepo) GetUserAchievement(ctx context.Context, userID, achievementID string) (*models.UserAchievement, error) {
	if m.getUserAchievementFn != nil {
		return m.getUserAchievementFn(ctx, userID, achievementID)
	}
	return nil, nil
}

func (m *mockAchievementRepo) ListUserAchievements(ctx context.Context, userID string) ([]*models.UserAchievement, error) {
	if m.listUserAchievementsFn != nil {
		return m.listUserAchievementsFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockAchievementRepo) GetUserAchievementSummary(ctx context.Context, userID string) (*models.UserAchievementSummary, error) {
	if m.getUserAchievementSummaryFn != nil {
		return m.getUserAchievementSummaryFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockAchievementRepo) GetRecentAchievements(ctx context.Context, limit int) ([]*models.UserAchievement, error) {
	if m.getRecentAchievementsFn != nil {
		return m.getRecentAchievementsFn(ctx, limit)
	}
	return nil, nil
}

func (m *mockAchievementRepo) MarkNotified(ctx context.Context, userAchievementID string) error {
	if m.markNotifiedFn != nil {
		return m.markNotifiedFn(ctx, userAchievementID)
	}
	return nil
}

func (m *mockAchievementRepo) UpdateProgress(ctx context.Context, progress *models.AchievementProgress) error {
	if m.updateProgressFn != nil {
		return m.updateProgressFn(ctx, progress)
	}
	return nil
}

func (m *mockAchievementRepo) GetProgress(ctx context.Context, userID, achievementID string) (*models.AchievementProgress, error) {
	if m.getProgressFn != nil {
		return m.getProgressFn(ctx, userID, achievementID)
	}
	return nil, nil
}

func (m *mockAchievementRepo) ListUserProgress(ctx context.Context, userID string) ([]*models.AchievementProgress, error) {
	if m.listUserProgressFn != nil {
		return m.listUserProgressFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockAchievementRepo) ListAchievementsWithProgress(ctx context.Context, userID string) ([]*models.AchievementWithProgress, error) {
	if m.listAchievementsWithProgressFn != nil {
		return m.listAchievementsWithProgressFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockAchievementRepo) ListAllUserAchievements(ctx context.Context) ([]*models.UserAchievement, error) {
	if m.listAllUserAchievementsFn != nil {
		return m.listAllUserAchievementsFn(ctx)
	}
	return nil, nil
}

func (m *mockAchievementRepo) GetLeaderboard(ctx context.Context, limit int, timeRange string) (*models.Leaderboard, error) {
	if m.getLeaderboardFn != nil {
		return m.getLeaderboardFn(ctx, limit, timeRange)
	}
	return &models.Leaderboard{}, nil
}

// -----------------------------------------------------------------------------
// Tests
// -----------------------------------------------------------------------------

func TestNewCachedAchievementRepo(t *testing.T) {
	mockRepo := &mockAchievementRepo{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// With logger
	repo := NewCachedAchievementRepo(mockRepo, nil, logger)
	if repo == nil {
		t.Fatal("expected non-nil repo")
	}
	if repo.repo != mockRepo {
		t.Error("repo not set correctly")
	}
	if repo.logger != logger {
		t.Error("logger not set correctly")
	}

	// With nil logger (should use default)
	repo2 := NewCachedAchievementRepo(mockRepo, nil, nil)
	if repo2.logger == nil {
		t.Error("expected default logger to be set")
	}
}

func TestCachedAchievementRepo_CreateAchievement(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	tests := []struct {
		name    string
		repoErr error
		wantErr bool
	}{
		{
			name:    "success",
			repoErr: nil,
			wantErr: false,
		},
		{
			name:    "repo error",
			repoErr: errors.New("database error"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockAchievementRepo{
				createAchievementFn: func(ctx context.Context, achievement *models.Achievement) error {
					return tt.repoErr
				},
			}

			repo := NewCachedAchievementRepo(mockRepo, nil, logger)
			err := repo.CreateAchievement(context.Background(), &models.Achievement{ID: "test"})

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestCachedAchievementRepo_GetAchievementByID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	tests := []struct {
		name        string
		achievement *models.Achievement
		repoErr     error
		wantErr     bool
	}{
		{
			name:        "success",
			achievement: &models.Achievement{ID: "ach-1", Name: "Test"},
			repoErr:     nil,
			wantErr:     false,
		},
		{
			name:        "not found",
			achievement: nil,
			repoErr:     nil,
			wantErr:     false,
		},
		{
			name:        "repo error",
			achievement: nil,
			repoErr:     errors.New("database error"),
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockAchievementRepo{
				getAchievementByIDFn: func(ctx context.Context, id string) (*models.Achievement, error) {
					return tt.achievement, tt.repoErr
				},
			}

			repo := NewCachedAchievementRepo(mockRepo, nil, logger)
			result, err := repo.GetAchievementByID(context.Background(), "ach-1")

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.achievement == nil && result != nil {
				t.Error("expected nil result")
			}
			if tt.achievement != nil && result == nil {
				t.Error("expected non-nil result")
			}
			if tt.achievement != nil && result != nil && result.ID != tt.achievement.ID {
				t.Errorf("ID mismatch: got %s, want %s", result.ID, tt.achievement.ID)
			}
		})
	}
}

func TestCachedAchievementRepo_ListAchievements(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	achievements := []*models.Achievement{
		{ID: "ach-1", Name: "Achievement 1"},
		{ID: "ach-2", Name: "Achievement 2"},
	}

	tests := []struct {
		name         string
		achievements []*models.Achievement
		repoErr      error
		wantErr      bool
	}{
		{
			name:         "success",
			achievements: achievements,
			repoErr:      nil,
			wantErr:      false,
		},
		{
			name:         "empty list",
			achievements: []*models.Achievement{},
			repoErr:      nil,
			wantErr:      false,
		},
		{
			name:         "repo error",
			achievements: nil,
			repoErr:      errors.New("database error"),
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockAchievementRepo{
				listAchievementsFn: func(ctx context.Context, filter AchievementFilter) ([]*models.Achievement, error) {
					return tt.achievements, tt.repoErr
				},
			}

			repo := NewCachedAchievementRepo(mockRepo, nil, logger)
			result, err := repo.ListAchievements(context.Background(), AchievementFilter{})

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(result) != len(tt.achievements) {
				t.Errorf("length mismatch: got %d, want %d", len(result), len(tt.achievements))
			}
		})
	}
}

func TestCachedAchievementRepo_UpdateAchievement(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	tests := []struct {
		name    string
		repoErr error
		wantErr bool
	}{
		{
			name:    "success",
			repoErr: nil,
			wantErr: false,
		},
		{
			name:    "repo error",
			repoErr: errors.New("database error"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockAchievementRepo{
				updateAchievementFn: func(ctx context.Context, achievement *models.Achievement) error {
					return tt.repoErr
				},
			}

			repo := NewCachedAchievementRepo(mockRepo, nil, logger)
			err := repo.UpdateAchievement(context.Background(), &models.Achievement{ID: "test"})

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestCachedAchievementRepo_DeleteAchievement(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	tests := []struct {
		name    string
		repoErr error
		wantErr bool
	}{
		{
			name:    "success",
			repoErr: nil,
			wantErr: false,
		},
		{
			name:    "repo error",
			repoErr: errors.New("database error"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockAchievementRepo{
				deleteAchievementFn: func(ctx context.Context, id string) error {
					return tt.repoErr
				},
			}

			repo := NewCachedAchievementRepo(mockRepo, nil, logger)
			err := repo.DeleteAchievement(context.Background(), "test-id")

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestCachedAchievementRepo_AwardAchievement(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	tests := []struct {
		name    string
		repoErr error
		wantErr bool
	}{
		{
			name:    "success",
			repoErr: nil,
			wantErr: false,
		},
		{
			name:    "repo error",
			repoErr: errors.New("database error"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockAchievementRepo{
				awardAchievementFn: func(ctx context.Context, ua *models.UserAchievement) error {
					return tt.repoErr
				},
			}

			repo := NewCachedAchievementRepo(mockRepo, nil, logger)
			err := repo.AwardAchievement(context.Background(), &models.UserAchievement{
				UserID:        "user-1",
				AchievementID: "ach-1",
			})

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestCachedAchievementRepo_GetUserAchievement(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	mockRepo := &mockAchievementRepo{
		getUserAchievementFn: func(ctx context.Context, userID, achievementID string) (*models.UserAchievement, error) {
			return &models.UserAchievement{
				UserID:        userID,
				AchievementID: achievementID,
			}, nil
		},
	}

	repo := NewCachedAchievementRepo(mockRepo, nil, logger)
	result, err := repo.GetUserAchievement(context.Background(), "user-1", "ach-1")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.UserID != "user-1" {
		t.Errorf("UserID = %s, want user-1", result.UserID)
	}
}

func TestCachedAchievementRepo_ListUserAchievements(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	achievements := []*models.UserAchievement{
		{UserID: "user-1", AchievementID: "ach-1"},
		{UserID: "user-1", AchievementID: "ach-2"},
	}

	mockRepo := &mockAchievementRepo{
		listUserAchievementsFn: func(ctx context.Context, userID string) ([]*models.UserAchievement, error) {
			return achievements, nil
		},
	}

	repo := NewCachedAchievementRepo(mockRepo, nil, logger)
	result, err := repo.ListUserAchievements(context.Background(), "user-1")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("length = %d, want 2", len(result))
	}
}

func TestCachedAchievementRepo_GetUserAchievementSummary(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	tests := []struct {
		name    string
		summary *models.UserAchievementSummary
		repoErr error
		wantErr bool
	}{
		{
			name: "success",
			summary: &models.UserAchievementSummary{
				TotalEarned: 5,
				TotalPoints: 100,
			},
			repoErr: nil,
			wantErr: false,
		},
		{
			name:    "nil summary",
			summary: nil,
			repoErr: nil,
			wantErr: false,
		},
		{
			name:    "repo error",
			summary: nil,
			repoErr: errors.New("database error"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockAchievementRepo{
				getUserAchievementSummaryFn: func(ctx context.Context, userID string) (*models.UserAchievementSummary, error) {
					return tt.summary, tt.repoErr
				},
			}

			repo := NewCachedAchievementRepo(mockRepo, nil, logger)
			result, err := repo.GetUserAchievementSummary(context.Background(), "user-1")

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.summary == nil && result != nil {
				t.Error("expected nil result")
			}
			if tt.summary != nil && result == nil {
				t.Error("expected non-nil result")
			}
		})
	}
}

func TestCachedAchievementRepo_GetRecentAchievements(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	achievements := []*models.UserAchievement{
		{UserID: "user-1", AchievementID: "ach-1"},
	}

	mockRepo := &mockAchievementRepo{
		getRecentAchievementsFn: func(ctx context.Context, limit int) ([]*models.UserAchievement, error) {
			if limit != 10 {
				return nil, errors.New("unexpected limit")
			}
			return achievements, nil
		},
	}

	repo := NewCachedAchievementRepo(mockRepo, nil, logger)
	result, err := repo.GetRecentAchievements(context.Background(), 10)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("length = %d, want 1", len(result))
	}
}

func TestCachedAchievementRepo_MarkNotified(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	mockRepo := &mockAchievementRepo{
		markNotifiedFn: func(ctx context.Context, id string) error {
			if id != "ua-123" {
				return errors.New("unexpected ID")
			}
			return nil
		},
	}

	repo := NewCachedAchievementRepo(mockRepo, nil, logger)
	err := repo.MarkNotified(context.Background(), "ua-123")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCachedAchievementRepo_UpdateProgress(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	tests := []struct {
		name    string
		repoErr error
		wantErr bool
	}{
		{
			name:    "success",
			repoErr: nil,
			wantErr: false,
		},
		{
			name:    "repo error",
			repoErr: errors.New("database error"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockAchievementRepo{
				updateProgressFn: func(ctx context.Context, progress *models.AchievementProgress) error {
					return tt.repoErr
				},
			}

			repo := NewCachedAchievementRepo(mockRepo, nil, logger)
			err := repo.UpdateProgress(context.Background(), &models.AchievementProgress{
				UserID:        "user-1",
				AchievementID: "ach-1",
			})

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestCachedAchievementRepo_GetProgress(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	mockRepo := &mockAchievementRepo{
		getProgressFn: func(ctx context.Context, userID, achievementID string) (*models.AchievementProgress, error) {
			return &models.AchievementProgress{
				UserID:        userID,
				AchievementID: achievementID,
				Current:       5,
			}, nil
		},
	}

	repo := NewCachedAchievementRepo(mockRepo, nil, logger)
	result, err := repo.GetProgress(context.Background(), "user-1", "ach-1")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Current != 5 {
		t.Errorf("Current = %d, want 5", result.Current)
	}
}

func TestCachedAchievementRepo_ListUserProgress(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	progress := []*models.AchievementProgress{
		{UserID: "user-1", AchievementID: "ach-1"},
	}

	mockRepo := &mockAchievementRepo{
		listUserProgressFn: func(ctx context.Context, userID string) ([]*models.AchievementProgress, error) {
			return progress, nil
		},
	}

	repo := NewCachedAchievementRepo(mockRepo, nil, logger)
	result, err := repo.ListUserProgress(context.Background(), "user-1")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("length = %d, want 1", len(result))
	}
}

func TestCachedAchievementRepo_ListAchievementsWithProgress(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	achievements := []*models.AchievementWithProgress{
		{Achievement: models.Achievement{ID: "ach-1"}},
	}

	tests := []struct {
		name         string
		achievements []*models.AchievementWithProgress
		repoErr      error
		wantErr      bool
	}{
		{
			name:         "success",
			achievements: achievements,
			repoErr:      nil,
			wantErr:      false,
		},
		{
			name:         "repo error",
			achievements: nil,
			repoErr:      errors.New("database error"),
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockAchievementRepo{
				listAchievementsWithProgressFn: func(ctx context.Context, userID string) ([]*models.AchievementWithProgress, error) {
					return tt.achievements, tt.repoErr
				},
			}

			repo := NewCachedAchievementRepo(mockRepo, nil, logger)
			result, err := repo.ListAchievementsWithProgress(context.Background(), "user-1")

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(result) != len(tt.achievements) {
				t.Errorf("length = %d, want %d", len(result), len(tt.achievements))
			}
		})
	}
}

func TestCachedAchievementRepo_ListAllUserAchievements(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	achievements := []*models.UserAchievement{
		{UserID: "user-1", AchievementID: "ach-1"},
		{UserID: "user-2", AchievementID: "ach-2"},
	}

	mockRepo := &mockAchievementRepo{
		listAllUserAchievementsFn: func(ctx context.Context) ([]*models.UserAchievement, error) {
			return achievements, nil
		},
	}

	repo := NewCachedAchievementRepo(mockRepo, nil, logger)
	result, err := repo.ListAllUserAchievements(context.Background())

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("length = %d, want 2", len(result))
	}
}

func TestHashAchievementFilter(t *testing.T) {
	filter1 := AchievementFilter{Type: "milestone"}
	filter2 := AchievementFilter{Type: "milestone"}
	filter3 := AchievementFilter{Type: "streak"}

	hash1 := hashAchievementFilter(filter1)
	hash2 := hashAchievementFilter(filter2)
	hash3 := hashAchievementFilter(filter3)

	// Same filter should produce same hash
	if hash1 != hash2 {
		t.Errorf("same filters should produce same hash: %s != %s", hash1, hash2)
	}

	// Different filters should produce different hash
	if hash1 == hash3 {
		t.Error("different filters should produce different hash")
	}

	// Hash should be 16 characters (8 bytes hex encoded)
	if len(hash1) != 16 {
		t.Errorf("hash length = %d, want 16", len(hash1))
	}
}

func TestCachedAchievementRepo_GetLeaderboard(t *testing.T) {
	expectedLeaderboard := &models.Leaderboard{
		Entries: []*models.LeaderboardEntry{
			{Rank: 1, UserID: "user1", TotalPoints: 100},
			{Rank: 2, UserID: "user2", TotalPoints: 50},
		},
		TotalUsers: 2,
		TimeRange:  "all",
	}

	mockRepo := &mockAchievementRepo{
		getLeaderboardFn: func(ctx context.Context, limit int, timeRange string) (*models.Leaderboard, error) {
			if limit != 10 {
				t.Errorf("limit = %d, want 10", limit)
			}
			if timeRange != "all" {
				t.Errorf("timeRange = %q, want 'all'", timeRange)
			}
			return expectedLeaderboard, nil
		},
	}

	repo := NewCachedAchievementRepo(mockRepo, nil, nil)
	result, err := repo.GetLeaderboard(context.Background(), 10, "all")
	if err != nil {
		t.Fatalf("GetLeaderboard() error = %v", err)
	}
	if result != expectedLeaderboard {
		t.Error("GetLeaderboard() should return underlying repo result")
	}
	if len(result.Entries) != 2 {
		t.Errorf("entries length = %d, want 2", len(result.Entries))
	}
}
