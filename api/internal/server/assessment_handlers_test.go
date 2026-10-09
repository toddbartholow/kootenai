package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/checkpoint"
	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/testutil/mocks"
	"github.com/toddbartholow/kootenai/api/internal/websocket"
)

// -----------------------------------------------------------------------------
// Mock Assessment Result Repository (enhanced version for comprehensive testing)
// -----------------------------------------------------------------------------

type testAssessmentResultRepository struct {
	mu      sync.RWMutex
	results map[string]*models.AssessmentResult

	// Error injection
	CreateErr         error
	GetBySessionIDErr error
	UpdateErr         error
	UpdateStatusErr   error
	DeleteErr         error
	DeleteExpiredErr  error

	// Call tracking
	CreateCalls         []models.AssessmentResult
	GetBySessionIDCalls []string
	DeleteExpiredCalls  []time.Time
	DeletedCount        int64
}

func newTestAssessmentResultRepository() *testAssessmentResultRepository {
	return &testAssessmentResultRepository{
		results: make(map[string]*models.AssessmentResult),
	}
}

func (r *testAssessmentResultRepository) Create(ctx context.Context, result *models.AssessmentResult) error {
	if r.CreateErr != nil {
		return r.CreateErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.CreateCalls = append(r.CreateCalls, *result)
	r.results[result.SessionID] = result
	return nil
}

func (r *testAssessmentResultRepository) GetBySessionID(ctx context.Context, sessionID string) (*models.AssessmentResult, error) {
	if r.GetBySessionIDErr != nil {
		return nil, r.GetBySessionIDErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.GetBySessionIDCalls = append(r.GetBySessionIDCalls, sessionID)
	result, ok := r.results[sessionID]
	if !ok {
		return nil, nil
	}
	return result, nil
}

func (r *testAssessmentResultRepository) Update(ctx context.Context, result *models.AssessmentResult) error {
	if r.UpdateErr != nil {
		return r.UpdateErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.results[result.SessionID] = result
	return nil
}

func (r *testAssessmentResultRepository) UpdateStatus(ctx context.Context, sessionID string, status string) error {
	if r.UpdateStatusErr != nil {
		return r.UpdateStatusErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if result, ok := r.results[sessionID]; ok {
		result.Status = status
	}
	return nil
}

func (r *testAssessmentResultRepository) Delete(ctx context.Context, sessionID string) error {
	if r.DeleteErr != nil {
		return r.DeleteErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.results, sessionID)
	return nil
}

func (r *testAssessmentResultRepository) DeleteExpired(ctx context.Context, olderThan time.Time) (int64, error) {
	if r.DeleteExpiredErr != nil {
		return 0, r.DeleteExpiredErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.DeleteExpiredCalls = append(r.DeleteExpiredCalls, olderThan)
	return r.DeletedCount, nil
}

func (r *testAssessmentResultRepository) AddResult(result *models.AssessmentResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.results[result.SessionID] = result
}

// -----------------------------------------------------------------------------
// Test Helper Functions
// -----------------------------------------------------------------------------

func createTestServerForAssessment(t *testing.T, sessionRepo *mocks.FakeSessionRepository, eval *checkpoint.Evaluator, orch *mocks.FakeOrchestrator) *Server {
	t.Helper()
	cfg := DefaultConfig()
	wsHub := websocket.NewHub(newTestLogger())

	var opts []ServerOption
	if sessionRepo != nil {
		opts = append(opts, WithSessionRepo(sessionRepo))
	}
	if orch != nil {
		opts = append(opts, WithOrchestrator(orch))
	}

	srv, _ := New(cfg, nil, eval, wsHub, newTestLogger(), opts...)
	return srv
}

// -----------------------------------------------------------------------------
// AssessmentManager Unit Tests
// -----------------------------------------------------------------------------

func TestAssessmentManagerStoreResultInMemory(t *testing.T) {
	srv := newTestServer(t)
	am := srv.assessmentMgr

	result := &models.AssessmentResult{
		SessionID:  "session-1",
		Score:      80,
		MaxScore:   100,
		Percentage: 80.0,
		Status:     "completed",
	}

	am.storeResult(context.Background(), "session-1", result)

	if am.ResultCount() != 1 {
		t.Errorf("expected 1 cached result, got %d", am.ResultCount())
	}

	retrieved, ok := am.getResult(context.Background(), "session-1")
	if !ok {
		t.Fatal("expected to retrieve cached result")
	}
	if retrieved.Score != 80 {
		t.Errorf("expected score 80, got %d", retrieved.Score)
	}
}

func TestAssessmentManagerStoreResultWithPersistence(t *testing.T) {
	assessmentRepo := newTestAssessmentResultRepository()
	srv := newTestServer(t)
	am := srv.assessmentMgr
	am.repo = assessmentRepo

	result := &models.AssessmentResult{
		SessionID:  "session-1",
		Score:      80,
		MaxScore:   100,
		Percentage: 80.0,
		Status:     "completed",
	}

	am.storeResult(context.Background(), "session-1", result)

	// Verify database persistence
	if len(assessmentRepo.CreateCalls) != 1 {
		t.Errorf("expected 1 Create call, got %d", len(assessmentRepo.CreateCalls))
	}
	if assessmentRepo.CreateCalls[0].SessionID != "session-1" {
		t.Errorf("expected sessionID 'session-1', got '%s'", assessmentRepo.CreateCalls[0].SessionID)
	}
}

func TestAssessmentManagerStoreResultDatabaseError(t *testing.T) {
	assessmentRepo := newTestAssessmentResultRepository()
	assessmentRepo.CreateErr = errors.New("database error")
	srv := newTestServer(t)
	am := srv.assessmentMgr
	am.repo = assessmentRepo

	result := &models.AssessmentResult{
		SessionID: "session-1",
		Score:     80,
	}

	// Should not panic, just log error
	am.storeResult(context.Background(), "session-1", result)

	// Result should still be in memory cache
	if am.ResultCount() != 1 {
		t.Errorf("expected result in memory cache despite db error")
	}
}

func TestAssessmentManagerGetResultFromCache(t *testing.T) {
	srv := newTestServer(t)
	am := srv.assessmentMgr

	result := &models.AssessmentResult{
		SessionID: "session-1",
		Score:     80,
	}
	am.storeResult(context.Background(), "session-1", result)

	retrieved, ok := am.getResult(context.Background(), "session-1")
	if !ok {
		t.Fatal("expected to retrieve result")
	}
	if retrieved.Score != 80 {
		t.Errorf("expected score 80, got %d", retrieved.Score)
	}
}

func TestAssessmentManagerGetResultMissingSession(t *testing.T) {
	srv := newTestServer(t)
	am := srv.assessmentMgr

	_, ok := am.getResult(context.Background(), "nonexistent")
	if ok {
		t.Error("expected not found for missing result")
	}
}

func TestAssessmentManagerGetResultFromDatabase(t *testing.T) {
	assessmentRepo := newTestAssessmentResultRepository()
	assessmentRepo.AddResult(&models.AssessmentResult{
		SessionID: "session-1",
		Score:     75,
	})

	srv := newTestServer(t)
	am := srv.assessmentMgr
	am.repo = assessmentRepo

	retrieved, ok := am.getResult(context.Background(), "session-1")
	if !ok {
		t.Fatal("expected to retrieve result from database")
	}
	if retrieved.Score != 75 {
		t.Errorf("expected score 75, got %d", retrieved.Score)
	}

	// Should now be cached
	if am.ResultCount() != 1 {
		t.Errorf("expected result to be cached after retrieval")
	}
}

func TestAssessmentManagerGetResultDatabaseError(t *testing.T) {
	assessmentRepo := newTestAssessmentResultRepository()
	assessmentRepo.GetBySessionIDErr = errors.New("database error")

	srv := newTestServer(t)
	am := srv.assessmentMgr
	am.repo = assessmentRepo

	_, ok := am.getResult(context.Background(), "session-1")
	if ok {
		t.Error("expected not found when database error occurs")
	}
}

func TestAssessmentManagerResultCountMultiple(t *testing.T) {
	srv := newTestServer(t)
	am := srv.assessmentMgr

	if am.ResultCount() != 0 {
		t.Errorf("expected 0 results initially, got %d", am.ResultCount())
	}

	am.storeResult(context.Background(), "session-1", &models.AssessmentResult{SessionID: "session-1"})
	am.storeResult(context.Background(), "session-2", &models.AssessmentResult{SessionID: "session-2"})

	if am.ResultCount() != 2 {
		t.Errorf("expected 2 results, got %d", am.ResultCount())
	}
}

func TestAssessmentManagerCleanupExpiredFromCache(t *testing.T) {
	srv := newTestServer(t)
	am := srv.assessmentMgr
	am.ttl = 1 * time.Millisecond // Very short TTL for testing

	am.storeResult(context.Background(), "session-1", &models.AssessmentResult{SessionID: "session-1"})

	// Wait for TTL to expire
	time.Sleep(10 * time.Millisecond)

	// Manually trigger cleanup
	am.cleanupExpired(context.Background())

	if am.ResultCount() != 0 {
		t.Errorf("expected 0 results after cleanup, got %d", am.ResultCount())
	}
}

func TestAssessmentManagerCleanupExpiredFromDatabase(t *testing.T) {
	assessmentRepo := newTestAssessmentResultRepository()
	assessmentRepo.DeletedCount = 5

	srv := newTestServer(t)
	am := srv.assessmentMgr
	am.repo = assessmentRepo
	am.ttl = 1 * time.Hour

	am.cleanupExpired(context.Background())

	if len(assessmentRepo.DeleteExpiredCalls) != 1 {
		t.Errorf("expected 1 DeleteExpired call, got %d", len(assessmentRepo.DeleteExpiredCalls))
	}
}

func TestAssessmentManagerCleanupExpiredDatabaseError(t *testing.T) {
	assessmentRepo := newTestAssessmentResultRepository()
	assessmentRepo.DeleteExpiredErr = errors.New("database error")

	srv := newTestServer(t)
	am := srv.assessmentMgr
	am.repo = assessmentRepo

	// Should not panic
	am.cleanupExpired(context.Background())
}

// -----------------------------------------------------------------------------
// Handler Tests - handleGetAssessment (comprehensive)
// -----------------------------------------------------------------------------

// Removed TestHandleGetAssessmentServiceUnavailable and its three siblings:
// they set srv.assessmentMgr = nil after construction to exercise a
// handler-level nil check that no longer exists now that the manager owns
// its own routes. Zeroing the struct field doesn't affect routes that
// captured the manager reference at setup time, so the tests no longer
// exercise what they claim to. Nil-manager behavior is covered by the
// standard "manager not configured → route not registered → 404" contract
// that other managers already rely on.

func TestHandleGetAssessmentMissing(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/assessment/nonexistent", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestHandleGetAssessmentSuccess(t *testing.T) {
	srv := newTestServer(t)
	am := srv.assessmentMgr

	result := &models.AssessmentResult{
		SessionID:   "session-1",
		Score:       85,
		MaxScore:    100,
		Percentage:  85.0,
		ItemCount:   10,
		PassedCount: 8,
		Status:      "completed",
	}
	am.storeResult(context.Background(), "session-1", result)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/assessment/session-1", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response models.AssessmentResult
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.SessionID != "session-1" {
		t.Errorf("expected sessionId 'session-1', got '%s'", response.SessionID)
	}
	if response.Score != 85 {
		t.Errorf("expected score 85, got %d", response.Score)
	}
	if response.Percentage != 85.0 {
		t.Errorf("expected percentage 85.0, got %f", response.Percentage)
	}
}

// -----------------------------------------------------------------------------
// Handler Tests - handleGetAssessmentStatus (comprehensive)
// -----------------------------------------------------------------------------

func TestHandleGetAssessmentStatusNotStarted(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/assessment/session-1/status", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["sessionId"] != "session-1" {
		t.Errorf("expected sessionId 'session-1', got '%v'", response["sessionId"])
	}
	if response["status"] != "not_started" {
		t.Errorf("expected status 'not_started', got '%v'", response["status"])
	}
}

func TestHandleGetAssessmentStatusSuccess(t *testing.T) {
	srv := newTestServer(t)
	am := srv.assessmentMgr

	now := time.Now()
	result := &models.AssessmentResult{
		SessionID:   "session-1",
		Score:       75,
		MaxScore:    100,
		Percentage:  75.0,
		ItemCount:   10,
		PassedCount: 7,
		Status:      "in_progress",
		LastChecked: now,
		TimeElapsed: "5m30s",
	}
	am.storeResult(context.Background(), "session-1", result)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/assessment/session-1/status", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["sessionId"] != "session-1" {
		t.Errorf("expected sessionId 'session-1', got '%v'", response["sessionId"])
	}
	if response["status"] != "in_progress" {
		t.Errorf("expected status 'in_progress', got '%v'", response["status"])
	}
	if response["score"].(float64) != 75 {
		t.Errorf("expected score 75, got %v", response["score"])
	}
	if response["maxScore"].(float64) != 100 {
		t.Errorf("expected maxScore 100, got %v", response["maxScore"])
	}
	if response["percentage"].(float64) != 75.0 {
		t.Errorf("expected percentage 75.0, got %v", response["percentage"])
	}
	if response["itemCount"].(float64) != 10 {
		t.Errorf("expected itemCount 10, got %v", response["itemCount"])
	}
	if response["passedCount"].(float64) != 7 {
		t.Errorf("expected passedCount 7, got %v", response["passedCount"])
	}
}

// -----------------------------------------------------------------------------
// Handler Tests - handleGetComponents (comprehensive)
// -----------------------------------------------------------------------------

func TestHandleGetComponentsMissing(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/assessment/nonexistent/components", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestHandleGetComponentsSuccess(t *testing.T) {
	srv := newTestServer(t)
	am := srv.assessmentMgr

	result := &models.AssessmentResult{
		SessionID: "session-1",
		Components: []models.ComponentResult{
			{
				ID:           "comp-1",
				Description:  "Network Configuration",
				TotalItems:   5,
				PassedItems:  4,
				MaxPoints:    50,
				EarnedPoints: 40,
				Percentage:   80.0,
			},
			{
				ID:           "comp-2",
				Description:  "Security Settings",
				TotalItems:   3,
				PassedItems:  2,
				MaxPoints:    30,
				EarnedPoints: 20,
				Percentage:   66.67,
			},
		},
	}
	am.storeResult(context.Background(), "session-1", result)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/assessment/session-1/components", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["sessionId"] != "session-1" {
		t.Errorf("expected sessionId 'session-1', got '%v'", response["sessionId"])
	}

	components, ok := response["components"].([]interface{})
	if !ok {
		t.Fatal("expected components array in response")
	}
	if len(components) != 2 {
		t.Errorf("expected 2 components, got %d", len(components))
	}
}

// -----------------------------------------------------------------------------
// Handler Tests - handleGetDeviceAssessment (comprehensive)
// -----------------------------------------------------------------------------

func TestHandleGetDeviceAssessmentMissing(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/assessment/nonexistent/devices/router1", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestHandleGetDeviceAssessmentDeviceNotFound(t *testing.T) {
	srv := newTestServer(t)
	am := srv.assessmentMgr

	result := &models.AssessmentResult{
		SessionID: "session-1",
		Devices: []models.DeviceResult{
			{Name: "router1", Type: "router"},
		},
	}
	am.storeResult(context.Background(), "session-1", result)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/assessment/session-1/devices/nonexistent", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response["error"] != "device not found" {
		t.Errorf("expected error 'device not found', got '%s'", response["error"])
	}
}

func TestHandleGetDeviceAssessmentSuccess(t *testing.T) {
	srv := newTestServer(t)
	am := srv.assessmentMgr

	result := &models.AssessmentResult{
		SessionID: "session-1",
		Devices: []models.DeviceResult{
			{
				Name:         "router1",
				Type:         "router",
				Status:       models.AssessmentStatusCorrect,
				TotalItems:   5,
				PassedItems:  4,
				EarnedPoints: 40,
				MaxPoints:    50,
				Checks: []models.CheckResult{
					{
						ID:          "check-1",
						Description: "Interface configured",
						Status:      models.AssessmentStatusCorrect,
						Points:      10,
					},
				},
			},
			{
				Name: "switch1",
				Type: "switch",
			},
		},
	}
	am.storeResult(context.Background(), "session-1", result)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/assessment/session-1/devices/router1", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response models.DeviceResult
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Name != "router1" {
		t.Errorf("expected device name 'router1', got '%s'", response.Name)
	}
	if response.Type != "router" {
		t.Errorf("expected device type 'router', got '%s'", response.Type)
	}
	if response.TotalItems != 5 {
		t.Errorf("expected totalItems 5, got %d", response.TotalItems)
	}
	if len(response.Checks) != 1 {
		t.Errorf("expected 1 check, got %d", len(response.Checks))
	}
}

// -----------------------------------------------------------------------------
// Handler Tests - handleRunAssessment (comprehensive)
// -----------------------------------------------------------------------------

func TestHandleRunAssessmentServiceUnavailable(t *testing.T) {
	cfg := DefaultConfig()
	wsHub := websocket.NewHub(newTestLogger())
	srv, _ := New(cfg, nil, nil, wsHub, newTestLogger())
	srv.assessmentMgr = nil

	req := httptest.NewRequest(http.MethodPost, "/api/v1/assessment/session-1/verify", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestHandleRunAssessmentSessionNotFound(t *testing.T) {
	eval := checkpoint.NewEvaluator(newTestLogger())
	srv := createTestServerForAssessment(t, nil, eval, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/assessment/nonexistent/verify", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

// -----------------------------------------------------------------------------
// AssessmentManagerConfig Tests
// -----------------------------------------------------------------------------

func TestAssessmentManagerConfigFields(t *testing.T) {
	repo := newTestAssessmentResultRepository()
	sessionRepo := mocks.NewFakeSessionRepository()
	labTemplateRepo := mocks.NewFakeLabTemplateRepository()

	am := NewAssessmentManager(AssessmentManagerConfig{
		Repo:            repo,
		SessionRepo:     sessionRepo,
		LabTemplateRepo: labTemplateRepo,
		TTL:             2 * time.Hour,
	})

	if am.repo == nil {
		t.Error("expected repo to be set")
	}
	if am.sessionRepo == nil {
		t.Error("expected session repo to be set")
	}
	if am.labTemplateRepo == nil {
		t.Error("expected lab template repo to be set")
	}
	if am.ttl != 2*time.Hour {
		t.Errorf("expected TTL 2h, got %v", am.ttl)
	}
}

// -----------------------------------------------------------------------------
// Assessment Manager Start/Stop Tests
// -----------------------------------------------------------------------------

func TestAssessmentManagerStartAndStop(t *testing.T) {
	srv := newTestServer(t)
	am := srv.assessmentMgr

	ctx, cancel := context.WithCancel(context.Background())

	// Start the manager
	am.Start(ctx)

	// Let it run briefly
	time.Sleep(10 * time.Millisecond)

	// Stop it via context cancellation
	cancel()

	// Stop should complete without hanging
	done := make(chan struct{})
	go func() {
		am.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("Stop() did not complete in time")
	}
}

// -----------------------------------------------------------------------------
// Default Constants Tests
// -----------------------------------------------------------------------------

func TestAssessmentDefaultConstants(t *testing.T) {
	if DefaultResultTTL != 1*time.Hour {
		t.Errorf("expected DefaultResultTTL 1h, got %v", DefaultResultTTL)
	}
	if ResultCleanupInterval != 5*time.Minute {
		t.Errorf("expected ResultCleanupInterval 5m, got %v", ResultCleanupInterval)
	}
}

// -----------------------------------------------------------------------------
// Concurrent Access Tests
// -----------------------------------------------------------------------------

func TestAssessmentManagerConcurrentAccess(t *testing.T) {
	srv := newTestServer(t)
	am := srv.assessmentMgr

	var wg sync.WaitGroup
	ctx := context.Background()

	// Concurrent writes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sessionID := "session-" + string(rune('A'+id%26))
			result := &models.AssessmentResult{
				SessionID: sessionID,
				Score:     id,
			}
			am.storeResult(ctx, sessionID, result)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sessionID := "session-" + string(rune('A'+id%26))
			am.getResult(ctx, sessionID)
		}(i)
	}

	// Concurrent count
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			am.ResultCount()
		}()
	}

	wg.Wait()

	// Should not panic or deadlock
	if am.ResultCount() == 0 {
		t.Error("expected some results to be stored")
	}
}

// -----------------------------------------------------------------------------
// TTL Refresh Tests
// -----------------------------------------------------------------------------

func TestAssessmentManagerTTLRefreshOnAccess(t *testing.T) {
	srv := newTestServer(t)
	am := srv.assessmentMgr
	am.ttl = 100 * time.Millisecond

	result := &models.AssessmentResult{SessionID: "session-1", Score: 50}
	am.storeResult(context.Background(), "session-1", result)

	// Wait for half TTL
	time.Sleep(60 * time.Millisecond)

	// Access should refresh TTL
	_, ok := am.getResult(context.Background(), "session-1")
	if !ok {
		t.Fatal("expected result to still be cached")
	}

	// Wait another 60ms (would have expired without refresh)
	time.Sleep(60 * time.Millisecond)

	// Should still be accessible due to TTL refresh
	_, ok = am.getResult(context.Background(), "session-1")
	if !ok {
		t.Error("expected result to still be cached after TTL refresh")
	}
}

// -----------------------------------------------------------------------------
// NewAssessmentManager Tests
// -----------------------------------------------------------------------------

func TestNewAssessmentManagerWithOptions(t *testing.T) {
	repo := newTestAssessmentResultRepository()
	sessionRepo := mocks.NewFakeSessionRepository()
	labTemplateRepo := mocks.NewFakeLabTemplateRepository()

	am := NewAssessmentManager(AssessmentManagerConfig{
		Repo:            repo,
		SessionRepo:     sessionRepo,
		LabTemplateRepo: labTemplateRepo,
		TTL:             30 * time.Minute,
	})

	if am.repo == nil {
		t.Error("expected assessment repo to be set")
	}
	if am.ttl != 30*time.Minute {
		t.Errorf("expected TTL 30m, got %v", am.ttl)
	}
	if am.sessionRepo == nil {
		t.Error("expected session repo to be set")
	}
	if am.labTemplateRepo == nil {
		t.Error("expected lab template repo to be set")
	}
	if am.verifier == nil {
		t.Error("expected verifier to be set")
	}
	if am.runner == nil {
		t.Error("expected runner to be set")
	}
	if am.logger == nil {
		t.Error("expected logger to be set")
	}
}

// -----------------------------------------------------------------------------
// Assessment Result with Devices and Components Tests
// -----------------------------------------------------------------------------

func TestHandleGetDeviceAssessmentMultipleDevices(t *testing.T) {
	srv := newTestServer(t)
	am := srv.assessmentMgr

	result := &models.AssessmentResult{
		SessionID: "session-1",
		Devices: []models.DeviceResult{
			{
				Name:         "router1",
				Type:         "router",
				Status:       models.AssessmentStatusCorrect,
				TotalItems:   5,
				PassedItems:  5,
				EarnedPoints: 50,
				MaxPoints:    50,
			},
			{
				Name:         "switch1",
				Type:         "switch",
				Status:       models.AssessmentStatusIncorrect,
				TotalItems:   3,
				PassedItems:  1,
				EarnedPoints: 10,
				MaxPoints:    30,
			},
			{
				Name:         "host1",
				Type:         "host",
				Status:       models.AssessmentStatusPending,
				TotalItems:   2,
				PassedItems:  0,
				EarnedPoints: 0,
				MaxPoints:    20,
			},
		},
	}
	am.storeResult(context.Background(), "session-1", result)

	// Test router1
	req := httptest.NewRequest(http.MethodGet, "/api/v1/assessment/session-1/devices/router1", nil)
	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d for router1, got %d", http.StatusOK, rr.Code)
	}

	// Test switch1
	req = httptest.NewRequest(http.MethodGet, "/api/v1/assessment/session-1/devices/switch1", nil)
	rr = httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d for switch1, got %d", http.StatusOK, rr.Code)
	}

	var response models.DeviceResult
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Name != "switch1" {
		t.Errorf("expected device name 'switch1', got '%s'", response.Name)
	}
	if response.Status != models.AssessmentStatusIncorrect {
		t.Errorf("expected status 'incorrect', got '%s'", response.Status)
	}

	// Test host1
	req = httptest.NewRequest(http.MethodGet, "/api/v1/assessment/session-1/devices/host1", nil)
	rr = httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d for host1, got %d", http.StatusOK, rr.Code)
	}
}

func TestHandleGetComponentsEmpty(t *testing.T) {
	srv := newTestServer(t)
	am := srv.assessmentMgr

	result := &models.AssessmentResult{
		SessionID:  "session-1",
		Components: []models.ComponentResult{},
	}
	am.storeResult(context.Background(), "session-1", result)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/assessment/session-1/components", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	components, ok := response["components"].([]interface{})
	if !ok {
		t.Fatal("expected components array in response")
	}
	if len(components) != 0 {
		t.Errorf("expected 0 components, got %d", len(components))
	}
}

func TestHandleGetDeviceAssessmentEmptyDevices(t *testing.T) {
	srv := newTestServer(t)
	am := srv.assessmentMgr

	result := &models.AssessmentResult{
		SessionID: "session-1",
		Devices:   []models.DeviceResult{},
	}
	am.storeResult(context.Background(), "session-1", result)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/assessment/session-1/devices/anydevice", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

// -----------------------------------------------------------------------------
// Cache Update Tests
// -----------------------------------------------------------------------------

func TestAssessmentManagerCacheUpdate(t *testing.T) {
	srv := newTestServer(t)
	am := srv.assessmentMgr
	ctx := context.Background()

	// Store initial result
	result1 := &models.AssessmentResult{
		SessionID: "session-1",
		Score:     50,
		Status:    "in_progress",
	}
	am.storeResult(ctx, "session-1", result1)

	// Update result
	result2 := &models.AssessmentResult{
		SessionID: "session-1",
		Score:     100,
		Status:    "completed",
	}
	am.storeResult(ctx, "session-1", result2)

	// Verify update
	retrieved, ok := am.getResult(ctx, "session-1")
	if !ok {
		t.Fatal("expected to retrieve result")
	}
	if retrieved.Score != 100 {
		t.Errorf("expected score 100, got %d", retrieved.Score)
	}
	if retrieved.Status != "completed" {
		t.Errorf("expected status 'completed', got '%s'", retrieved.Status)
	}

	// Count should still be 1 (update, not add)
	if am.ResultCount() != 1 {
		t.Errorf("expected 1 result after update, got %d", am.ResultCount())
	}
}
