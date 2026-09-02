package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/toddbartholow/kootenai/api/internal/canvas"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/events"
	"github.com/toddbartholow/kootenai/api/internal/models"
	natspkg "github.com/toddbartholow/kootenai/api/internal/nats"
)

// GradeSyncHandler processes grade update messages and syncs to Canvas
type GradeSyncHandler struct {
	gradeService  *canvas.GradeService
	gradeSyncRepo repositories.GradeSyncRepository
	sessionRepo   repositories.SessionRepository
	natsClient    *natspkg.Client
	consumer      *natspkg.Consumer
	logger        *slog.Logger
	canvasBaseURL string
}

// GradeSyncHandlerConfig configures the grade sync handler
type GradeSyncHandlerConfig struct {
	GradeService  *canvas.GradeService
	GradeSyncRepo repositories.GradeSyncRepository
	SessionRepo   repositories.SessionRepository
	NATSClient    *natspkg.Client
	CanvasBaseURL string
	Logger        *slog.Logger
}

// NewGradeSyncHandler creates a new grade sync handler
func NewGradeSyncHandler(cfg GradeSyncHandlerConfig) *GradeSyncHandler {
	return &GradeSyncHandler{
		gradeService:  cfg.GradeService,
		gradeSyncRepo: cfg.GradeSyncRepo,
		sessionRepo:   cfg.SessionRepo,
		natsClient:    cfg.NATSClient,
		logger:        cfg.Logger,
		canvasBaseURL: cfg.CanvasBaseURL,
	}
}

// Start starts the grade sync consumer
func (h *GradeSyncHandler) Start(ctx context.Context) error {
	if h.natsClient == nil {
		h.logger.Warn("NATS client not configured, grade sync disabled")
		return nil
	}

	consumer, err := natspkg.NewGradeSyncConsumer(h.natsClient, h.handleMessage, h.logger)
	if err != nil {
		return fmt.Errorf("creating grade sync consumer: %w", err)
	}

	h.consumer = consumer
	if err := h.consumer.Start(ctx); err != nil {
		return fmt.Errorf("starting grade sync consumer: %w", err)
	}

	h.logger.Info("Grade sync handler started")
	return nil
}

// Stop stops the grade sync consumer
func (h *GradeSyncHandler) Stop() {
	if h.consumer != nil {
		h.consumer.Stop()
		h.logger.Info("Grade sync handler stopped")
	}
}

// handleMessage processes a single grade update message
func (h *GradeSyncHandler) handleMessage(ctx context.Context, msg jetstream.Msg) error {
	var gradeUpdate events.GradeUpdate
	if err := json.Unmarshal(msg.Data(), &gradeUpdate); err != nil {
		h.logger.Error("Failed to unmarshal grade update", "error", err)
		return err // Will NAK and retry
	}

	h.logger.Info("Processing grade update",
		"sessionId", gradeUpdate.SessionID,
		"userId", gradeUpdate.UserID,
		"percentage", gradeUpdate.Percentage,
		"requiresSync", gradeUpdate.RequiresSync,
	)

	// Check if Canvas sync is required
	if !gradeUpdate.RequiresSync {
		h.logger.Debug("Grade update does not require Canvas sync", "sessionId", gradeUpdate.SessionID)
		return nil
	}

	// Verify we have Canvas details
	if gradeUpdate.CanvasCourseID == "" || gradeUpdate.CanvasAssignmentID == "" || gradeUpdate.CanvasUserID == "" {
		h.logger.Warn("Grade update missing Canvas details, skipping sync",
			"sessionId", gradeUpdate.SessionID,
			"courseId", gradeUpdate.CanvasCourseID,
			"assignmentId", gradeUpdate.CanvasAssignmentID,
			"userId", gradeUpdate.CanvasUserID,
		)
		return nil // Don't retry, just skip
	}

	// Create or update grade sync queue entry
	if h.gradeSyncRepo != nil {
		entry := &models.GradeSyncEntry{
			ID:                 uuid.New().String(),
			SessionID:          gradeUpdate.SessionID,
			Status:             "processing",
			EarnedPoints:       gradeUpdate.EarnedPoints,
			MaxPoints:          gradeUpdate.MaxPoints,
			Percentage:         gradeUpdate.Percentage,
			CanvasCourseID:     gradeUpdate.CanvasCourseID,
			CanvasAssignmentID: gradeUpdate.CanvasAssignmentID,
			CanvasUserID:       gradeUpdate.CanvasUserID,
			Attempts:           1,
			CreatedAt:          time.Now(),
		}
		now := time.Now()
		entry.LastAttemptAt = &now

		if err := h.gradeSyncRepo.Create(ctx, entry); err != nil {
			h.logger.Error("Failed to create grade sync entry", "error", err, "sessionId", gradeUpdate.SessionID)
			// Continue anyway to attempt sync
		}
	}

	// Attempt Canvas grade submission
	if h.gradeService == nil {
		h.logger.Warn("Grade service not configured, cannot sync to Canvas")
		return fmt.Errorf("grade service not configured")
	}

	// Build the line item URL from Canvas details
	lineItemURL := h.buildLineItemURL(gradeUpdate.CanvasCourseID, gradeUpdate.CanvasAssignmentID)

	// Generate comment with checkpoint summary
	comment := h.buildGradeComment(gradeUpdate)

	// Submit the grade
	err := h.gradeService.SubmitGrade(
		ctx,
		lineItemURL,
		gradeUpdate.CanvasUserID,
		gradeUpdate.Percentage,
		float64(gradeUpdate.MaxPoints),
		comment,
	)

	if err != nil {
		h.logger.Error("Failed to submit grade to Canvas",
			"error", err,
			"sessionId", gradeUpdate.SessionID,
			"lineItemURL", lineItemURL,
		)

		// Update sync entry with error
		if h.gradeSyncRepo != nil {
			_ = h.gradeSyncRepo.MarkFailed(ctx, gradeUpdate.SessionID, err.Error())
		}

		return err // Will NAK and retry
	}

	// Update sync entry as completed
	if h.gradeSyncRepo != nil {
		_ = h.gradeSyncRepo.MarkCompleted(ctx, gradeUpdate.SessionID)
	}

	// Update session with sync timestamp
	if h.sessionRepo != nil {
		now := time.Now()
		_ = h.sessionRepo.MarkGradeSynced(ctx, gradeUpdate.SessionID, now)
	}

	h.logger.Info("Grade synced to Canvas successfully",
		"sessionId", gradeUpdate.SessionID,
		"userId", gradeUpdate.CanvasUserID,
		"percentage", gradeUpdate.Percentage,
	)

	return nil
}

// buildLineItemURL constructs the Canvas AGS line item URL
func (h *GradeSyncHandler) buildLineItemURL(courseID, assignmentID string) string {
	// Canvas LTI AGS line item URL format
	// https://canvas.example.com/api/lti/courses/{course_id}/line_items/{line_item_id}
	if h.canvasBaseURL == "" {
		return ""
	}
	return fmt.Sprintf("%s/api/lti/courses/%s/line_items/%s", h.canvasBaseURL, courseID, assignmentID)
}

// buildGradeComment generates a comment summarizing the grade
func (h *GradeSyncHandler) buildGradeComment(update events.GradeUpdate) string {
	passedStr := "Not passed"
	if update.Passed {
		passedStr = "Passed"
	}

	comment := fmt.Sprintf("Lab Score: %d/%d (%.1f%%) - %s",
		update.EarnedPoints,
		update.MaxPoints,
		update.Percentage,
		passedStr,
	)

	// Add checkpoint summary if available
	if len(update.CheckpointStates) > 0 {
		passed := 0
		for _, cp := range update.CheckpointStates {
			if cp.Status == models.CheckpointStatusPassed {
				passed++
			}
		}
		comment += fmt.Sprintf(" | Checkpoints: %d/%d completed", passed, len(update.CheckpointStates))
	}

	return comment
}

// ProcessPendingGrades processes any pending grades in the sync queue
// This is useful for catching up after downtime or retrying failed syncs
func (h *GradeSyncHandler) ProcessPendingGrades(ctx context.Context) error {
	if h.gradeSyncRepo == nil {
		return nil
	}

	entries, err := h.gradeSyncRepo.GetPending(ctx, 100)
	if err != nil {
		return fmt.Errorf("getting pending grade sync entries: %w", err)
	}

	if len(entries) == 0 {
		return nil
	}

	h.logger.Info("Processing pending grade sync entries", "count", len(entries))

	for _, entry := range entries {
		// Mark as processing
		if err := h.gradeSyncRepo.MarkProcessing(ctx, entry.ID); err != nil {
			h.logger.Error("Failed to mark entry as processing", "error", err, "id", entry.ID)
			continue
		}

		// Build line item URL
		lineItemURL := h.buildLineItemURL(entry.CanvasCourseID, entry.CanvasAssignmentID)

		// Submit grade
		comment := fmt.Sprintf("Lab Score: %d/%d (%.1f%%)", entry.EarnedPoints, entry.MaxPoints, entry.Percentage)
		err := h.gradeService.SubmitGrade(
			ctx,
			lineItemURL,
			entry.CanvasUserID,
			entry.Percentage,
			float64(entry.MaxPoints),
			comment,
		)

		if err != nil {
			h.logger.Error("Failed to sync pending grade",
				"error", err,
				"sessionId", entry.SessionID,
			)
			_ = h.gradeSyncRepo.MarkFailed(ctx, entry.ID, err.Error())
			continue
		}

		_ = h.gradeSyncRepo.MarkCompleted(ctx, entry.ID)
		h.logger.Info("Synced pending grade", "sessionId", entry.SessionID)
	}

	return nil
}
