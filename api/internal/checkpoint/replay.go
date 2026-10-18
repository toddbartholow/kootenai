// Package checkpoint provides checkpoint evaluation and event replay functionality
package checkpoint

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/events"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// ReplayResult represents the result of replaying events for a session
type ReplayResult struct {
	SessionID       string                             `json:"sessionId"`
	LabTemplate     string                             `json:"labTemplate"`
	TotalEvents     int                                `json:"totalEvents"`
	ProcessedEvents int                                `json:"processedEvents"`
	MatchedEvents   int                                `json:"matchedEvents"`
	Checkpoints     map[string]*CheckpointReplayResult `json:"checkpoints"`
	Timeline        []*EventReplayDetail               `json:"timeline"`
	Duration        time.Duration                      `json:"duration"`
	Errors          []string                           `json:"errors,omitempty"`
}

// CheckpointReplayResult represents replay results for a single checkpoint
type CheckpointReplayResult struct {
	CheckpointID   string                `json:"checkpointId"`
	Description    string                `json:"description"`
	Points         int                   `json:"points"`
	Status         string                `json:"status"` // passed, failed, pending
	PassedAt       *time.Time            `json:"passedAt,omitempty"`
	TriggersMet    map[int]bool          `json:"triggersMet"`
	TriggerDetails []TriggerReplayDetail `json:"triggerDetails"`
	MatchedEvents  []*EventReplayDetail  `json:"matchedEvents"`
}

// TriggerReplayDetail provides details about a trigger evaluation
type TriggerReplayDetail struct {
	TriggerIndex int            `json:"triggerIndex"`
	Type         string         `json:"type"`
	Target       string         `json:"target"`
	Match        map[string]any `json:"match"`
	Met          bool           `json:"met"`
	MetAt        *time.Time     `json:"metAt,omitempty"`
	MetByEventID int64          `json:"metByEventId,omitempty"`
	Reason       string         `json:"reason,omitempty"`
}

// EventReplayDetail provides details about an event during replay
type EventReplayDetail struct {
	EventID            int64          `json:"eventId"`
	Timestamp          time.Time      `json:"timestamp"`
	VMName             string         `json:"vmName"`
	EventType          string         `json:"eventType"`
	Data               map[string]any `json:"data,omitempty"`
	Matched            bool           `json:"matched"`
	MatchedCheckpoints []string       `json:"matchedCheckpoints,omitempty"`
	EvaluationDetails  []string       `json:"evaluationDetails,omitempty"`
}

// ReplayService provides event replay functionality for debugging
type ReplayService struct {
	logger       *slog.Logger
	evaluator    *Evaluator
	eventRepo    repositories.EventRepository
	sessionRepo  repositories.SessionRepository
	templateRepo repositories.LabTemplateRepository
}

// NewReplayService creates a new replay service
func NewReplayService(
	logger *slog.Logger,
	evaluator *Evaluator,
	eventRepo repositories.EventRepository,
	sessionRepo repositories.SessionRepository,
	templateRepo repositories.LabTemplateRepository,
) *ReplayService {
	return &ReplayService{
		logger:       logger,
		evaluator:    evaluator,
		eventRepo:    eventRepo,
		sessionRepo:  sessionRepo,
		templateRepo: templateRepo,
	}
}

// ReplaySession replays all events for a session and returns diagnostic information
func (s *ReplayService) ReplaySession(ctx context.Context, sessionID string) (*ReplayResult, error) {
	startTime := time.Now()

	// Get session info
	session, err := s.sessionRepo.GetByID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("getting session: %w", err)
	}
	if session == nil {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Get lab template
	templateRecord, err := s.templateRepo.GetByID(ctx, session.LabTemplateID)
	if err != nil {
		return nil, fmt.Errorf("getting template: %w", err)
	}
	if templateRecord == nil {
		return nil, fmt.Errorf("template not found: %s", session.LabTemplateID)
	}

	// Parse the template spec
	template, err := s.parseTemplateRecord(templateRecord)
	if err != nil {
		return nil, fmt.Errorf("parsing template: %w", err)
	}

	// Get all events for the session
	storedEvents, err := s.eventRepo.GetBySessionID(ctx, sessionID, 10000)
	if err != nil {
		return nil, fmt.Errorf("getting events: %w", err)
	}

	// Initialize result
	result := &ReplayResult{
		SessionID:   sessionID,
		LabTemplate: templateRecord.Name,
		TotalEvents: len(storedEvents),
		Checkpoints: make(map[string]*CheckpointReplayResult),
		Timeline:    make([]*EventReplayDetail, 0, len(storedEvents)),
	}

	// Initialize checkpoint results
	for _, cp := range template.Spec.Objectives {
		triggerDetails := make([]TriggerReplayDetail, len(cp.Triggers))
		for i, trigger := range cp.Triggers {
			matchMap := s.triggerMatchToMap(trigger.Match)
			triggerDetails[i] = TriggerReplayDetail{
				TriggerIndex: i,
				Type:         string(trigger.Type),
				Target:       trigger.Target,
				Match:        matchMap,
				Met:          false,
			}
		}

		result.Checkpoints[cp.ID] = &CheckpointReplayResult{
			CheckpointID:   cp.ID,
			Description:    cp.Description,
			Points:         cp.Points,
			Status:         "pending",
			TriggersMet:    make(map[int]bool),
			TriggerDetails: triggerDetails,
			MatchedEvents:  make([]*EventReplayDetail, 0),
		}
	}

	// Create a temporary evaluator state for this replay.
	//
	// Deliberately constructed WITHOUT WithMetrics: replay walks a session's
	// entire event history, so a metrics-wired evaluator would re-count every
	// checkpoint the student already passed and inflate checkpoints_passed_total
	// by the length of the history. This omission is load-bearing.
	tempEval := NewEvaluator(s.logger)
	if err := tempEval.RegisterTemplate(template); err != nil {
		return nil, fmt.Errorf("registering template: %w", err)
	}
	if err := tempEval.StartSession(sessionID, session.PodID, session.UserID, templateRecord.Name); err != nil {
		return nil, fmt.Errorf("starting replay session: %w", err)
	}

	// Process events in chronological order (reverse since they come DESC)
	for i := len(storedEvents) - 1; i >= 0; i-- {
		event := storedEvents[i]
		result.ProcessedEvents++

		// Convert stored event to VMEvent
		vmEvent := &events.VMEvent{
			MessageHeader: events.MessageHeader{
				ID:        fmt.Sprintf("%d", event.ID),
				Timestamp: event.Timestamp,
			},
			PodID:     event.PodID,
			VMName:    event.VMName,
			AgentID:   event.AgentID,
			EventType: event.EventType,
			Data:      event.Data,
		}

		// Evaluate the event
		passedCheckpoints, evalErr := tempEval.EvaluateEvent(ctx, vmEvent, sessionID)

		// Create event detail
		eventDetail := &EventReplayDetail{
			EventID:   event.ID,
			Timestamp: event.Timestamp,
			VMName:    event.VMName,
			EventType: event.EventType,
			Matched:   len(passedCheckpoints) > 0,
		}

		// Parse data for display
		if event.Data != nil {
			var dataMap map[string]any
			if json.Unmarshal(event.Data, &dataMap) == nil {
				eventDetail.Data = dataMap
			}
		}

		if evalErr != nil {
			eventDetail.EvaluationDetails = append(eventDetail.EvaluationDetails,
				fmt.Sprintf("Error: %v", evalErr))
			result.Errors = append(result.Errors, fmt.Sprintf("Event %d: %v", event.ID, evalErr))
		}

		// Record which checkpoints were matched
		if len(passedCheckpoints) > 0 {
			result.MatchedEvents++
			eventDetail.MatchedCheckpoints = passedCheckpoints

			// Update checkpoint results
			for _, cpID := range passedCheckpoints {
				if cpResult, ok := result.Checkpoints[cpID]; ok {
					cpResult.Status = "passed"
					cpResult.PassedAt = &event.Timestamp
					cpResult.MatchedEvents = append(cpResult.MatchedEvents, eventDetail)
				}
			}
		}

		// Add evaluation details
		eventDetail.EvaluationDetails = s.getEventEvaluationDetails(vmEvent, template, result.Checkpoints)

		result.Timeline = append(result.Timeline, eventDetail)
	}

	// Update trigger details based on final state
	state, _ := tempEval.GetSessionProgress(sessionID)
	if state != nil {
		for cpID, cpProgress := range state.Checkpoints {
			if cpResult, ok := result.Checkpoints[cpID]; ok {
				for triggerIdx, met := range cpProgress.TriggersMet {
					cpResult.TriggersMet[triggerIdx] = met
					if triggerIdx < len(cpResult.TriggerDetails) {
						cpResult.TriggerDetails[triggerIdx].Met = met
					}
				}
				if cpProgress.Status == models.CheckpointStatusPassed {
					cpResult.Status = "passed"
				} else if cpProgress.Status == models.CheckpointStatusFailed {
					cpResult.Status = "failed"
				}
			}
		}
	}

	result.Duration = time.Since(startTime)

	s.logger.Info("Event replay completed",
		slog.String("session_id", sessionID),
		slog.Int("total_events", result.TotalEvents),
		slog.Int("matched_events", result.MatchedEvents),
		slog.Duration("duration", result.Duration),
	)

	return result, nil
}

// ReplayEvent replays a single event and returns detailed evaluation information
func (s *ReplayService) ReplayEvent(ctx context.Context, sessionID string, eventID int64) (*EventReplayDetail, error) {
	// Get the event
	event, err := s.eventRepo.GetByID(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("getting event: %w", err)
	}
	if event == nil {
		return nil, fmt.Errorf("event not found: %d", eventID)
	}

	// Get session info
	session, err := s.sessionRepo.GetByID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("getting session: %w", err)
	}
	if session == nil {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Get lab template
	templateRecord, err := s.templateRepo.GetByID(ctx, session.LabTemplateID)
	if err != nil {
		return nil, fmt.Errorf("getting template: %w", err)
	}
	if templateRecord == nil {
		return nil, fmt.Errorf("template not found: %s", session.LabTemplateID)
	}

	// Parse the template spec
	template, err := s.parseTemplateRecord(templateRecord)
	if err != nil {
		return nil, fmt.Errorf("parsing template: %w", err)
	}

	// Convert to VMEvent
	vmEvent := &events.VMEvent{
		MessageHeader: events.MessageHeader{
			ID:        fmt.Sprintf("%d", event.ID),
			Timestamp: event.Timestamp,
		},
		PodID:     event.PodID,
		VMName:    event.VMName,
		AgentID:   event.AgentID,
		EventType: event.EventType,
		Data:      event.Data,
	}

	// Create event detail
	detail := &EventReplayDetail{
		EventID:   event.ID,
		Timestamp: event.Timestamp,
		VMName:    event.VMName,
		EventType: event.EventType,
	}

	// Parse data
	if event.Data != nil {
		var dataMap map[string]any
		if json.Unmarshal(event.Data, &dataMap) == nil {
			detail.Data = dataMap
		}
	}

	// Get detailed evaluation for each checkpoint
	checkpointResults := make(map[string]*CheckpointReplayResult)
	for _, cp := range template.Spec.Objectives {
		checkpointResults[cp.ID] = &CheckpointReplayResult{
			CheckpointID: cp.ID,
			Description:  cp.Description,
			Status:       "pending",
			TriggersMet:  make(map[int]bool),
		}
	}

	detail.EvaluationDetails = s.getEventEvaluationDetails(vmEvent, template, checkpointResults)

	// Check for matches
	for cpID, cpResult := range checkpointResults {
		if cpResult.Status == "passed" {
			detail.Matched = true
			detail.MatchedCheckpoints = append(detail.MatchedCheckpoints, cpID)
		}
	}

	return detail, nil
}

// getEventEvaluationDetails generates detailed evaluation information for an event
func (s *ReplayService) getEventEvaluationDetails(
	event *events.VMEvent,
	template *models.LabTemplate,
	checkpointResults map[string]*CheckpointReplayResult,
) []string {
	var details []string

	for _, cp := range template.Spec.Objectives {
		cpResult := checkpointResults[cp.ID]
		if cpResult == nil {
			continue
		}

		for i, trigger := range cp.Triggers {
			// Check if event type matches trigger
			if !s.evaluator.triggerTypeMatchesEvent(trigger.Type, event.EventType) {
				details = append(details,
					fmt.Sprintf("[%s] Trigger %d (%s): Event type mismatch (expected %s for trigger, got %s)",
						cp.ID, i, trigger.Type, trigger.Type, event.EventType))
				continue
			}

			// Check if VM matches target
			if trigger.Target != "" && trigger.Target != event.VMName {
				details = append(details,
					fmt.Sprintf("[%s] Trigger %d (%s): VM mismatch (expected %s, got %s)",
						cp.ID, i, trigger.Type, trigger.Target, event.VMName))
				continue
			}

			// Try to evaluate the match
			matched := s.evaluator.evaluateMatch(&trigger, event)
			if matched {
				details = append(details,
					fmt.Sprintf("[%s] Trigger %d (%s): MATCHED on VM %s",
						cp.ID, i, trigger.Type, event.VMName))
				cpResult.TriggersMet[i] = true

				// Check if all triggers are met
				allMet := true
				for j := range cp.Triggers {
					if !cpResult.TriggersMet[j] {
						allMet = false
						break
					}
				}
				if allMet || len(cp.Triggers) == 0 {
					cpResult.Status = "passed"
				}
			} else {
				details = append(details,
					fmt.Sprintf("[%s] Trigger %d (%s): No match on VM %s - criteria not satisfied",
						cp.ID, i, trigger.Type, event.VMName))
			}
		}
	}

	return details
}

// triggerMatchToMap converts TriggerMatch to a map for display
func (s *ReplayService) triggerMatchToMap(match models.TriggerMatch) map[string]any {
	result := make(map[string]any)

	if match.Path != "" {
		result["path"] = match.Path
	}
	if match.Contains != "" {
		result["contains"] = match.Contains
	}
	if match.Regex != "" {
		result["regex"] = match.Regex
	}
	if match.Package != "" {
		result["package"] = match.Package
	}
	if match.State != "" {
		result["state"] = match.State
	}
	if match.Name != "" {
		result["name"] = match.Name
	}
	if match.Pattern != "" {
		result["pattern"] = match.Pattern
	}
	if match.User != "" {
		result["user"] = match.User
	}
	if match.Protocol != "" {
		result["protocol"] = match.Protocol
	}
	if match.Port > 0 {
		result["port"] = match.Port
	}
	if match.Username != "" {
		result["username"] = match.Username
	}
	if match.MountPoint != "" {
		result["mountPoint"] = match.MountPoint
	}
	if match.ThresholdPct > 0 {
		result["thresholdPct"] = match.ThresholdPct
	}
	if match.ThresholdValue > 0 {
		result["thresholdValue"] = match.ThresholdValue
	}
	if match.Operator != "" {
		result["operator"] = match.Operator
	}
	if match.ProcessName != "" {
		result["processName"] = match.ProcessName
	}

	return result
}

// CompareReplays compares two replay results to find differences
func (s *ReplayService) CompareReplays(original, replay *ReplayResult) *ReplayComparison {
	comparison := &ReplayComparison{
		SessionID:       original.SessionID,
		OriginalEvents:  original.TotalEvents,
		ReplayedEvents:  replay.TotalEvents,
		EventsDiff:      replay.TotalEvents - original.TotalEvents,
		CheckpointDiffs: make([]CheckpointDiff, 0),
	}

	// Compare checkpoints
	for cpID, origCP := range original.Checkpoints {
		replayCP, exists := replay.Checkpoints[cpID]
		if !exists {
			comparison.CheckpointDiffs = append(comparison.CheckpointDiffs, CheckpointDiff{
				CheckpointID:   cpID,
				OriginalStatus: origCP.Status,
				ReplayStatus:   "missing",
				Different:      true,
			})
			continue
		}

		diff := CheckpointDiff{
			CheckpointID:   cpID,
			OriginalStatus: origCP.Status,
			ReplayStatus:   replayCP.Status,
			Different:      origCP.Status != replayCP.Status,
		}

		if diff.Different {
			diff.Reason = fmt.Sprintf("Status changed from %s to %s", origCP.Status, replayCP.Status)
		}

		comparison.CheckpointDiffs = append(comparison.CheckpointDiffs, diff)
	}

	comparison.HasDifferences = len(comparison.CheckpointDiffs) > 0
	for _, diff := range comparison.CheckpointDiffs {
		if diff.Different {
			comparison.HasDifferences = true
			break
		}
	}

	return comparison
}

// ReplayComparison represents a comparison between two replays
type ReplayComparison struct {
	SessionID       string           `json:"sessionId"`
	OriginalEvents  int              `json:"originalEvents"`
	ReplayedEvents  int              `json:"replayedEvents"`
	EventsDiff      int              `json:"eventsDiff"`
	HasDifferences  bool             `json:"hasDifferences"`
	CheckpointDiffs []CheckpointDiff `json:"checkpointDiffs"`
}

// CheckpointDiff represents a difference in checkpoint status between replays
type CheckpointDiff struct {
	CheckpointID   string `json:"checkpointId"`
	OriginalStatus string `json:"originalStatus"`
	ReplayStatus   string `json:"replayStatus"`
	Different      bool   `json:"different"`
	Reason         string `json:"reason,omitempty"`
}

// GetCheckpointDiagnostics returns detailed diagnostics for why a checkpoint is in its current state
func (s *ReplayService) GetCheckpointDiagnostics(ctx context.Context, sessionID, checkpointID string) (*CheckpointDiagnostics, error) {
	replay, err := s.ReplaySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	cpResult, ok := replay.Checkpoints[checkpointID]
	if !ok {
		return nil, fmt.Errorf("checkpoint not found: %s", checkpointID)
	}

	diagnostics := &CheckpointDiagnostics{
		CheckpointID:    checkpointID,
		Status:          cpResult.Status,
		TotalTriggers:   len(cpResult.TriggerDetails),
		MetTriggers:     0,
		Triggers:        cpResult.TriggerDetails,
		RelevantEvents:  make([]*EventReplayDetail, 0),
		Recommendations: make([]string, 0),
	}

	// Count met triggers
	for _, met := range cpResult.TriggersMet {
		if met {
			diagnostics.MetTriggers++
		}
	}

	// Find relevant events (those that match the checkpoint's trigger types)
	for _, event := range replay.Timeline {
		for _, trigger := range cpResult.TriggerDetails {
			if s.evaluator.triggerTypeMatchesEvent(models.TriggerType(trigger.Type), event.EventType) {
				diagnostics.RelevantEvents = append(diagnostics.RelevantEvents, event)
				break
			}
		}
	}

	// Generate recommendations
	if cpResult.Status != "passed" {
		for i, trigger := range cpResult.TriggerDetails {
			if !trigger.Met {
				recommendation := fmt.Sprintf("Trigger %d (%s) not met. ", i, trigger.Type)
				switch trigger.Type {
				case "file_exists", "file_content":
					if path, ok := trigger.Match["path"].(string); ok {
						recommendation += fmt.Sprintf("Ensure file '%s' exists/is modified.", path)
					}
				case "package":
					if pkg, ok := trigger.Match["package"].(string); ok {
						recommendation += fmt.Sprintf("Ensure package '%s' is installed.", pkg)
					}
				case "service":
					if name, ok := trigger.Match["name"].(string); ok {
						recommendation += fmt.Sprintf("Ensure service '%s' is in expected state.", name)
					}
				case "command_executed":
					if pattern, ok := trigger.Match["pattern"].(string); ok {
						recommendation += fmt.Sprintf("Execute command matching pattern '%s'.", pattern)
					}
				default:
					recommendation += "Check trigger criteria and ensure conditions are met."
				}
				diagnostics.Recommendations = append(diagnostics.Recommendations, recommendation)
			}
		}
	}

	return diagnostics, nil
}

// CheckpointDiagnostics provides detailed diagnostics for a checkpoint
type CheckpointDiagnostics struct {
	CheckpointID    string                `json:"checkpointId"`
	Status          string                `json:"status"`
	TotalTriggers   int                   `json:"totalTriggers"`
	MetTriggers     int                   `json:"metTriggers"`
	Triggers        []TriggerReplayDetail `json:"triggers"`
	RelevantEvents  []*EventReplayDetail  `json:"relevantEvents"`
	Recommendations []string              `json:"recommendations"`
}

// parseTemplateRecord converts a LabTemplateRecord to a LabTemplate
func (s *ReplayService) parseTemplateRecord(record *models.LabTemplateRecord) (*models.LabTemplate, error) {
	template := &models.LabTemplate{
		Metadata: models.LabMetadata{
			Name:        record.Name,
			Description: record.Description,
			Version:     record.Version,
		},
	}

	// Parse the spec from JSON
	if len(record.Spec) > 0 {
		if err := json.Unmarshal(record.Spec, &template.Spec); err != nil {
			return nil, fmt.Errorf("parsing template spec: %w", err)
		}
	}

	// If checkpoints are stored separately, parse them
	if len(record.Checkpoints) > 0 {
		var checkpoints []models.Checkpoint
		if err := json.Unmarshal(record.Checkpoints, &checkpoints); err != nil {
			return nil, fmt.Errorf("parsing checkpoints: %w", err)
		}
		template.Spec.Objectives = checkpoints
	}

	return template, nil
}
