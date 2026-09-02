package events

import (
	"encoding/json"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// -----------------------------------------------------------------------------
// Base Message Types
// -----------------------------------------------------------------------------

// MessageHeader contains common fields for all messages
type MessageHeader struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Source    string    `json:"source"`  // Component that generated the message
	Version   string    `json:"version"` // Message schema version
}

// -----------------------------------------------------------------------------
// VM Event Messages (from Wazuh)
// -----------------------------------------------------------------------------

// VMEvent represents an event received from a VM's Wazuh agent
type VMEvent struct {
	MessageHeader

	// Context
	PodID     string `json:"podId"`
	SessionID string `json:"sessionId,omitempty"`
	VMName    string `json:"vmName"`
	AgentID   string `json:"agentId"`

	// Wazuh event details
	RuleID      int      `json:"ruleId,omitempty"`
	RuleLevel   int      `json:"ruleLevel,omitempty"`
	Description string   `json:"description,omitempty"`
	Groups      []string `json:"groups,omitempty"`
	Location    string   `json:"location,omitempty"`
	Decoder     string   `json:"decoder,omitempty"`

	// Wazuh alert reference (from webhook)
	WazuhAlertID  string `json:"wazuhAlertId,omitempty"`
	WazuhRuleID   string `json:"wazuhRuleId,omitempty"`
	WazuhRuleDesc string `json:"wazuhRuleDesc,omitempty"`
	WazuhLevel    int    `json:"wazuhLevel,omitempty"`

	// Event type and payload
	EventType string          `json:"eventType"`
	Timestamp time.Time       `json:"eventTimestamp,omitempty"`
	Data      json.RawMessage `json:"data"`
}

// SyscheckEvent represents a file integrity monitoring event
type SyscheckEvent struct {
	VMEvent
	Syscheck models.SyscheckData `json:"syscheck"`
}

// AuditEvent represents a command execution audit event
type AuditEvent struct {
	VMEvent
	Audit models.AuditData `json:"audit"`
}

// PackageEvent represents a package installation/removal event
type PackageEvent struct {
	VMEvent
	Package models.PackageData `json:"package"`
}

// ServiceEvent represents a service state change event
type ServiceEvent struct {
	VMEvent
	Service models.ServiceData `json:"service"`
}

// UserEvent represents a user account change event
type UserEvent struct {
	VMEvent
	User models.UserData `json:"user"`
}

// -----------------------------------------------------------------------------
// Checkpoint Messages
// -----------------------------------------------------------------------------

// CheckpointUpdate represents a checkpoint state change
type CheckpointUpdate struct {
	MessageHeader

	PodID        string                  `json:"podId"`
	SessionID    string                  `json:"sessionId"`
	CheckpointID string                  `json:"checkpointId"`
	Action       string                  `json:"action"` // passed, failed, reset, skipped
	Status       models.CheckpointStatus `json:"status"`

	// Scoring
	Points       int `json:"points"`
	EarnedPoints int `json:"earnedPoints"`

	// What triggered this update
	TriggerType    string `json:"triggerType,omitempty"`
	TriggerEventID string `json:"triggerEventId,omitempty"`

	// Updated session totals
	SessionEarnedPoints int     `json:"sessionEarnedPoints"`
	SessionMaxPoints    int     `json:"sessionMaxPoints"`
	SessionPercentage   float64 `json:"sessionPercentage"`

	// Optional feedback
	Feedback string `json:"feedback,omitempty"`
}

// -----------------------------------------------------------------------------
// Session Messages
// -----------------------------------------------------------------------------

// SessionEvent represents a session lifecycle event
type SessionEvent struct {
	MessageHeader

	SessionID string `json:"sessionId"`
	PodID     string `json:"podId"`
	UserID    string `json:"userId"`
	Action    string `json:"action"` // started, ended, paused, resumed, reset, submitted

	// Current state
	LabTemplate  string  `json:"labTemplate"`
	EarnedPoints int     `json:"earnedPoints"`
	MaxPoints    int     `json:"maxPoints"`
	Percentage   float64 `json:"percentage"`
	Passed       bool    `json:"passed"`

	// Timing
	StartedAt *time.Time `json:"startedAt,omitempty"`
	EndedAt   *time.Time `json:"endedAt,omitempty"`
	DueAt     *time.Time `json:"dueAt,omitempty"`

	// Canvas integration
	CanvasAssignmentID string `json:"canvasAssignmentId,omitempty"`
}

// -----------------------------------------------------------------------------
// Grade Messages
// -----------------------------------------------------------------------------

// GradeUpdate represents a grade change that may need to be synced to Canvas
type GradeUpdate struct {
	MessageHeader

	SessionID string `json:"sessionId"`
	UserID    string `json:"userId"`

	// Grade info
	EarnedPoints int     `json:"earnedPoints"`
	MaxPoints    int     `json:"maxPoints"`
	Percentage   float64 `json:"percentage"`
	Passed       bool    `json:"passed"`

	// Canvas details
	CanvasAssignmentID string `json:"canvasAssignmentId,omitempty"`
	CanvasCourseID     string `json:"canvasCourseId,omitempty"`
	CanvasUserID       string `json:"canvasUserId,omitempty"`

	// Sync status
	RequiresSync bool       `json:"requiresSync"`
	SyncedAt     *time.Time `json:"syncedAt,omitempty"`
	SyncError    string     `json:"syncError,omitempty"`

	// Checkpoint summary
	CheckpointStates []models.CheckpointState `json:"checkpointStates,omitempty"`
}

// -----------------------------------------------------------------------------
// WebSocket Broadcast Messages
// -----------------------------------------------------------------------------

// WebSocketMessage is a wrapper for messages broadcast to WebSocket clients
type WebSocketMessage struct {
	Type    string          `json:"type"`    // event, checkpoint, session, grade, monitoring_event
	Subject string          `json:"subject"` // NATS subject this came from
	Payload json.RawMessage `json:"payload"`
}

// MonitoringEvent represents an event for the real-time monitoring dashboard
type MonitoringEvent struct {
	ID                 int64           `json:"id"`
	Timestamp          time.Time       `json:"timestamp"`
	PodID              string          `json:"podId"`
	SessionID          string          `json:"sessionId,omitempty"`
	VMName             string          `json:"vmName"`
	AgentID            string          `json:"agentId,omitempty"`
	EventType          string          `json:"eventType"`
	RuleID             string          `json:"ruleId,omitempty"`
	RuleLevel          int             `json:"ruleLevel,omitempty"`
	Description        string          `json:"description,omitempty"`
	Data               json.RawMessage `json:"data,omitempty"`
	Processed          bool            `json:"processed"`
	MatchedCheckpoints []string        `json:"matchedCheckpoints,omitempty"`
}

// CheckpointBroadcast is sent to WebSocket clients when a checkpoint changes
type CheckpointBroadcast struct {
	PodID        string                  `json:"podId"`
	SessionID    string                  `json:"sessionId"`
	CheckpointID string                  `json:"checkpointId"`
	Status       models.CheckpointStatus `json:"status"`
	EarnedPoints int                     `json:"earnedPoints"`
	Points       int                     `json:"points"`

	// Session totals for progress bar
	SessionEarnedPoints int     `json:"sessionEarnedPoints"`
	SessionMaxPoints    int     `json:"sessionMaxPoints"`
	SessionPercentage   float64 `json:"sessionPercentage"`
	SessionPassed       bool    `json:"sessionPassed"`

	Timestamp time.Time `json:"timestamp"`
}

// -----------------------------------------------------------------------------
// Job Messages
// -----------------------------------------------------------------------------

// JobPriority defines job execution priority
type JobPriority int

const (
	JobPriorityLow    JobPriority = 0
	JobPriorityNormal JobPriority = 5
	JobPriorityHigh   JobPriority = 10
)

// AchievementEvaluationJob is queued when a session completes to check achievements
type AchievementEvaluationJob struct {
	MessageHeader

	SessionID string `json:"sessionId"`
	UserID    string `json:"userId"`

	// Session details for evaluation
	LabTemplateID string  `json:"labTemplateId"`
	Passed        bool    `json:"passed"`
	Percentage    float64 `json:"percentage"`
	EarnedPoints  int     `json:"earnedPoints"`
	MaxPoints     int     `json:"maxPoints"`

	// Optional context
	EnrollmentID *string `json:"enrollmentId,omitempty"`
	ModuleID     *string `json:"moduleId,omitempty"`

	// Job metadata
	Priority JobPriority `json:"priority"`
	Retries  int         `json:"retries"`
	MaxRetry int         `json:"maxRetry"`
}

// GradeSyncJob is queued when grades need to be synced to Canvas
type GradeSyncJob struct {
	MessageHeader

	SessionID string `json:"sessionId"`
	UserID    string `json:"userId"`

	// Grade details
	EarnedPoints int     `json:"earnedPoints"`
	MaxPoints    int     `json:"maxPoints"`
	Percentage   float64 `json:"percentage"`

	// Canvas details
	CanvasAssignmentID string `json:"canvasAssignmentId"`
	CanvasCourseID     string `json:"canvasCourseId"`
	CanvasUserID       string `json:"canvasUserId"`

	// Job metadata
	Priority JobPriority `json:"priority"`
	Retries  int         `json:"retries"`
	MaxRetry int         `json:"maxRetry"`
}

// EventProcessingJob is queued for batch event processing
type EventProcessingJob struct {
	MessageHeader

	PodID     string `json:"podId"`
	SessionID string `json:"sessionId,omitempty"`
	VMName    string `json:"vmName,omitempty"`

	// Events to process
	EventIDs []int64 `json:"eventIds,omitempty"`

	// Job metadata
	Priority  JobPriority `json:"priority"`
	BatchSize int         `json:"batchSize"`
	Retries   int         `json:"retries"`
	MaxRetry  int         `json:"maxRetry"`
}

// JobResult represents the outcome of a job execution
type JobResult struct {
	MessageHeader

	JobID     string    `json:"jobId"`
	JobType   string    `json:"jobType"`
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
	Duration  int64     `json:"durationMs"`
	Retries   int       `json:"retries"`
	Timestamp time.Time `json:"completedAt"`

	// Type-specific results
	AchievementsAwarded int  `json:"achievementsAwarded,omitempty"`
	EventsProcessed     int  `json:"eventsProcessed,omitempty"`
	GradeSynced         bool `json:"gradeSynced,omitempty"`
}

// NewAchievementJob creates a new achievement evaluation job
func NewAchievementJob(sessionID, userID, labTemplateID string, passed bool, percentage float64, earnedPoints, maxPoints int) *AchievementEvaluationJob {
	return &AchievementEvaluationJob{
		MessageHeader: NewMessageHeader("session-handler"),
		SessionID:     sessionID,
		UserID:        userID,
		LabTemplateID: labTemplateID,
		Passed:        passed,
		Percentage:    percentage,
		EarnedPoints:  earnedPoints,
		MaxPoints:     maxPoints,
		Priority:      JobPriorityNormal,
		MaxRetry:      3,
	}
}

// NewGradeSyncJob creates a new grade sync job
func NewGradeSyncJob(sessionID, userID string, earnedPoints, maxPoints int, percentage float64, canvasAssignmentID, canvasCourseID, canvasUserID string) *GradeSyncJob {
	return &GradeSyncJob{
		MessageHeader:      NewMessageHeader("grade-handler"),
		SessionID:          sessionID,
		UserID:             userID,
		EarnedPoints:       earnedPoints,
		MaxPoints:          maxPoints,
		Percentage:         percentage,
		CanvasAssignmentID: canvasAssignmentID,
		CanvasCourseID:     canvasCourseID,
		CanvasUserID:       canvasUserID,
		Priority:           JobPriorityHigh,
		MaxRetry:           10, // Retry more for external sync
	}
}

// NewEventProcessingJob creates a new event processing job
func NewEventProcessingJob(podID, sessionID string, eventIDs []int64) *EventProcessingJob {
	return &EventProcessingJob{
		MessageHeader: NewMessageHeader("event-handler"),
		PodID:         podID,
		SessionID:     sessionID,
		EventIDs:      eventIDs,
		Priority:      JobPriorityNormal,
		BatchSize:     100,
		MaxRetry:      5,
	}
}

// ParseAchievementJob parses a raw message as an AchievementEvaluationJob
func ParseAchievementJob(data []byte) (*AchievementEvaluationJob, error) {
	var job AchievementEvaluationJob
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, err
	}
	return &job, nil
}

// ParseGradeSyncJob parses a raw message as a GradeSyncJob
func ParseGradeSyncJob(data []byte) (*GradeSyncJob, error) {
	var job GradeSyncJob
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, err
	}
	return &job, nil
}

// ParseEventProcessingJob parses a raw message as an EventProcessingJob
func ParseEventProcessingJob(data []byte) (*EventProcessingJob, error) {
	var job EventProcessingJob
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, err
	}
	return &job, nil
}

// -----------------------------------------------------------------------------
// Helper Functions
// -----------------------------------------------------------------------------

// NewMessageHeader creates a new message header with current timestamp
func NewMessageHeader(source string) MessageHeader {
	return MessageHeader{
		ID:        generateMessageID(),
		Timestamp: time.Now().UTC(),
		Source:    source,
		Version:   "1.0",
	}
}

// generateMessageID creates a unique message ID
func generateMessageID() string {
	// Use timestamp + random suffix for uniqueness
	return time.Now().UTC().Format("20060102150405.000000")
}

// ParseVMEvent attempts to parse a raw message as a VMEvent
func ParseVMEvent(data []byte) (*VMEvent, error) {
	var event VMEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, err
	}
	return &event, nil
}

// ParseCheckpointUpdate attempts to parse a raw message as a CheckpointUpdate
func ParseCheckpointUpdate(data []byte) (*CheckpointUpdate, error) {
	var update CheckpointUpdate
	if err := json.Unmarshal(data, &update); err != nil {
		return nil, err
	}
	return &update, nil
}

// ParseSessionEvent attempts to parse a raw message as a SessionEvent
func ParseSessionEvent(data []byte) (*SessionEvent, error) {
	var event SessionEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, err
	}
	return &event, nil
}

// ToJSON serializes any message to JSON bytes
func ToJSON(msg any) ([]byte, error) {
	return json.Marshal(msg)
}
