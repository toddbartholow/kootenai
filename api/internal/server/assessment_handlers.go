package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/toddbartholow/kootenai/api/internal/assessment"
	"github.com/toddbartholow/kootenai/api/internal/checkpoint"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/orchestrator"
	"github.com/toddbartholow/kootenai/api/internal/server/httputil"
	"github.com/toddbartholow/kootenai/api/internal/websocket"
)

const (
	// DefaultResultTTL is how long assessment results are kept before cleanup
	DefaultResultTTL = 1 * time.Hour
	// ResultCleanupInterval is how often to check for expired results
	ResultCleanupInterval = 5 * time.Minute
)

// cachedResult wraps an assessment result with expiration time
type cachedResult struct {
	result    *models.AssessmentResult
	expiresAt time.Time
}

// AssessmentManager manages assessment state for sessions.
type AssessmentManager struct {
	verifier        *assessment.Verifier
	runner          *assessment.AssessmentRunner
	repo            repositories.AssessmentResultRepository // Optional database persistence
	sessionRepo     repositories.SessionRepository
	labTemplateRepo repositories.LabTemplateRepository
	evaluator       *checkpoint.Evaluator
	orchestrator    orchestrator.Client
	wsHub           *websocket.Hub
	results         map[string]*cachedResult
	mu              sync.RWMutex
	ttl             time.Duration
	logger          *slog.Logger
	responder       *httputil.Responder
	stopCh          chan struct{}
	wg              sync.WaitGroup
}

// AssessmentManagerConfig configures an AssessmentManager.
type AssessmentManagerConfig struct {
	// Repo is the optional database persistence layer. When nil the manager
	// runs in memory-only mode.
	Repo            repositories.AssessmentResultRepository
	SessionRepo     repositories.SessionRepository
	LabTemplateRepo repositories.LabTemplateRepository
	// Evaluator resolves session → pod; required for RunAssessment.
	Evaluator *checkpoint.Evaluator
	// Orchestrator resolves pod → VM IP map; required for RunAssessment.
	Orchestrator orchestrator.Client
	// WsHub broadcasts assessment progress updates. Optional.
	WsHub     *websocket.Hub
	Logger    *slog.Logger
	Responder *httputil.Responder
	// TTL is how long results live in cache before cleanup. Zero means DefaultResultTTL.
	TTL time.Duration
}

// NewAssessmentManager creates a new AssessmentManager.
func NewAssessmentManager(cfg AssessmentManagerConfig) *AssessmentManager {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	responder := cfg.Responder
	if responder == nil {
		responder = httputil.NewResponder(logger)
	}
	ttl := cfg.TTL
	if ttl == 0 {
		ttl = DefaultResultTTL
	}
	verifier := assessment.NewVerifier(logger)
	return &AssessmentManager{
		verifier:        verifier,
		runner:          assessment.NewAssessmentRunner(verifier, logger),
		repo:            cfg.Repo,
		sessionRepo:     cfg.SessionRepo,
		labTemplateRepo: cfg.LabTemplateRepo,
		evaluator:       cfg.Evaluator,
		orchestrator:    cfg.Orchestrator,
		wsHub:           cfg.WsHub,
		results:         make(map[string]*cachedResult),
		ttl:             ttl,
		logger:          logger,
		responder:       responder,
		stopCh:          make(chan struct{}),
	}
}

// Start starts the background cleanup goroutine
func (am *AssessmentManager) Start(ctx context.Context) {
	am.wg.Add(1)
	go am.cleanupLoop(ctx)
	am.logger.Info("Assessment manager started", "resultTTL", am.ttl)
}

// Stop stops the background cleanup goroutine
func (am *AssessmentManager) Stop() {
	close(am.stopCh)
	am.wg.Wait()
	am.logger.Info("Assessment manager stopped")
}

// cleanupLoop periodically removes expired results
func (am *AssessmentManager) cleanupLoop(ctx context.Context) {
	defer am.wg.Done()
	ticker := time.NewTicker(ResultCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-am.stopCh:
			return
		case <-ticker.C:
			am.cleanupExpired(ctx)
		}
	}
}

// cleanupExpired removes results that have exceeded their TTL
func (am *AssessmentManager) cleanupExpired(ctx context.Context) {
	now := time.Now()
	am.mu.Lock()

	var expired []string
	for sessionID, cached := range am.results {
		if now.After(cached.expiresAt) {
			expired = append(expired, sessionID)
		}
	}

	for _, sessionID := range expired {
		delete(am.results, sessionID)
	}
	am.mu.Unlock()

	if len(expired) > 0 {
		am.logger.Debug("Cleaned up expired assessment results from cache", "count", len(expired))
	}

	// Also clean up database if repo is configured
	if am.repo != nil {
		cutoff := now.Add(-am.ttl)
		deleted, err := am.repo.DeleteExpired(ctx, cutoff)
		if err != nil {
			am.logger.Error("Failed to clean up expired assessment results from database", "error", err)
		} else if deleted > 0 {
			am.logger.Debug("Cleaned up expired assessment results from database", "count", deleted)
		}
	}
}

// storeResult stores a result with TTL (in-memory cache and optionally database)
func (am *AssessmentManager) storeResult(ctx context.Context, sessionID string, result *models.AssessmentResult) {
	// Store in memory cache
	am.mu.Lock()
	am.results[sessionID] = &cachedResult{
		result:    result,
		expiresAt: time.Now().Add(am.ttl),
	}
	am.mu.Unlock()

	// Persist to database if repo is configured
	if am.repo != nil {
		if err := am.repo.Create(ctx, result); err != nil {
			am.logger.Error("Failed to persist assessment result to database",
				"sessionID", sessionID,
				"error", err,
			)
		}
	}
}

// getResult retrieves a result (checks cache first, then database)
func (am *AssessmentManager) getResult(ctx context.Context, sessionID string) (*models.AssessmentResult, bool) {
	// Check cache first
	am.mu.Lock()
	cached, ok := am.results[sessionID]
	if ok {
		// Refresh TTL on access
		cached.expiresAt = time.Now().Add(am.ttl)
		am.mu.Unlock()
		return cached.result, true
	}
	am.mu.Unlock()

	// Check database if repo is configured
	if am.repo != nil {
		result, err := am.repo.GetBySessionID(ctx, sessionID)
		if err != nil {
			am.logger.Error("Failed to retrieve assessment result from database",
				"sessionID", sessionID,
				"error", err,
			)
			return nil, false
		}
		if result != nil {
			// Cache the result from database
			am.mu.Lock()
			am.results[sessionID] = &cachedResult{
				result:    result,
				expiresAt: time.Now().Add(am.ttl),
			}
			am.mu.Unlock()
			return result, true
		}
	}

	return nil, false
}

// ResultCount returns the number of cached results
func (am *AssessmentManager) ResultCount() int {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return len(am.results)
}

// SetupRoutes registers assessment routes on the router.
func (am *AssessmentManager) SetupRoutes(r chi.Router) {
	r.Route("/assessment", func(r chi.Router) {
		r.Get("/{sessionID}", am.handleGetAssessment)
		r.Post("/{sessionID}/verify", am.handleRunAssessment)
		r.Get("/{sessionID}/status", am.handleGetAssessmentStatus)
		r.Get("/{sessionID}/components", am.handleGetComponents)
		r.Get("/{sessionID}/devices/{deviceName}", am.handleGetDeviceAssessment)
	})
}

// handleGetAssessment returns the full assessment result for a session
func (am *AssessmentManager) handleGetAssessment(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")

	result, ok := am.getResult(r.Context(), sessionID)
	if !ok {
		am.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "assessment.errors.notFound", nil)
		return
	}

	am.responder.JSONResponse(w, http.StatusOK, result)
}

// handleRunAssessment triggers a new assessment run
func (am *AssessmentManager) handleRunAssessment(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")
	ctx := r.Context()

	if am.evaluator == nil || am.orchestrator == nil {
		am.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "assessment.errors.runtimeNotConfigured", nil)
		return
	}

	// Get the session to get pod info
	progress, err := am.evaluator.GetSessionProgress(sessionID)
	if err != nil {
		am.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "assessment.errors.sessionNotFound", nil)
		return
	}

	// Get pod to get VM IPs
	pod, err := am.orchestrator.GetPod(ctx, progress.PodID)
	if err != nil {
		am.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "assessment.errors.podNotFound", nil)
		return
	}

	// Build VM IP map
	vmIPs := make(map[string]string)
	for _, vm := range pod.VMs {
		if vm.IPAddress != "" {
			vmIPs[vm.Name] = vm.IPAddress
		}
	}

	// Parse request for optional assessment template override
	var req struct {
		Template *models.AssessmentTemplate `json:"template,omitempty"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	// Get assessment template from request or load from lab template
	var template models.AssessmentTemplate
	if req.Template != nil {
		template = *req.Template
	} else {
		loadedTemplate, err := am.loadAssessmentTemplate(ctx, sessionID)
		if err != nil {
			am.responder.SafeErrorResponse(w, err, "load assessment template")
			return
		}
		template = *loadedTemplate
	}

	// Create update channel for WebSocket broadcasts
	updateCh := make(chan models.AssessmentUpdate, 100)

	// Broadcast updates via WebSocket (with nil check to prevent panic)
	go func() {
		for update := range updateCh {
			update.SessionID = sessionID
			if am.wsHub != nil {
				am.wsHub.BroadcastAssessmentUpdate(&update)
			}
		}
	}()

	// Run assessment
	result, err := am.runner.RunAssessment(r.Context(), template, vmIPs, updateCh)
	close(updateCh)

	if err != nil {
		am.responder.SafeErrorResponse(w, err, "run assessment")
		return
	}

	result.SessionID = sessionID

	// Store result with TTL
	am.storeResult(r.Context(), sessionID, result)

	am.responder.JSONResponse(w, http.StatusOK, result)
}

// handleGetAssessmentStatus returns a summary of assessment status
func (am *AssessmentManager) handleGetAssessmentStatus(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")

	result, ok := am.getResult(r.Context(), sessionID)
	if !ok {
		am.responder.JSONResponse(w, http.StatusOK, map[string]any{
			"sessionId": sessionID,
			"status":    "not_started",
		})
		return
	}

	am.responder.JSONResponse(w, http.StatusOK, map[string]any{
		"sessionId":   sessionID,
		"status":      result.Status,
		"score":       result.Score,
		"maxScore":    result.MaxScore,
		"percentage":  result.Percentage,
		"itemCount":   result.ItemCount,
		"passedCount": result.PassedCount,
		"lastChecked": result.LastChecked,
		"timeElapsed": result.TimeElapsed,
	})
}

// handleGetComponents returns component-level results
func (am *AssessmentManager) handleGetComponents(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")

	result, ok := am.getResult(r.Context(), sessionID)
	if !ok {
		am.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "assessment.errors.notFound", nil)
		return
	}

	am.responder.JSONResponse(w, http.StatusOK, map[string]any{
		"sessionId":  sessionID,
		"components": result.Components,
	})
}

// loadAssessmentTemplate loads the assessment template from the lab template
// associated with the session.
func (am *AssessmentManager) loadAssessmentTemplate(ctx context.Context, sessionID string) (*models.AssessmentTemplate, error) {
	if am.sessionRepo == nil {
		return nil, fmt.Errorf("session repository not configured")
	}
	if am.labTemplateRepo == nil {
		return nil, fmt.Errorf("lab template repository not configured")
	}

	// Get the session to get the lab template ID
	session, err := am.sessionRepo.GetByID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("getting session: %w", err)
	}
	if session == nil {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Get the lab template record
	templateRecord, err := am.labTemplateRepo.GetByID(ctx, session.LabTemplateID)
	if err != nil {
		return nil, fmt.Errorf("getting lab template: %w", err)
	}
	if templateRecord == nil {
		// Try by name if ID didn't match
		templateRecord, err = am.labTemplateRepo.GetByName(ctx, session.LabTemplateID)
		if err != nil {
			return nil, fmt.Errorf("getting lab template by name: %w", err)
		}
	}
	if templateRecord == nil {
		return nil, fmt.Errorf("lab template not found: %s", session.LabTemplateID)
	}

	// Convert to LabTemplate to access the spec
	labTemplate, err := templateRecord.ToLabTemplate()
	if err != nil {
		return nil, fmt.Errorf("parsing lab template: %w", err)
	}

	// Check if the lab template has an assessment template defined
	if labTemplate.Spec.Assessment == nil {
		return nil, fmt.Errorf("lab template %q does not have an assessment template defined", templateRecord.Name)
	}

	return labTemplate.Spec.Assessment, nil
}

// handleGetDeviceAssessment returns assessment results for a specific device
func (am *AssessmentManager) handleGetDeviceAssessment(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")
	deviceName := chi.URLParam(r, "deviceName")

	result, ok := am.getResult(r.Context(), sessionID)
	if !ok {
		am.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "assessment.errors.notFound", nil)
		return
	}

	for _, device := range result.Devices {
		if device.Name == deviceName {
			am.responder.JSONResponse(w, http.StatusOK, device)
			return
		}
	}

	am.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "assessment.errors.deviceNotFound", nil)
}
