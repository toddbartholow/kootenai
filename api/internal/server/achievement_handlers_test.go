package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/toddbartholow/kootenai/api/internal/achievements"
	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// -----------------------------------------------------------------------------
// Mock Achievement Repository for Handler Tests
// (Separate from integration_test.go mock to support error injection)
// -----------------------------------------------------------------------------

type achievementHandlerMockRepo struct {
	achievements         []*models.Achievement
	userAchievements     []*models.UserAchievement
	achievementsWithProg []*models.AchievementWithProgress
	userSummary          *models.UserAchievementSummary
	listErr              error
	getErr               error
	getRecentErr         error
	listWithProgressErr  error
	getSummaryErr        error
	achievementByID      map[string]*models.Achievement
}

func newAchievementHandlerMockRepo() *achievementHandlerMockRepo {
	return &achievementHandlerMockRepo{
		achievementByID: make(map[string]*models.Achievement),
	}
}

func (m *achievementHandlerMockRepo) ListAchievements(ctx context.Context, filter repositories.AchievementFilter) ([]*models.Achievement, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	// Apply filters
	var result []*models.Achievement
	for _, a := range m.achievements {
		if filter.Type != "" && a.Type != filter.Type {
			continue
		}
		if filter.Tier != "" && a.Tier != filter.Tier {
			continue
		}
		if filter.IsActive != nil && a.IsActive != *filter.IsActive {
			continue
		}
		result = append(result, a)
	}
	return result, nil
}

func (m *achievementHandlerMockRepo) GetAchievementByID(ctx context.Context, id string) (*models.Achievement, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.achievementByID[id], nil
}

func (m *achievementHandlerMockRepo) GetRecentAchievements(ctx context.Context, limit int) ([]*models.UserAchievement, error) {
	if m.getRecentErr != nil {
		return nil, m.getRecentErr
	}
	if len(m.userAchievements) <= limit {
		return m.userAchievements, nil
	}
	return m.userAchievements[:limit], nil
}

// Stub implementations for unused methods
func (m *achievementHandlerMockRepo) CreateAchievement(ctx context.Context, achievement *models.Achievement) error {
	return nil
}
func (m *achievementHandlerMockRepo) GetAchievementsByIDs(ctx context.Context, ids []string) ([]*models.Achievement, error) {
	return nil, nil
}
func (m *achievementHandlerMockRepo) UpdateAchievement(ctx context.Context, achievement *models.Achievement) error {
	return nil
}
func (m *achievementHandlerMockRepo) DeleteAchievement(ctx context.Context, id string) error {
	return nil
}
func (m *achievementHandlerMockRepo) AwardAchievement(ctx context.Context, userAchievement *models.UserAchievement) error {
	return nil
}
func (m *achievementHandlerMockRepo) GetUserAchievement(ctx context.Context, userID, achievementID string) (*models.UserAchievement, error) {
	return nil, nil
}
func (m *achievementHandlerMockRepo) ListUserAchievements(ctx context.Context, userID string) ([]*models.UserAchievement, error) {
	return nil, nil
}
func (m *achievementHandlerMockRepo) GetUserAchievementSummary(ctx context.Context, userID string) (*models.UserAchievementSummary, error) {
	if m.getSummaryErr != nil {
		return nil, m.getSummaryErr
	}
	return m.userSummary, nil
}
func (m *achievementHandlerMockRepo) MarkNotified(ctx context.Context, userAchievementID string) error {
	return nil
}
func (m *achievementHandlerMockRepo) UpdateProgress(ctx context.Context, progress *models.AchievementProgress) error {
	return nil
}
func (m *achievementHandlerMockRepo) GetProgress(ctx context.Context, userID, achievementID string) (*models.AchievementProgress, error) {
	return nil, nil
}
func (m *achievementHandlerMockRepo) ListUserProgress(ctx context.Context, userID string) ([]*models.AchievementProgress, error) {
	return nil, nil
}
func (m *achievementHandlerMockRepo) ListAchievementsWithProgress(ctx context.Context, userID string) ([]*models.AchievementWithProgress, error) {
	if m.listWithProgressErr != nil {
		return nil, m.listWithProgressErr
	}
	return m.achievementsWithProg, nil
}
func (m *achievementHandlerMockRepo) ListAllUserAchievements(ctx context.Context) ([]*models.UserAchievement, error) {
	return nil, nil
}
func (m *achievementHandlerMockRepo) GetLeaderboard(ctx context.Context, limit int, timeRange string) (*models.Leaderboard, error) {
	return nil, nil
}

// -----------------------------------------------------------------------------
// handleListAchievements Tests
// -----------------------------------------------------------------------------

func TestHandleListAchievements(t *testing.T) {
	t.Run("success - no repo returns empty list", func(t *testing.T) {
		srv := newTestServer(t)
		srv.achievementMgr.achievementRepo = nil

		req := httptest.NewRequest(http.MethodGet, "/api/v1/achievements", nil)
		rr := httptest.NewRecorder()

		srv.achievementMgr.handleListAchievements(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
	})

	t.Run("success - returns achievements from repo", func(t *testing.T) {
		srv := newTestServer(t)
		mockRepo := newAchievementHandlerMockRepo()
		mockRepo.achievements = []*models.Achievement{
			{
				ID:          "ach-1",
				Name:        "First Lab",
				Description: "Complete your first lab",
				Type:        models.AchievementTypeLabCompletion,
				Tier:        models.AchievementTierBronze,
				Points:      10,
				IsActive:    true,
			},
			{
				ID:          "ach-2",
				Name:        "Speed Demon",
				Description: "Complete a lab in under 10 minutes",
				Type:        models.AchievementTypeSpeed,
				Tier:        models.AchievementTierGold,
				Points:      50,
				IsActive:    true,
			},
		}
		srv.achievementMgr.achievementRepo = mockRepo

		req := httptest.NewRequest(http.MethodGet, "/api/v1/achievements", nil)
		rr := httptest.NewRecorder()

		srv.achievementMgr.handleListAchievements(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		count, ok := response["count"].(float64)
		if !ok || count != 2 {
			t.Errorf("expected count 2, got %v", response["count"])
		}
	})

	t.Run("success - filters by type", func(t *testing.T) {
		srv := newTestServer(t)
		mockRepo := newAchievementHandlerMockRepo()
		mockRepo.achievements = []*models.Achievement{
			{ID: "ach-1", Type: models.AchievementTypeLabCompletion, IsActive: true},
			{ID: "ach-2", Type: models.AchievementTypeSpeed, IsActive: true},
			{ID: "ach-3", Type: models.AchievementTypeLabCompletion, IsActive: true},
		}
		srv.achievementMgr.achievementRepo = mockRepo

		req := httptest.NewRequest(http.MethodGet, "/api/v1/achievements?type=lab_completion", nil)
		rr := httptest.NewRecorder()

		srv.achievementMgr.handleListAchievements(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response map[string]interface{}
		json.NewDecoder(rr.Body).Decode(&response)

		count, _ := response["count"].(float64)
		if count != 2 {
			t.Errorf("expected 2 lab_completion achievements, got %v", count)
		}
	})

	t.Run("success - filters by tier", func(t *testing.T) {
		srv := newTestServer(t)
		mockRepo := newAchievementHandlerMockRepo()
		mockRepo.achievements = []*models.Achievement{
			{ID: "ach-1", Tier: models.AchievementTierBronze, IsActive: true},
			{ID: "ach-2", Tier: models.AchievementTierGold, IsActive: true},
			{ID: "ach-3", Tier: models.AchievementTierGold, IsActive: true},
		}
		srv.achievementMgr.achievementRepo = mockRepo

		req := httptest.NewRequest(http.MethodGet, "/api/v1/achievements?tier=gold", nil)
		rr := httptest.NewRecorder()

		srv.achievementMgr.handleListAchievements(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response map[string]interface{}
		json.NewDecoder(rr.Body).Decode(&response)

		count, _ := response["count"].(float64)
		if count != 2 {
			t.Errorf("expected 2 gold achievements, got %v", count)
		}
	})

	t.Run("success - filters by active status", func(t *testing.T) {
		srv := newTestServer(t)
		mockRepo := newAchievementHandlerMockRepo()
		mockRepo.achievements = []*models.Achievement{
			{ID: "ach-1", IsActive: true},
			{ID: "ach-2", IsActive: false},
			{ID: "ach-3", IsActive: true},
		}
		srv.achievementMgr.achievementRepo = mockRepo

		req := httptest.NewRequest(http.MethodGet, "/api/v1/achievements?active=true", nil)
		rr := httptest.NewRecorder()

		srv.achievementMgr.handleListAchievements(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response map[string]interface{}
		json.NewDecoder(rr.Body).Decode(&response)

		count, _ := response["count"].(float64)
		if count != 2 {
			t.Errorf("expected 2 active achievements, got %v", count)
		}
	})

	t.Run("error - repo returns error", func(t *testing.T) {
		srv := newTestServer(t)
		mockRepo := newAchievementHandlerMockRepo()
		mockRepo.listErr = errors.New("database error")
		srv.achievementMgr.achievementRepo = mockRepo

		req := httptest.NewRequest(http.MethodGet, "/api/v1/achievements", nil)
		rr := httptest.NewRecorder()

		srv.achievementMgr.handleListAchievements(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// handleGetAchievement Tests
// -----------------------------------------------------------------------------

func TestHandleGetAchievement(t *testing.T) {
	t.Run("error - no repo", func(t *testing.T) {
		srv := newTestServer(t)
		srv.achievementMgr.achievementRepo = nil

		req := httptest.NewRequest(http.MethodGet, "/api/v1/achievements/ach-123", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("achievementID", "ach-123")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		rr := httptest.NewRecorder()
		srv.achievementMgr.handleGetAchievement(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("success - returns achievement by ID", func(t *testing.T) {
		srv := newTestServer(t)
		mockRepo := newAchievementHandlerMockRepo()
		now := time.Now()
		mockRepo.achievementByID["ach-123"] = &models.Achievement{
			ID:          "ach-123",
			Name:        "Test Achievement",
			Description: "A test achievement",
			Type:        models.AchievementTypeMilestone,
			Tier:        models.AchievementTierSilver,
			Points:      25,
			IsActive:    true,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		srv.achievementMgr.achievementRepo = mockRepo

		req := httptest.NewRequest(http.MethodGet, "/api/v1/achievements/ach-123", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("achievementID", "ach-123")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		rr := httptest.NewRecorder()
		srv.achievementMgr.handleGetAchievement(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response models.Achievement
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response.ID != "ach-123" {
			t.Errorf("expected achievement ID 'ach-123', got %s", response.ID)
		}
		if response.Name != "Test Achievement" {
			t.Errorf("expected name 'Test Achievement', got %s", response.Name)
		}
	})

	t.Run("not found - achievement does not exist", func(t *testing.T) {
		srv := newTestServer(t)
		mockRepo := newAchievementHandlerMockRepo()
		// Empty achievementByID map - no achievements exist
		srv.achievementMgr.achievementRepo = mockRepo

		req := httptest.NewRequest(http.MethodGet, "/api/v1/achievements/nonexistent", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("achievementID", "nonexistent")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		rr := httptest.NewRecorder()
		srv.achievementMgr.handleGetAchievement(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("error - repo returns error", func(t *testing.T) {
		srv := newTestServer(t)
		mockRepo := newAchievementHandlerMockRepo()
		mockRepo.getErr = errors.New("database connection error")
		srv.achievementMgr.achievementRepo = mockRepo

		req := httptest.NewRequest(http.MethodGet, "/api/v1/achievements/ach-123", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("achievementID", "ach-123")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		rr := httptest.NewRecorder()
		srv.achievementMgr.handleGetAchievement(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// handleListUserAchievements Tests
// -----------------------------------------------------------------------------

func TestHandleListUserAchievements(t *testing.T) {
	t.Run("success - no service returns empty list", func(t *testing.T) {
		srv := newTestServer(t)
		srv.achievementMgr.achievementService = nil

		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/user-123/achievements", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("userID", "user-123")
		ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
		ctx = auth.ContextWithUser(ctx, &auth.User{ID: "user-123", Roles: []string{"student"}})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		srv.achievementMgr.handleListUserAchievements(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
	})

	t.Run("success - returns user achievements", func(t *testing.T) {
		srv := newTestServer(t)
		mockRepo := newAchievementHandlerMockRepo()
		// Create a real achievements.Service with the mock repo
		achService := newTestAchievementService(mockRepo)
		srv.achievementMgr.achievementService = achService

		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/user-123/achievements", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("userID", "user-123")
		ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
		ctx = auth.ContextWithUser(ctx, &auth.User{ID: "user-123", Roles: []string{"student"}})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		srv.achievementMgr.handleListUserAchievements(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if _, ok := response["achievements"]; !ok {
			t.Error("expected achievements in response")
		}
	})

	t.Run("error - service returns error", func(t *testing.T) {
		srv := newTestServer(t)
		mockRepo := newAchievementHandlerMockRepo()
		mockRepo.listWithProgressErr = errors.New("database error")
		achService := newTestAchievementService(mockRepo)
		srv.achievementMgr.achievementService = achService

		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/user-123/achievements", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("userID", "user-123")
		ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
		ctx = auth.ContextWithUser(ctx, &auth.User{ID: "user-123", Roles: []string{"student"}})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		srv.achievementMgr.handleListUserAchievements(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// handleGetUserAchievementSummary Tests
// -----------------------------------------------------------------------------

func TestHandleGetUserAchievementSummary(t *testing.T) {
	t.Run("error - no service", func(t *testing.T) {
		srv := newTestServer(t)
		srv.achievementMgr.achievementService = nil

		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/user-123/achievements/summary", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("userID", "user-123")
		ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
		ctx = auth.ContextWithUser(ctx, &auth.User{ID: "user-123", Roles: []string{"student"}})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		srv.achievementMgr.handleGetUserAchievementSummary(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("success - returns summary", func(t *testing.T) {
		srv := newTestServer(t)
		mockRepo := newAchievementHandlerMockRepo()
		mockRepo.userSummary = &models.UserAchievementSummary{
			UserID:            "user-123",
			TotalEarned:       3,
			TotalPoints:       60,
			TotalAvailable:    100,
			CompletionPercent: 60.0,
		}
		achService := newTestAchievementService(mockRepo)
		srv.achievementMgr.achievementService = achService

		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/user-123/achievements/summary", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("userID", "user-123")
		ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
		ctx = auth.ContextWithUser(ctx, &auth.User{ID: "user-123", Roles: []string{"student"}})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		srv.achievementMgr.handleGetUserAchievementSummary(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response models.UserAchievementSummary
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response.TotalEarned != 3 {
			t.Errorf("expected totalEarned 3, got %d", response.TotalEarned)
		}
	})

	t.Run("error - service returns error", func(t *testing.T) {
		srv := newTestServer(t)
		mockRepo := newAchievementHandlerMockRepo()
		mockRepo.getSummaryErr = errors.New("database error")
		achService := newTestAchievementService(mockRepo)
		srv.achievementMgr.achievementService = achService

		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/user-123/achievements/summary", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("userID", "user-123")
		ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
		ctx = auth.ContextWithUser(ctx, &auth.User{ID: "user-123", Roles: []string{"student"}})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		srv.achievementMgr.handleGetUserAchievementSummary(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// handleGetRecentAchievements Tests
// -----------------------------------------------------------------------------

func TestHandleGetRecentAchievements(t *testing.T) {
	t.Run("success - no repo returns empty list", func(t *testing.T) {
		srv := newTestServer(t)
		srv.achievementMgr.achievementRepo = nil

		req := httptest.NewRequest(http.MethodGet, "/api/v1/achievements/recent", nil)
		rr := httptest.NewRecorder()

		srv.achievementMgr.handleGetRecentAchievements(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
	})

	t.Run("success - returns recent achievements", func(t *testing.T) {
		srv := newTestServer(t)
		mockRepo := newAchievementHandlerMockRepo()
		now := time.Now()
		mockRepo.userAchievements = []*models.UserAchievement{
			{
				ID:            "ua-1",
				UserID:        "user-1",
				AchievementID: "ach-1",
				EarnedAt:      now,
				Progress:      100,
			},
			{
				ID:            "ua-2",
				UserID:        "user-2",
				AchievementID: "ach-2",
				EarnedAt:      now.Add(-time.Hour),
				Progress:      100,
			},
		}
		srv.achievementMgr.achievementRepo = mockRepo

		req := httptest.NewRequest(http.MethodGet, "/api/v1/achievements/recent", nil)
		rr := httptest.NewRecorder()

		srv.achievementMgr.handleGetRecentAchievements(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		count, ok := response["count"].(float64)
		if !ok || count != 2 {
			t.Errorf("expected count 2, got %v", response["count"])
		}
	})

	t.Run("success - respects limit", func(t *testing.T) {
		srv := newTestServer(t)
		mockRepo := newAchievementHandlerMockRepo()
		now := time.Now()
		// Add more than 10 achievements (the default limit)
		for i := 0; i < 15; i++ {
			mockRepo.userAchievements = append(mockRepo.userAchievements, &models.UserAchievement{
				ID:            "ua-" + string(rune('a'+i)),
				UserID:        "user-1",
				AchievementID: "ach-" + string(rune('a'+i)),
				EarnedAt:      now.Add(-time.Duration(i) * time.Hour),
			})
		}
		srv.achievementMgr.achievementRepo = mockRepo

		req := httptest.NewRequest(http.MethodGet, "/api/v1/achievements/recent", nil)
		rr := httptest.NewRecorder()

		srv.achievementMgr.handleGetRecentAchievements(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response map[string]interface{}
		json.NewDecoder(rr.Body).Decode(&response)

		count, _ := response["count"].(float64)
		if count != 10 {
			t.Errorf("expected count 10 (limit), got %v", count)
		}
	})

	t.Run("error - repo returns error", func(t *testing.T) {
		srv := newTestServer(t)
		mockRepo := newAchievementHandlerMockRepo()
		mockRepo.getRecentErr = errors.New("database error")
		srv.achievementMgr.achievementRepo = mockRepo

		req := httptest.NewRequest(http.MethodGet, "/api/v1/achievements/recent", nil)
		rr := httptest.NewRecorder()

		srv.achievementMgr.handleGetRecentAchievements(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// WithAchievementService Tests
// -----------------------------------------------------------------------------

func TestWithAchievementService(t *testing.T) {
	srv := &Server{}

	// Initially nil (deps not yet allocated)
	if srv.deps != nil {
		t.Error("expected deps to be nil initially")
	}

	// Test that the option works (nil is valid for test)
	opt := WithAchievementService(nil)
	opt(srv)
	// deps allocated but field is nil since we passed nil
	if srv.ensureDeps().achievementService != nil {
		t.Error("expected achievementService to remain nil when set to nil")
	}
}

// -----------------------------------------------------------------------------
// WithAchievementRepo Tests
// -----------------------------------------------------------------------------

func TestWithAchievementRepo(t *testing.T) {
	srv := &Server{}

	// Initially nil (deps not yet allocated)
	if srv.deps != nil {
		t.Error("expected deps to be nil initially")
	}

	// Test that the option works (nil is valid for test)
	opt := WithAchievementRepo(nil)
	opt(srv)
	// deps allocated but field is nil since we passed nil
	if srv.ensureDeps().achievementRepo != nil {
		t.Error("expected achievementRepo to remain nil when set to nil")
	}
}

// -----------------------------------------------------------------------------
// Test Helper Functions
// -----------------------------------------------------------------------------

// newTestAchievementService creates an achievements.Service with a mock repository for testing
func newTestAchievementService(mockRepo *achievementHandlerMockRepo) *achievements.Service {
	return achievements.NewService(mockRepo, nil, newTestLogger())
}
