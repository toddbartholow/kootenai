// Package models contains shared data types for the lab platform
package models

import (
	"encoding/json"
	"time"
)

// EventRecord represents a stored event for audit/replay
type EventRecord struct {
	ID          int64           `json:"id"`
	Timestamp   time.Time       `json:"timestamp"`
	PodID       string          `json:"podId"`
	SessionID   string          `json:"sessionId"`
	VMName      string          `json:"vmName"`
	AgentID     string          `json:"agentId"`
	EventType   string          `json:"eventType"`
	RuleID      int             `json:"ruleId,omitempty"`
	Data        json.RawMessage `json:"data"`
	Processed   bool            `json:"processed"`
	Checkpoints []string        `json:"matchedCheckpoints,omitempty"`
}

// GradeReport represents a final grade report for Canvas integration
type GradeReport struct {
	SessionID        string            `json:"sessionId"`
	UserID           string            `json:"userId"`
	LabTemplate      string            `json:"labTemplate"`
	TotalPoints      int               `json:"totalPoints"`
	MaxPoints        int               `json:"maxPoints"`
	Percentage       float64           `json:"percentage"`
	Passed           bool              `json:"passed"`
	CompletedAt      time.Time         `json:"completedAt"`
	Duration         time.Duration     `json:"duration"`
	CheckpointReport []CheckpointState `json:"checkpoints"`
	Comments         string            `json:"comments,omitempty"`
}

// Session represents a lab session entity for database persistence.
// This is the canonical session model for repositories and HTTP handlers.
type Session struct {
	ID                 string            `json:"id"`
	PodID              string            `json:"podId"`
	UserID             string            `json:"userId"`
	LabTemplateID      string            `json:"labTemplateId"`
	Status             string            `json:"status"` // active, paused, completed, submitted, expired
	EarnedPoints       int               `json:"earnedPoints"`
	MaxPoints          int               `json:"maxPoints"`
	Percentage         float64           `json:"percentage"`
	Passed             bool              `json:"passed"`
	PassingThreshold   int               `json:"passingThreshold"`
	CanvasCourseID     string            `json:"canvasCourseId,omitempty"`
	CanvasAssignmentID string            `json:"canvasAssignmentId,omitempty"`
	CanvasUserID       string            `json:"canvasUserId,omitempty"`
	StartedAt          time.Time         `json:"startedAt"`
	EndedAt            *time.Time        `json:"endedAt,omitempty"`
	DueAt              *time.Time        `json:"dueAt,omitempty"`
	GradeSyncedAt      *time.Time        `json:"gradeSyncedAt,omitempty"`
	GradeSyncError     string            `json:"gradeSyncError,omitempty"`
	Metadata           map[string]string `json:"metadata,omitempty"`
	// Pathway tracking fields
	EnrollmentID *string `json:"enrollmentId,omitempty"`
	ModuleID     *string `json:"moduleId,omitempty"`
	// Multi-tenancy fields
	OrganizationID *string `json:"organizationId,omitempty"`
	TeamID         *string `json:"teamId,omitempty"`
	// LTI deep linking
	LTIAssignmentID *string `json:"ltiAssignmentId,omitempty"`
}

// Event represents an event entity for database operations
type Event struct {
	ID                 int64           `json:"id"`
	Timestamp          time.Time       `json:"timestamp"`
	PodID              string          `json:"podId"`
	SessionID          string          `json:"sessionId,omitempty"`
	VMName             string          `json:"vmName"`
	AgentID            string          `json:"agentId"`
	EventType          string          `json:"eventType"`
	Source             string          `json:"source"`
	WazuhAlertID       string          `json:"wazuhAlertId,omitempty"`
	WazuhRuleID        string          `json:"wazuhRuleId,omitempty"`
	WazuhRuleDesc      string          `json:"wazuhRuleDesc,omitempty"`
	WazuhLevel         int             `json:"wazuhLevel,omitempty"`
	Data               json.RawMessage `json:"data"`
	Processed          bool            `json:"processed"`
	MatchedCheckpoints []string        `json:"matchedCheckpoints,omitempty"`
}

// GradeSyncEntry represents a grade sync queue entry
type GradeSyncEntry struct {
	ID                 string     `json:"id"`
	SessionID          string     `json:"sessionId"`
	Status             string     `json:"status"` // pending, processing, completed, failed
	EarnedPoints       int        `json:"earnedPoints"`
	MaxPoints          int        `json:"maxPoints"`
	Percentage         float64    `json:"percentage"`
	CanvasCourseID     string     `json:"canvasCourseId"`
	CanvasAssignmentID string     `json:"canvasAssignmentId"`
	CanvasUserID       string     `json:"canvasUserId"`
	Attempts           int        `json:"attempts"`
	LastAttemptAt      *time.Time `json:"lastAttemptAt,omitempty"`
	CompletedAt        *time.Time `json:"completedAt,omitempty"`
	ErrorMessage       string     `json:"errorMessage,omitempty"`
	CreatedAt          time.Time  `json:"createdAt"`
}
