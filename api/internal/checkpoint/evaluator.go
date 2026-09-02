// Package checkpoint implements checkpoint evaluation logic
package checkpoint

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/events"
	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/regexutil"
)

// safePercentage computes earned/max*100 without dividing by zero.
func safePercentage(earned, max int) float64 {
	if max == 0 {
		return 0.0
	}
	return float64(earned) / float64(max) * 100
}

// CheckpointOutcomeRecorder is the subset of the metrics API needed to count
// checkpoint outcomes. Implemented by *metrics.Metrics; nil disables
// recording.
//
// Only terminal outcomes are counted: a checkpoint that passes verification,
// or one that fails it. The intermediate pending_verification state is not an
// outcome and is not recorded.
type CheckpointOutcomeRecorder interface {
	CheckpointPassed()
	CheckpointFailed()
}

// Evaluator evaluates events against checkpoint triggers
type Evaluator struct {
	// Templates maps lab template names to their checkpoint definitions
	templates map[string]*LabCheckpoints
	mu        sync.RWMutex

	// Sessions maps session IDs to their checkpoint states
	sessions  map[string]*SessionState
	sessionMu sync.RWMutex

	// Callback when checkpoint status changes
	onCheckpointUpdate func(ctx context.Context, update *events.CheckpointUpdate) error

	// Callback when a student appears stuck on a checkpoint
	onHintNudge OnHintNudgeFunc

	// Default stuck threshold (overridden per-template via AutoHintDelay)
	defaultStuckThreshold time.Duration

	// Active verifier for cross-checking passive event results
	activeVerifier *ActiveVerifier

	// Tampering detector for reporting verification mismatches
	tamperingDetector *TamperingDetector

	// Recorder notified when a checkpoint reaches a terminal outcome
	metrics CheckpointOutcomeRecorder

	logger *slog.Logger
}

// LabCheckpoints holds parsed checkpoint definitions for a lab template
type LabCheckpoints struct {
	TemplateName    string
	Config          *models.CheckpointConfig
	Checkpoints     map[string]*models.Checkpoint // Keyed by checkpoint ID
	MaxPoints       int
	CheckpointOrder []string // Ordered list of checkpoint IDs
}

// SessionState tracks checkpoint progress for a session
type SessionState struct {
	SessionID    string
	PodID        string
	UserID       string
	TemplateName string
	Checkpoints  map[string]*CheckpointProgress
	MaxPoints    int
	EarnedPoints int
	StartedAt    time.Time
}

// CheckpointProgress tracks progress on a single checkpoint
type CheckpointProgress struct {
	CheckpointID          string
	Status                models.CheckpointStatus
	Points                int
	EarnedPoints          int
	PassedAt              *time.Time
	TriggersMet           map[int]bool // Which triggers have been satisfied (by index)
	FirstAttemptAt        *time.Time   // When the first trigger evaluation attempt occurred
	AttemptCount          int          // Number of trigger evaluation attempts
	LastActivityAt        *time.Time   // Last time any trigger was evaluated for this checkpoint
	PendingVerificationAt *time.Time   // When active verification was requested
}

// HintNudge represents a nudge sent when a student appears stuck
type HintNudge struct {
	CheckpointID   string `json:"checkpointId"`
	CheckpointName string `json:"checkpointName"`
	MinutesStuck   int    `json:"minutesStuck"`
	AttemptCount   int    `json:"attemptCount"`
	NextHintLevel  int    `json:"nextHintLevel"`
}

// OnHintNudgeFunc is the callback type for hint nudge events
type OnHintNudgeFunc func(sessionID, podID string, nudge *HintNudge)

// EvaluatorOption configures an Evaluator at construction.
type EvaluatorOption func(*Evaluator)

// WithMetrics sets the recorder notified when a checkpoint passes or fails.
// Injected at construction rather than set afterwards: the active-check runner
// and the NATS checkpoint consumer read this field from their own goroutines,
// and both are started before the HTTP server is built.
func WithMetrics(rec CheckpointOutcomeRecorder) EvaluatorOption {
	return func(e *Evaluator) {
		e.metrics = rec
	}
}

// NewEvaluator creates a new checkpoint evaluator
func NewEvaluator(logger *slog.Logger, opts ...EvaluatorOption) *Evaluator {
	e := &Evaluator{
		templates:             make(map[string]*LabCheckpoints),
		sessions:              make(map[string]*SessionState),
		defaultStuckThreshold: 5 * time.Minute,
		logger:                logger,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// SetCheckpointUpdateCallback sets the callback for checkpoint updates
func (e *Evaluator) SetCheckpointUpdateCallback(fn func(ctx context.Context, update *events.CheckpointUpdate) error) {
	e.onCheckpointUpdate = fn
}

// SetHintNudgeCallback sets the callback for hint nudge events
func (e *Evaluator) SetHintNudgeCallback(fn OnHintNudgeFunc) {
	e.onHintNudge = fn
}

// SetActiveVerifier sets the active verifier for cross-checking passive results
func (e *Evaluator) SetActiveVerifier(av *ActiveVerifier) {
	e.activeVerifier = av
}

// SetTamperingDetector sets the tampering detector for reporting mismatches
func (e *Evaluator) SetTamperingDetector(td *TamperingDetector) {
	e.tamperingDetector = td
}

// StartStuckDetection runs a background goroutine that checks for stuck students every 60s.
// It sends hint nudges via the onHintNudge callback when a student hasn't progressed
// on a pending checkpoint beyond the threshold.
func (e *Evaluator) StartStuckDetection(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				e.checkStuckCheckpoints()
			}
		}
	}()
	e.logger.Info("Started stuck checkpoint detection")
}

// checkStuckCheckpoints iterates sessions and sends nudges for stuck checkpoints
func (e *Evaluator) checkStuckCheckpoints() {
	if e.onHintNudge == nil {
		return
	}

	now := time.Now()
	threshold := e.defaultStuckThreshold

	e.sessionMu.RLock()
	defer e.sessionMu.RUnlock()

	for _, session := range e.sessions {
		e.mu.RLock()
		tmpl := e.templates[session.TemplateName]
		e.mu.RUnlock()

		for cpID, progress := range session.Checkpoints {
			if progress.Status != models.CheckpointStatusPending {
				continue
			}

			// Use first activity or session start as baseline
			baseline := session.StartedAt
			if progress.LastActivityAt != nil {
				baseline = *progress.LastActivityAt
			} else if progress.FirstAttemptAt != nil {
				baseline = *progress.FirstAttemptAt
			}

			stuckDuration := now.Sub(baseline)
			if stuckDuration < threshold {
				continue
			}

			cpName := cpID
			if tmpl != nil {
				if cp, ok := tmpl.Checkpoints[cpID]; ok {
					cpName = cp.Description
				}
			}

			nudge := &HintNudge{
				CheckpointID:   cpID,
				CheckpointName: cpName,
				MinutesStuck:   int(stuckDuration.Minutes()),
				AttemptCount:   progress.AttemptCount,
				NextHintLevel:  1,
			}

			e.onHintNudge(session.SessionID, session.PodID, nudge)

			// Advance the threshold for this checkpoint (exponential backoff: double it)
			// by updating LastActivityAt to now, so next nudge is threshold from now
			progress.LastActivityAt = &now
		}
	}
}

// RegisterTemplate registers a lab template's checkpoints for evaluation
func (e *Evaluator) RegisterTemplate(template *models.LabTemplate) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(template.Spec.Objectives) == 0 {
		return nil // No checkpoints to register
	}

	lc := &LabCheckpoints{
		TemplateName:    template.Metadata.Name,
		Config:          template.Spec.Checkpoints,
		Checkpoints:     make(map[string]*models.Checkpoint),
		CheckpointOrder: make([]string, 0, len(template.Spec.Objectives)),
	}

	for i := range template.Spec.Objectives {
		cp := &template.Spec.Objectives[i]
		lc.Checkpoints[cp.ID] = cp
		lc.CheckpointOrder = append(lc.CheckpointOrder, cp.ID)
		lc.MaxPoints += cp.Points
	}

	e.templates[template.Metadata.Name] = lc
	e.logger.Info("Registered template checkpoints",
		"template", template.Metadata.Name,
		"checkpoints", len(lc.Checkpoints),
		"maxPoints", lc.MaxPoints,
	)

	return nil
}

// StartSession initializes checkpoint tracking for a new session
func (e *Evaluator) StartSession(sessionID, podID, userID, templateName string) error {
	e.mu.RLock()
	template, ok := e.templates[templateName]
	e.mu.RUnlock()

	if !ok {
		return fmt.Errorf("template %q not registered", templateName)
	}

	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()

	state := &SessionState{
		SessionID:    sessionID,
		PodID:        podID,
		UserID:       userID,
		TemplateName: templateName,
		Checkpoints:  make(map[string]*CheckpointProgress),
		MaxPoints:    template.MaxPoints,
		StartedAt:    time.Now(),
	}

	for id, cp := range template.Checkpoints {
		state.Checkpoints[id] = &CheckpointProgress{
			CheckpointID: id,
			Status:       models.CheckpointStatusPending,
			Points:       cp.Points,
			TriggersMet:  make(map[int]bool),
		}
	}

	e.sessions[sessionID] = state
	e.logger.Info("Started session checkpoint tracking",
		"sessionId", sessionID,
		"podId", podID,
		"template", templateName,
	)

	return nil
}

// EndSession removes session tracking
func (e *Evaluator) EndSession(sessionID string) {
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()
	delete(e.sessions, sessionID)
}

// GetTemplateCheckpoints returns all checkpoints for a template
func (e *Evaluator) GetTemplateCheckpoints(templateName string) []models.Checkpoint {
	e.mu.RLock()
	defer e.mu.RUnlock()

	template, ok := e.templates[templateName]
	if !ok {
		return nil
	}

	checkpoints := make([]models.Checkpoint, 0, len(template.Checkpoints))
	for _, cp := range template.Checkpoints {
		checkpoints = append(checkpoints, *cp)
	}
	return checkpoints
}

// EvaluateEvent evaluates an event against all checkpoints for the session.
// Mutations happen under sessionMu; callback dispatch happens after the lock
// is released so a slow DB/NATS publisher cannot serialise evaluation across
// every session.
func (e *Evaluator) EvaluateEvent(ctx context.Context, event *events.VMEvent, sessionID string) ([]string, error) {
	e.sessionMu.Lock()

	session, ok := e.sessions[sessionID]
	if !ok {
		e.sessionMu.Unlock()
		return nil, fmt.Errorf("session %q not found", sessionID)
	}

	e.mu.RLock()
	template, ok := e.templates[session.TemplateName]
	e.mu.RUnlock()

	if !ok {
		e.sessionMu.Unlock()
		return nil, fmt.Errorf("template %q not found", session.TemplateName)
	}

	var (
		passedCheckpoints []string
		pendingUpdates    []*events.CheckpointUpdate
		verifyLaunches    []func()
	)

	// Evaluate each pending checkpoint
	for cpID, progress := range session.Checkpoints {
		if progress.Status != models.CheckpointStatusPending {
			continue // Already passed or failed
		}

		checkpoint := template.Checkpoints[cpID]

		// Check dependencies
		if !e.dependenciesMet(session, checkpoint) {
			continue
		}

		// Track attempt activity for stuck detection
		now := time.Now()
		progress.AttemptCount++
		progress.LastActivityAt = &now
		if progress.FirstAttemptAt == nil {
			progress.FirstAttemptAt = &now
		}

		// Evaluate triggers
		matched, triggerIdx := e.evaluateTriggers(event, checkpoint)
		if !matched {
			continue
		}
		progress.TriggersMet[triggerIdx] = true

		if !e.checkpointComplete(progress, checkpoint) {
			continue
		}

		// If active verification is available and the trigger type is verifiable,
		// mark as pending_verification and queue async verification.
		if e.activeVerifier != nil && e.shouldRequireActiveVerification(checkpoint) {
			now := time.Now()
			progress.Status = models.CheckpointStatusPendingVerification
			progress.PendingVerificationAt = &now

			e.logger.Info("Checkpoint pending active verification",
				"sessionId", sessionID,
				"checkpointId", cpID,
				"points", progress.Points,
			)

			if e.onCheckpointUpdate != nil {
				pendingUpdates = append(pendingUpdates, &events.CheckpointUpdate{
					MessageHeader:       events.NewMessageHeader("checkpoint-evaluator"),
					PodID:               session.PodID,
					SessionID:           sessionID,
					CheckpointID:        cpID,
					Action:              "pending_verification",
					Status:              models.CheckpointStatusPendingVerification,
					Points:              progress.Points,
					TriggerEventID:      event.ID,
					SessionEarnedPoints: session.EarnedPoints,
					SessionMaxPoints:    session.MaxPoints,
				})
			}

			// Capture state for the goroutine to launch after we release the lock.
			capturedSessionID := session.SessionID
			capturedPodID := session.PodID
			capturedCP := checkpoint
			capturedCPID := cpID
			verifyLaunches = append(verifyLaunches, func() {
				go e.verifyAndConfirm(context.Background(), capturedSessionID, capturedPodID, capturedCPID, capturedCP)
			})
			continue
		}

		// No active verifier or trigger type not verifiable — pass immediately.
		passedAt := time.Now()
		if e.metrics != nil {
			e.metrics.CheckpointPassed()
		}
		progress.Status = models.CheckpointStatusPassed
		progress.PassedAt = &passedAt
		progress.EarnedPoints = progress.Points
		session.EarnedPoints += progress.Points

		passedCheckpoints = append(passedCheckpoints, cpID)

		e.logger.Info("Checkpoint passed",
			"sessionId", sessionID,
			"checkpointId", cpID,
			"points", progress.Points,
			"totalEarned", session.EarnedPoints,
		)

		if e.onCheckpointUpdate != nil {
			pendingUpdates = append(pendingUpdates, &events.CheckpointUpdate{
				MessageHeader:       events.NewMessageHeader("checkpoint-evaluator"),
				PodID:               session.PodID,
				SessionID:           sessionID,
				CheckpointID:        cpID,
				Action:              events.CheckpointActionPassed,
				Status:              models.CheckpointStatusPassed,
				Points:              progress.Points,
				EarnedPoints:        progress.EarnedPoints,
				TriggerEventID:      event.ID,
				SessionEarnedPoints: session.EarnedPoints,
				SessionMaxPoints:    session.MaxPoints,
				SessionPercentage:   safePercentage(session.EarnedPoints, session.MaxPoints),
			})
		}
	}

	e.sessionMu.Unlock()

	for _, update := range pendingUpdates {
		if err := e.onCheckpointUpdate(ctx, update); err != nil {
			e.logger.Error("Failed to publish checkpoint update", "error", err)
		}
	}
	for _, launch := range verifyLaunches {
		launch()
	}

	return passedCheckpoints, nil
}

// MarkCheckpointPassed marks a checkpoint as passed (used by active check runner)
func (e *Evaluator) MarkCheckpointPassed(ctx context.Context, sessionID, checkpointID string) error {
	e.sessionMu.Lock()

	session, ok := e.sessions[sessionID]
	if !ok {
		e.sessionMu.Unlock()
		return fmt.Errorf("session %q not found", sessionID)
	}

	progress, ok := session.Checkpoints[checkpointID]
	if !ok {
		e.sessionMu.Unlock()
		return fmt.Errorf("checkpoint %q not found in session", checkpointID)
	}

	// Already passed
	if progress.Status == models.CheckpointStatusPassed {
		e.sessionMu.Unlock()
		return nil
	}

	// Mark as passed
	now := time.Now()
	if e.metrics != nil {
		e.metrics.CheckpointPassed()
	}
	progress.Status = models.CheckpointStatusPassed
	progress.PassedAt = &now
	progress.EarnedPoints = progress.Points
	session.EarnedPoints += progress.Points

	e.logger.Info("Checkpoint marked as passed",
		"sessionId", sessionID,
		"checkpointId", checkpointID,
		"points", progress.Points,
		"totalEarned", session.EarnedPoints,
	)

	var pendingUpdate *events.CheckpointUpdate
	if e.onCheckpointUpdate != nil {
		pendingUpdate = &events.CheckpointUpdate{
			MessageHeader:       events.NewMessageHeader("active-check-runner"),
			PodID:               session.PodID,
			SessionID:           sessionID,
			CheckpointID:        checkpointID,
			Action:              events.CheckpointActionPassed,
			Status:              models.CheckpointStatusPassed,
			Points:              progress.Points,
			EarnedPoints:        progress.EarnedPoints,
			SessionEarnedPoints: session.EarnedPoints,
			SessionMaxPoints:    session.MaxPoints,
		}
	}
	e.sessionMu.Unlock()

	if pendingUpdate != nil {
		if err := e.onCheckpointUpdate(ctx, pendingUpdate); err != nil {
			e.logger.Error("Failed to publish checkpoint update", "error", err)
		}
	}

	return nil
}

// verifiableTriggerTypes are trigger types that can be actively verified via QEMU guest agent
var verifiableTriggerTypes = map[models.TriggerType]bool{
	models.TriggerTypeFileExists:  true,
	models.TriggerTypeFileContent: true,
	models.TriggerTypeService:     true,
	models.TriggerTypePackage:     true,
	models.TriggerTypeUserCreated: true,
	models.TriggerTypePermission:  true,
}

// shouldRequireActiveVerification returns true if the checkpoint has any
// actively verifiable triggers (file checks, service status, etc.)
func (e *Evaluator) shouldRequireActiveVerification(checkpoint *models.Checkpoint) bool {
	for _, trigger := range checkpoint.Triggers {
		if verifiableTriggerTypes[trigger.Type] {
			return true
		}
	}
	return false
}

// verifyAndConfirm performs async active verification after a passive event matched.
// On success, transitions checkpoint from pending_verification → passed.
// On failure, marks as verification_failed and reports to tampering detector.
func (e *Evaluator) verifyAndConfirm(ctx context.Context, sessionID, podID, checkpointID string, checkpoint *models.Checkpoint) {
	verifyCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	result, activePass, err := e.activeVerifier.CrossVerify(verifyCtx, podID, checkpoint, true)
	if err != nil {
		e.logger.Error("Active verification failed",
			"sessionId", sessionID,
			"checkpointId", checkpointID,
			"error", err,
		)
		// On error, leave as pending_verification for the ActiveCheckRunner to retry
		return
	}

	if activePass {
		e.MarkCheckpointVerified(ctx, sessionID, checkpointID)
	} else {
		e.MarkCheckpointVerificationFailed(ctx, sessionID, checkpointID)
		// Report mismatch to tampering detector
		if e.tamperingDetector != nil {
			e.tamperingDetector.RecordVerificationMismatch(sessionID, checkpointID, true, false)
		}
		e.logger.Warn("Active verification disagreed with passive event",
			"sessionId", sessionID,
			"checkpointId", checkpointID,
			"method", result.Method,
		)
	}
}

// MarkCheckpointVerified transitions a checkpoint from pending_verification → passed
func (e *Evaluator) MarkCheckpointVerified(ctx context.Context, sessionID, checkpointID string) {
	e.sessionMu.Lock()

	session, ok := e.sessions[sessionID]
	if !ok {
		e.sessionMu.Unlock()
		return
	}

	progress, ok := session.Checkpoints[checkpointID]
	if !ok {
		e.sessionMu.Unlock()
		return
	}

	if progress.Status != models.CheckpointStatusPendingVerification {
		e.sessionMu.Unlock()
		return // Already resolved
	}

	now := time.Now()
	if e.metrics != nil {
		e.metrics.CheckpointPassed()
	}
	progress.Status = models.CheckpointStatusPassed
	progress.PassedAt = &now
	progress.EarnedPoints = progress.Points
	session.EarnedPoints += progress.Points

	e.logger.Info("Checkpoint verified and passed",
		"sessionId", sessionID,
		"checkpointId", checkpointID,
		"points", progress.Points,
		"totalEarned", session.EarnedPoints,
	)

	var pendingUpdate *events.CheckpointUpdate
	if e.onCheckpointUpdate != nil {
		pendingUpdate = &events.CheckpointUpdate{
			MessageHeader:       events.NewMessageHeader("active-verifier"),
			PodID:               session.PodID,
			SessionID:           sessionID,
			CheckpointID:        checkpointID,
			Action:              events.CheckpointActionPassed,
			Status:              models.CheckpointStatusPassed,
			Points:              progress.Points,
			EarnedPoints:        progress.EarnedPoints,
			SessionEarnedPoints: session.EarnedPoints,
			SessionMaxPoints:    session.MaxPoints,
			SessionPercentage:   safePercentage(session.EarnedPoints, session.MaxPoints),
		}
	}
	e.sessionMu.Unlock()

	if pendingUpdate != nil {
		if err := e.onCheckpointUpdate(ctx, pendingUpdate); err != nil {
			e.logger.Error("Failed to publish verified checkpoint update", "error", err)
		}
	}
}

// MarkCheckpointVerificationFailed marks a checkpoint as verification_failed
func (e *Evaluator) MarkCheckpointVerificationFailed(ctx context.Context, sessionID, checkpointID string) {
	e.sessionMu.Lock()

	session, ok := e.sessions[sessionID]
	if !ok {
		e.sessionMu.Unlock()
		return
	}

	progress, ok := session.Checkpoints[checkpointID]
	if !ok {
		e.sessionMu.Unlock()
		return
	}

	if progress.Status != models.CheckpointStatusPendingVerification {
		e.sessionMu.Unlock()
		return
	}

	if e.metrics != nil {
		e.metrics.CheckpointFailed()
	}
	progress.Status = models.CheckpointStatusVerificationFailed

	e.logger.Warn("Checkpoint verification failed",
		"sessionId", sessionID,
		"checkpointId", checkpointID,
	)

	var pendingUpdate *events.CheckpointUpdate
	if e.onCheckpointUpdate != nil {
		pendingUpdate = &events.CheckpointUpdate{
			MessageHeader:       events.NewMessageHeader("active-verifier"),
			PodID:               session.PodID,
			SessionID:           sessionID,
			CheckpointID:        checkpointID,
			Action:              "verification_failed",
			Status:              models.CheckpointStatusVerificationFailed,
			Points:              progress.Points,
			SessionEarnedPoints: session.EarnedPoints,
			SessionMaxPoints:    session.MaxPoints,
		}
	}
	e.sessionMu.Unlock()

	if pendingUpdate != nil {
		if err := e.onCheckpointUpdate(ctx, pendingUpdate); err != nil {
			e.logger.Error("Failed to publish verification failed update", "error", err)
		}
	}
}

// dependenciesMet checks if all dependencies for a checkpoint are satisfied
func (e *Evaluator) dependenciesMet(session *SessionState, checkpoint *models.Checkpoint) bool {
	for _, depID := range checkpoint.DependsOn {
		depProgress, ok := session.Checkpoints[depID]
		if !ok || depProgress.Status != models.CheckpointStatusPassed {
			return false
		}
	}
	return true
}

// evaluateTriggers checks if the event matches any trigger
func (e *Evaluator) evaluateTriggers(event *events.VMEvent, checkpoint *models.Checkpoint) (matched bool, triggerIndex int) {
	for i, trigger := range checkpoint.Triggers {
		// Check if this trigger targets the right VM
		// Wazuh agent names may be formatted as {podID}-{vmName}, so check for suffix match
		if trigger.Target != event.VMName && !strings.HasSuffix(event.VMName, "-"+trigger.Target) {
			continue
		}

		// Check trigger type matches event type
		if !e.triggerTypeMatchesEvent(trigger.Type, event.EventType) {
			continue
		}

		// Evaluate the specific match criteria
		if e.evaluateMatch(&trigger, event) {
			return true, i
		}
	}
	return false, -1
}

// triggerTypeMatchesEvent maps trigger types to event types
func (e *Evaluator) triggerTypeMatchesEvent(triggerType models.TriggerType, eventType string) bool {
	mapping := map[models.TriggerType][]string{
		models.TriggerTypeFileExists:      {events.EventTypeSyscheck},
		models.TriggerTypeFileContent:     {events.EventTypeSyscheck},
		models.TriggerTypeFileDeleted:     {events.EventTypeSyscheck},
		models.TriggerTypePackage:         {events.EventTypePackage},
		models.TriggerTypeService:         {events.EventTypeService},
		models.TriggerTypeCommandExecuted: {events.EventTypeAudit},
		models.TriggerTypeUserCreated:     {events.EventTypeUser, events.EventTypeAuth},
		models.TriggerTypePermission:      {events.EventTypeSyscheck},
		models.TriggerTypeNetwork:         {events.EventTypeNetwork},
		// New system metric triggers
		models.TriggerTypeDiskUsage:      {events.EventTypeDiskUsage},
		models.TriggerTypeCPULoad:        {events.EventTypeCPULoad},
		models.TriggerTypeMemoryUsage:    {events.EventTypeMemoryUsage},
		models.TriggerTypeProcessRunning: {events.EventTypeProcess},
		models.TriggerTypePortListening:  {events.EventTypePortListen, events.EventTypeNetwork},
		models.TriggerTypeCronJob:        {events.EventTypeCron},
		models.TriggerTypeFirewallRule:   {events.EventTypeFirewall},
	}

	allowedTypes, ok := mapping[triggerType]
	if !ok {
		return false
	}

	for _, t := range allowedTypes {
		if t == eventType {
			return true
		}
	}
	return false
}

// evaluateMatch evaluates the specific match criteria for a trigger
func (e *Evaluator) evaluateMatch(trigger *models.CheckpointTrigger, event *events.VMEvent) bool {
	switch trigger.Type {
	case models.TriggerTypeFileExists, models.TriggerTypeFileContent, models.TriggerTypeFileDeleted:
		return e.evaluateFileMatch(trigger, event)
	case models.TriggerTypePackage:
		return e.evaluatePackageMatch(trigger, event)
	case models.TriggerTypeService:
		return e.evaluateServiceMatch(trigger, event)
	case models.TriggerTypeCommandExecuted:
		return e.evaluateCommandMatch(trigger, event)
	case models.TriggerTypeUserCreated:
		return e.evaluateUserMatch(trigger, event)
	case models.TriggerTypePermission:
		return e.evaluatePermissionMatch(trigger, event)
	case models.TriggerTypeNetwork:
		return e.evaluateNetworkMatch(trigger, event)
	case models.TriggerTypeDiskUsage:
		return e.evaluateDiskUsageMatch(trigger, event)
	case models.TriggerTypeCPULoad:
		return e.evaluateCPULoadMatch(trigger, event)
	case models.TriggerTypeMemoryUsage:
		return e.evaluateMemoryUsageMatch(trigger, event)
	case models.TriggerTypeProcessRunning:
		return e.evaluateProcessMatch(trigger, event)
	case models.TriggerTypePortListening:
		return e.evaluatePortListenMatch(trigger, event)
	case models.TriggerTypeCronJob:
		return e.evaluateCronJobMatch(trigger, event)
	case models.TriggerTypeFirewallRule:
		return e.evaluateFirewallMatch(trigger, event)
	default:
		return false
	}
}

// evaluateFileMatch evaluates file-based triggers
func (e *Evaluator) evaluateFileMatch(trigger *models.CheckpointTrigger, event *events.VMEvent) bool {
	var syscheck models.SyscheckData
	if err := json.Unmarshal(event.Data, &syscheck); err != nil {
		return false
	}

	// Check path matches
	if trigger.Match.Path != "" && syscheck.Path != trigger.Match.Path {
		// Try glob/prefix match (explicit wildcard)
		switch {
		case strings.HasSuffix(trigger.Match.Path, "*"):
			if !strings.HasPrefix(syscheck.Path, strings.TrimSuffix(trigger.Match.Path, "*")) {
				return false
			}
		case trigger.Type == models.TriggerTypeFileExists:
			// For file_exists, also match if syscheck path is inside the trigger path
			// This handles directory existence checks - if /home/user/projects/file.txt exists,
			// then /home/user/projects also exists
			if !strings.HasPrefix(syscheck.Path, trigger.Match.Path+"/") && syscheck.Path != trigger.Match.Path {
				return false
			}
		default:
			return false
		}
	}

	// For file_exists, just check the file was added or modified
	if trigger.Type == models.TriggerTypeFileExists {
		return syscheck.Event == "added" || syscheck.Event == "modified"
	}

	// For file_deleted
	if trigger.Type == models.TriggerTypeFileDeleted {
		return syscheck.Event == "deleted"
	}

	// For file_content, check the content matches
	if trigger.Type == models.TriggerTypeFileContent {
		// Check contains
		if trigger.Match.Contains != "" {
			if syscheck.NewContent != "" && strings.Contains(syscheck.NewContent, trigger.Match.Contains) {
				return true
			}
			if syscheck.Diff != "" && strings.Contains(syscheck.Diff, trigger.Match.Contains) {
				return true
			}
		}

		// Check regex
		if trigger.Match.Regex != "" {
			re, err := regexutil.CachedCompile(trigger.Match.Regex)
			if err != nil {
				e.logger.Error("Invalid regex in trigger", "regex", trigger.Match.Regex, "error", err)
				return false
			}
			if syscheck.NewContent != "" && re.MatchString(syscheck.NewContent) {
				return true
			}
			if syscheck.Diff != "" && re.MatchString(syscheck.Diff) {
				return true
			}
		}
	}

	return false
}

// evaluatePackageMatch evaluates package-based triggers
func (e *Evaluator) evaluatePackageMatch(trigger *models.CheckpointTrigger, event *events.VMEvent) bool {
	var pkg models.PackageData
	if err := json.Unmarshal(event.Data, &pkg); err != nil {
		return false
	}

	// Check package name
	if trigger.Match.Package != "" && pkg.Package != trigger.Match.Package {
		return false
	}

	// Check state
	if trigger.Match.State != "" {
		switch trigger.Match.State {
		case "installed":
			return pkg.Action == "install" || pkg.Action == "upgrade"
		case "removed":
			return pkg.Action == "remove"
		}
	}

	return true
}

// evaluateServiceMatch evaluates service-based triggers
func (e *Evaluator) evaluateServiceMatch(trigger *models.CheckpointTrigger, event *events.VMEvent) bool {
	var svc models.ServiceData
	if err := json.Unmarshal(event.Data, &svc); err != nil {
		return false
	}

	// Check service name
	if trigger.Match.Name != "" && svc.Unit != trigger.Match.Name {
		// Try without .service suffix
		if svc.Unit != trigger.Match.Name+".service" {
			return false
		}
	}

	// Check state
	if trigger.Match.State != "" && svc.State != trigger.Match.State {
		return false
	}

	return true
}

// evaluateCommandMatch evaluates command execution triggers
func (e *Evaluator) evaluateCommandMatch(trigger *models.CheckpointTrigger, event *events.VMEvent) bool {
	var audit models.AuditData
	if err := json.Unmarshal(event.Data, &audit); err != nil {
		e.logger.Debug("Failed to unmarshal audit data", "error", err)
		return false
	}

	// Check command pattern
	if trigger.Match.Pattern != "" {
		re, err := regexutil.CachedCompile(trigger.Match.Pattern)
		if err != nil {
			e.logger.Error("Invalid regex in command trigger", "pattern", trigger.Match.Pattern, "error", err)
			return false
		}

		// Parse command to extract executable and arguments
		// Command can be "/usr/bin/df -h" or "df -h" or just "pwd"
		cmdParts := strings.Fields(audit.Command)
		exeParts := strings.Fields(audit.Exe)

		// Get just the executable names (without path and args)
		var cmdExeName, exeExeName string
		if len(cmdParts) > 0 {
			cmdExeName = filepath.Base(cmdParts[0])
		}
		if len(exeParts) > 0 {
			exeExeName = filepath.Base(exeParts[0])
		}

		// Try matching against:
		// 1. Full command string (e.g., "/usr/bin/df -h")
		// 2. Full exe string
		// 3. Just the executable name from command (e.g., "df")
		// 4. Just the executable name from exe
		// 5. Command with args but no path (e.g., "df -h")
		cmdWithoutPath := ""
		if len(cmdParts) > 0 {
			cmdParts[0] = filepath.Base(cmdParts[0])
			cmdWithoutPath = strings.Join(cmdParts, " ")
		}

		matched := re.MatchString(audit.Command) ||
			re.MatchString(audit.Exe) ||
			re.MatchString(cmdExeName) ||
			re.MatchString(exeExeName) ||
			re.MatchString(cmdWithoutPath)

		e.logger.Debug("Command match evaluation",
			"pattern", trigger.Match.Pattern,
			"command", audit.Command,
			"exe", audit.Exe,
			"cmdExeName", cmdExeName,
			"exeExeName", exeExeName,
			"cmdWithoutPath", cmdWithoutPath,
			"matched", matched,
		)

		if !matched {
			return false
		}
	}

	// Check user if specified
	if trigger.Match.User != "" && audit.User != trigger.Match.User {
		return false
	}

	return true
}

// evaluateUserMatch evaluates user creation triggers
func (e *Evaluator) evaluateUserMatch(trigger *models.CheckpointTrigger, event *events.VMEvent) bool {
	var user models.UserData
	if err := json.Unmarshal(event.Data, &user); err != nil {
		return false
	}

	// Check username
	if trigger.Match.Username != "" && user.Username != trigger.Match.Username {
		return false
	}

	// Check action
	if user.Action != "created" {
		return false
	}

	return true
}

// evaluatePermissionMatch evaluates file permission change triggers
func (e *Evaluator) evaluatePermissionMatch(trigger *models.CheckpointTrigger, event *events.VMEvent) bool {
	var syscheck models.SyscheckData
	if err := json.Unmarshal(event.Data, &syscheck); err != nil {
		return false
	}

	// Check path matches
	if trigger.Match.Path != "" && syscheck.Path != trigger.Match.Path {
		// Try glob/prefix match
		if !strings.HasPrefix(syscheck.Path, strings.TrimSuffix(trigger.Match.Path, "*")) {
			return false
		}
	}

	// Check if this is a permission change event
	if syscheck.Event != "modified" && syscheck.Event != "attributes" {
		return false
	}

	// Check expected permissions if specified
	if trigger.Match.Permission != "" {
		// Wazuh reports permissions in different formats
		// Check against new_perm field, or fall back to permissions field
		actualPerm := syscheck.NewPerm
		if actualPerm == "" {
			actualPerm = syscheck.Permissions
		}
		// Compare canonically rather than literally. Wazuh emits symbolic
		// ("rwxr-xr-x") on some versions and octal on others, and template
		// authors write "755", "0755" or the symbolic form interchangeably --
		// the models.TriggerMatch doc comment suggests "0644" while every
		// evaluator test uses "644". A literal comparison silently never
		// matches whenever those spellings differ, which costs the student the
		// points with no signal anywhere.
		if normalizePermission(actualPerm) != normalizePermission(trigger.Match.Permission) {
			return false
		}
	}

	// Check owner if specified
	if trigger.Match.Owner != "" {
		ownerMatch := false
		// Check against NewUser field first, then Owner
		if syscheck.NewUser != "" {
			ownerMatch = syscheck.NewUser == trigger.Match.Owner
		} else if syscheck.Owner != "" {
			ownerMatch = syscheck.Owner == trigger.Match.Owner
		}
		if !ownerMatch {
			return false
		}
	}

	// Check group if specified
	if trigger.Match.Group != "" {
		groupMatch := false
		if syscheck.NewGroup != "" {
			groupMatch = syscheck.NewGroup == trigger.Match.Group
		} else if syscheck.Group != "" {
			groupMatch = syscheck.Group == trigger.Match.Group
		}
		if !groupMatch {
			return false
		}
	}

	return true
}

// evaluateNetworkMatch evaluates network connection triggers
func (e *Evaluator) evaluateNetworkMatch(trigger *models.CheckpointTrigger, event *events.VMEvent) bool {
	var network models.NetworkData
	if err := json.Unmarshal(event.Data, &network); err != nil {
		return false
	}

	// Check protocol if specified (case-insensitive)
	if trigger.Match.Protocol != "" {
		if !strings.EqualFold(network.Protocol, trigger.Match.Protocol) {
			return false
		}
	}

	// Check port if specified
	if trigger.Match.Port != 0 {
		if network.LocalPort != trigger.Match.Port && network.RemotePort != trigger.Match.Port {
			return false
		}
	}

	// Check state if specified (e.g., "ESTABLISHED", "LISTEN")
	if trigger.Match.State != "" {
		if !strings.EqualFold(network.State, trigger.Match.State) {
			return false
		}
	}

	// Check remote address if specified (support both Destination and Address fields)
	targetAddr := trigger.Match.Destination
	if targetAddr == "" {
		targetAddr = trigger.Match.Address
	}
	if targetAddr != "" {
		if network.RemoteAddr != targetAddr {
			// Try prefix/CIDR match for partial IP matching
			if !strings.HasPrefix(network.RemoteAddr, strings.TrimSuffix(targetAddr, "*")) {
				return false
			}
		}
	}

	return true
}

// checkpointComplete checks if a checkpoint is complete
// For now, any single trigger match = complete
func (e *Evaluator) checkpointComplete(progress *CheckpointProgress, checkpoint *models.Checkpoint) bool {
	// At least one trigger must be met
	return len(progress.TriggersMet) > 0
}

// GetSessionProgress returns the current progress for a session
func (e *Evaluator) GetSessionProgress(sessionID string) (*SessionState, error) {
	e.sessionMu.RLock()
	defer e.sessionMu.RUnlock()

	session, ok := e.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session %q not found", sessionID)
	}

	return session, nil
}

// GetCheckpointStatus returns the status of a specific checkpoint
func (e *Evaluator) GetCheckpointStatus(sessionID, checkpointID string) (*CheckpointProgress, error) {
	e.sessionMu.RLock()
	defer e.sessionMu.RUnlock()

	session, ok := e.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session %q not found", sessionID)
	}

	progress, ok := session.Checkpoints[checkpointID]
	if !ok {
		return nil, fmt.Errorf("checkpoint %q not found", checkpointID)
	}

	return progress, nil
}

// CheckpointWithDescription combines progress with the checkpoint definition
type CheckpointWithDescription struct {
	CheckpointID string
	Description  string
	Hint         string
	Status       models.CheckpointStatus
	Points       int
	EarnedPoints int
	PassedAt     *time.Time
}

// GetSessionProgressWithDescriptions returns checkpoint progress with descriptions from the template
func (e *Evaluator) GetSessionProgressWithDescriptions(sessionID string) ([]CheckpointWithDescription, error) {
	e.sessionMu.RLock()
	session, ok := e.sessions[sessionID]
	e.sessionMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("session %q not found", sessionID)
	}

	e.mu.RLock()
	template, ok := e.templates[session.TemplateName]
	e.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("template %q not found", session.TemplateName)
	}

	result := make([]CheckpointWithDescription, 0, len(session.Checkpoints))

	// Use the checkpoint order from template if available
	checkpointOrder := template.CheckpointOrder
	if len(checkpointOrder) == 0 {
		// Fallback to iterating the map
		for id := range session.Checkpoints {
			checkpointOrder = append(checkpointOrder, id)
		}
	}

	for _, cpID := range checkpointOrder {
		progress, ok := session.Checkpoints[cpID]
		if !ok {
			continue
		}

		cp := template.Checkpoints[cpID]
		description := cpID // fallback to ID
		hint := ""
		if cp != nil {
			description = cp.Description
			hint = cp.Hint
		}

		result = append(result, CheckpointWithDescription{
			CheckpointID: cpID,
			Description:  description,
			Hint:         hint,
			Status:       progress.Status,
			Points:       progress.Points,
			EarnedPoints: progress.EarnedPoints,
			PassedAt:     progress.PassedAt,
		})
	}

	return result, nil
}

// =============================================================================
// System Metric Trigger Evaluators
// =============================================================================

// evaluateDiskUsageMatch evaluates disk usage triggers
func (e *Evaluator) evaluateDiskUsageMatch(trigger *models.CheckpointTrigger, event *events.VMEvent) bool {
	var disk models.DiskUsageData
	if err := json.Unmarshal(event.Data, &disk); err != nil {
		return false
	}

	// Check mount point if specified
	if trigger.Match.MountPoint != "" && disk.MountPoint != trigger.Match.MountPoint {
		return false
	}

	// Check threshold
	return e.checkThreshold(disk.UsedPercent, trigger.Match.ThresholdPct, trigger.Match.Operator)
}

// evaluateCPULoadMatch evaluates CPU load triggers
func (e *Evaluator) evaluateCPULoadMatch(trigger *models.CheckpointTrigger, event *events.VMEvent) bool {
	var cpu models.CPULoadData
	if err := json.Unmarshal(event.Data, &cpu); err != nil {
		return false
	}

	// Use CPU percentage if threshold_pct specified, otherwise use load1
	value := cpu.CPUPercent
	threshold := trigger.Match.ThresholdPct
	if threshold == 0 {
		value = cpu.Load1
		threshold = trigger.Match.ThresholdValue
	}

	return e.checkThreshold(value, threshold, trigger.Match.Operator)
}

// evaluateMemoryUsageMatch evaluates memory usage triggers
func (e *Evaluator) evaluateMemoryUsageMatch(trigger *models.CheckpointTrigger, event *events.VMEvent) bool {
	var mem models.MemoryUsageData
	if err := json.Unmarshal(event.Data, &mem); err != nil {
		return false
	}

	return e.checkThreshold(mem.UsedPercent, trigger.Match.ThresholdPct, trigger.Match.Operator)
}

// evaluateProcessMatch evaluates process running triggers
func (e *Evaluator) evaluateProcessMatch(trigger *models.CheckpointTrigger, event *events.VMEvent) bool {
	var proc models.ProcessData
	if err := json.Unmarshal(event.Data, &proc); err != nil {
		return false
	}

	// Check process name
	if trigger.Match.ProcessName != "" {
		if !strings.Contains(strings.ToLower(proc.Name), strings.ToLower(trigger.Match.ProcessName)) &&
			!strings.Contains(strings.ToLower(proc.CommandLine), strings.ToLower(trigger.Match.ProcessName)) {
			return false
		}
	}

	// Check if process is running
	return proc.State == "running" || proc.State == "sleeping"
}

// evaluatePortListenMatch evaluates port listening triggers
func (e *Evaluator) evaluatePortListenMatch(trigger *models.CheckpointTrigger, event *events.VMEvent) bool {
	var port models.PortListenData
	if err := json.Unmarshal(event.Data, &port); err != nil {
		return false
	}

	// Check port number
	if trigger.Match.Port > 0 && port.Port != trigger.Match.Port {
		return false
	}

	// Check protocol
	if trigger.Match.Protocol != "" && !strings.EqualFold(port.Protocol, trigger.Match.Protocol) {
		return false
	}

	// Check listen address
	if trigger.Match.ListenAddress != "" && port.Address != trigger.Match.ListenAddress {
		// Allow 0.0.0.0 to match any address
		if trigger.Match.ListenAddress != "0.0.0.0" {
			return false
		}
	}

	// Check state - allow various listening state representations
	if trigger.Match.State != "" {
		// Check if the state matches (case insensitive)
		if !strings.EqualFold(port.State, trigger.Match.State) {
			return false
		}
	}

	// If no state specified in trigger, check if actually listening
	if trigger.Match.State == "" {
		state := strings.ToLower(port.State)
		if state != "listen" && state != "listening" {
			return false
		}
	}

	return true
}

// evaluateCronJobMatch evaluates cron job triggers
func (e *Evaluator) evaluateCronJobMatch(trigger *models.CheckpointTrigger, event *events.VMEvent) bool {
	var cron models.CronJobData
	if err := json.Unmarshal(event.Data, &cron); err != nil {
		return false
	}

	// Check schedule pattern
	if trigger.Match.Schedule != "" && cron.Schedule != trigger.Match.Schedule {
		return false
	}

	// Check command pattern
	if trigger.Match.Command != "" {
		re, err := regexutil.CachedCompile(trigger.Match.Command)
		if err != nil {
			e.logger.Warn("Invalid cron command regex", "pattern", trigger.Match.Command, "error", err)
			return false
		}
		if !re.MatchString(cron.Command) {
			return false
		}
	}

	// Check user
	if trigger.Match.User != "" && cron.User != trigger.Match.User {
		return false
	}

	// Check pattern (for matching command or schedule)
	if trigger.Match.Pattern != "" {
		re, err := regexutil.CachedCompile(trigger.Match.Pattern)
		if err != nil {
			e.logger.Warn("Invalid cron pattern regex", "pattern", trigger.Match.Pattern, "error", err)
			return false
		}
		matched := re.MatchString(cron.Command)
		if !matched {
			// Try matching against schedule
			matched = re.MatchString(cron.Schedule)
		}
		if !matched {
			return false
		}
	}

	// If status is specified in trigger, check it; otherwise accept any active/created status
	if cron.Status == "" {
		return true // No status to check
	}
	status := strings.ToLower(cron.Status)
	return status == "active" || status == "created" || status == "enabled"
}

// evaluateFirewallMatch evaluates firewall rule triggers
func (e *Evaluator) evaluateFirewallMatch(trigger *models.CheckpointTrigger, event *events.VMEvent) bool {
	var rule models.FirewallRuleData
	if err := json.Unmarshal(event.Data, &rule); err != nil {
		return false
	}

	// Check chain
	if trigger.Match.Chain != "" && !strings.EqualFold(rule.Chain, trigger.Match.Chain) {
		return false
	}

	// Check action - map allow/deny to iptables actions
	if trigger.Match.Action != "" {
		triggerAction := strings.ToLower(trigger.Match.Action)
		ruleAction := strings.ToLower(rule.Action)

		// Map friendly names to iptables actions
		actionMatches := false
		switch triggerAction {
		case "allow", "accept":
			actionMatches = ruleAction == "accept"
		case "deny", "drop":
			actionMatches = ruleAction == "drop"
		case "reject":
			actionMatches = ruleAction == "reject"
		default:
			actionMatches = triggerAction == ruleAction
		}

		if !actionMatches {
			return false
		}
	}

	// Check protocol
	if trigger.Match.Protocol != "" && !strings.EqualFold(rule.Protocol, trigger.Match.Protocol) {
		return false
	}

	// Check port (trigger.Match.Port maps to rule.DestPort)
	if trigger.Match.Port > 0 && rule.DestPort != trigger.Match.Port {
		return false
	}

	// Check source/destination (trigger.Match.Destination can match rule.SourceIP or DestIP)
	if trigger.Match.Destination != "" {
		if rule.SourceIP != trigger.Match.Destination && rule.DestIP != trigger.Match.Destination {
			return false
		}
	}

	// Check interface
	if trigger.Match.Interface != "" && rule.Interface != trigger.Match.Interface {
		return false
	}

	return true
}

// checkThreshold compares a value against a threshold using the specified operator
func (e *Evaluator) checkThreshold(value, threshold float64, operator string) bool {
	op := strings.ToLower(operator)
	switch op {
	case "lt", "<":
		return value < threshold
	case "le", "<=":
		return value <= threshold
	case "eq", "=", "==":
		return value == threshold
	case "ne", "!=", "<>":
		return value != threshold
	case "ge", ">=":
		return value >= threshold
	case "gt", ">":
		return value > threshold
	case "":
		// Default to >= when no operator specified (e.g., disk usage >= 80%)
		return value >= threshold
	default:
		// Unknown operator - return false to be safe
		return false
	}
}

// normalizePermission reduces a file-permission string to four-digit octal so
// the spellings in use can be compared to each other.
//
// It accepts:
//   - symbolic, 9 or 10 characters: "rwxr-xr-x", "-rwsr-xr-x"
//   - octal with a mode prefix:     "100755", "104755"
//   - octal with or without a
//     leading zero:                 "0755", "755", "4755"
//
// The result keeps the special-bits digit, so a setuid, setgid or sticky file
// does NOT compare equal to the same file without those bits. That matters
// here: SUID escalation and the /tmp sticky bit are exactly what a security
// course grades, and collapsing them would credit a student for the wrong file
// mode. "755" and "4755" are different permissions and normalize differently.
//
// Anything it does not recognise is returned unchanged, so an unexpected format
// falls back to a literal comparison rather than matching something it should
// not.
func normalizePermission(perm string) string {
	perm = strings.TrimSpace(perm)
	if perm == "" {
		return ""
	}

	// Symbolic. A leading file-type character (d, -, l...) is optional, so try
	// the trailing nine characters. Each position is validated against the set
	// legal for it -- a looser check misreads arbitrary text that happens to
	// contain a dash.
	if len(perm) == 9 || len(perm) == 10 {
		if octal, ok := symbolicToOctal(perm[len(perm)-9:]); ok {
			return octal
		}
	}

	// Octal. Keep the low four digits, which drops the file-type bits in a
	// stat-style mode ("100755" -> "0755", "104755" -> "4755") and pads a bare
	// three-digit mode so every form lands on the same width.
	for _, c := range perm {
		if c < '0' || c > '7' {
			return perm
		}
	}
	if len(perm) > 4 {
		perm = perm[len(perm)-4:]
	}
	for len(perm) < 4 {
		perm = "0" + perm
	}
	return perm
}

// symbolicToOctal converts a nine-character symbolic permission such as
// "rwxr-xr-x" to four-digit octal ("0755"). It reports false unless every
// position holds a character legal for it, so non-permission text is left
// alone.
//
// The execute slot of each triad doubles as the special-bit slot: setuid for
// user, setgid for group, sticky for other. Lowercase (s, t) means the special
// bit is set AND execute is set; uppercase (S, T) means the special bit is set
// and execute is NOT. Treating uppercase as execute-set -- which an earlier
// version of this did -- reports mode 4644 as 744, wrong in both directions.
func symbolicToOctal(sym string) (string, bool) {
	// Which special character is legal in each triad's execute slot.
	specialChars := [3]string{"sS", "sS", "tT"}

	var perm [3]byte
	special := 0
	for group := 0; group < 3; group++ {
		bits := 0

		switch c := sym[group*3]; c {
		case 'r':
			bits |= 4
		case '-':
		default:
			return "", false
		}

		switch c := sym[group*3+1]; c {
		case 'w':
			bits |= 2
		case '-':
		default:
			return "", false
		}

		switch c := sym[group*3+2]; {
		case c == 'x':
			bits |= 1
		case c == '-':
		case strings.IndexByte(specialChars[group], c) >= 0:
			special |= 4 >> group // setuid 4, setgid 2, sticky 1
			if c >= 'a' {         // lowercase: execute is set as well
				bits |= 1
			}
		default:
			return "", false
		}

		perm[group] = byte('0' + bits)
	}
	return string([]byte{byte('0' + special), perm[0], perm[1], perm[2]}), true
}
