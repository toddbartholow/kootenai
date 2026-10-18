// Package models contains shared data types for the lab platform
package models

import "time"

// TriggerType defines the type of checkpoint trigger
type TriggerType string

const (
	TriggerTypeFileExists      TriggerType = "file_exists"
	TriggerTypeFileContent     TriggerType = "file_content"
	TriggerTypeFileDeleted     TriggerType = "file_deleted"
	TriggerTypePackage         TriggerType = "package"
	TriggerTypeService         TriggerType = "service"
	TriggerTypeCommandExecuted TriggerType = "command_executed"
	TriggerTypeUserCreated     TriggerType = "user_created"
	TriggerTypePermission      TriggerType = "permission_changed"
	TriggerTypeNetwork         TriggerType = "network_connection"
	TriggerTypeActiveCheck     TriggerType = "active_check"
	TriggerTypeCustom          TriggerType = "custom"
	// System metric triggers (for resource monitoring)
	TriggerTypeDiskUsage      TriggerType = "disk_usage"
	TriggerTypeCPULoad        TriggerType = "cpu_load"
	TriggerTypeMemoryUsage    TriggerType = "memory_usage"
	TriggerTypeProcessRunning TriggerType = "process_running"
	TriggerTypePortListening  TriggerType = "port_listening"
	TriggerTypeCronJob        TriggerType = "cron_job"
	TriggerTypeFirewallRule   TriggerType = "firewall_rule"
)

// AllTriggerTypes lists every trigger type the platform recognises.
//
// The template validator accepts exactly these. It used to carry its own
// hand-maintained list, which had fallen seven behind the constants above --
// disk_usage, cpu_load, memory_usage, process_running, port_listening,
// cron_job and firewall_rule were implemented in the checkpoint evaluator but
// rejected at validation, so no template could use them.
//
// Keep this in sync with TriggerType in python/lab_templates/parser.py.
var AllTriggerTypes = []TriggerType{
	TriggerTypeFileExists,
	TriggerTypeFileContent,
	TriggerTypeFileDeleted,
	TriggerTypePackage,
	TriggerTypeService,
	TriggerTypeCommandExecuted,
	TriggerTypeUserCreated,
	TriggerTypePermission,
	TriggerTypeNetwork,
	TriggerTypeActiveCheck,
	TriggerTypeCustom,
	TriggerTypeDiskUsage,
	TriggerTypeCPULoad,
	TriggerTypeMemoryUsage,
	TriggerTypeProcessRunning,
	TriggerTypePortListening,
	TriggerTypeCronJob,
	TriggerTypeFirewallRule,
}

// CheckpointStatus represents the state of a checkpoint
type CheckpointStatus string

const (
	CheckpointStatusPending             CheckpointStatus = "pending"
	CheckpointStatusPassed              CheckpointStatus = "passed"
	CheckpointStatusFailed              CheckpointStatus = "failed"
	CheckpointStatusSkipped             CheckpointStatus = "skipped"
	CheckpointStatusPartial             CheckpointStatus = "partial"
	CheckpointStatusPendingVerification CheckpointStatus = "pending_verification"
	CheckpointStatusVerificationFailed  CheckpointStatus = "verification_failed"
)

// TriggerMatch defines the matching criteria for a trigger
type TriggerMatch struct {
	// File-based triggers
	Path       string `yaml:"path,omitempty" json:"path,omitempty"`
	Contains   string `yaml:"contains,omitempty" json:"contains,omitempty"`
	Regex      string `yaml:"regex,omitempty" json:"regex,omitempty"`
	Mode       string `yaml:"mode,omitempty" json:"mode,omitempty"`
	Owner      string `yaml:"owner,omitempty" json:"owner,omitempty"`
	Group      string `yaml:"group,omitempty" json:"group,omitempty"`
	Permission string `yaml:"permission,omitempty" json:"permission,omitempty"` // e.g., "0644", "rw-r--r--"

	// Package triggers
	Package string `yaml:"package,omitempty" json:"package,omitempty"`
	State   string `yaml:"state,omitempty" json:"state,omitempty"` // installed, removed, active, inactive

	// Service triggers
	Name string `yaml:"name,omitempty" json:"name,omitempty"`

	// Command triggers
	Pattern string `yaml:"pattern,omitempty" json:"pattern,omitempty"`
	User    string `yaml:"user,omitempty" json:"user,omitempty"`

	// Network triggers
	Protocol    string `yaml:"protocol,omitempty" json:"protocol,omitempty"`
	Destination string `yaml:"destination,omitempty" json:"destination,omitempty"` // Remote address
	Address     string `yaml:"address,omitempty" json:"address,omitempty"`         // Alias for destination
	Port        int    `yaml:"port,omitempty" json:"port,omitempty"`

	// User triggers
	Username string   `yaml:"username,omitempty" json:"username,omitempty"`
	UID      int      `yaml:"uid,omitempty" json:"uid,omitempty"`
	Groups   []string `yaml:"groups,omitempty" json:"groups,omitempty"`

	// System metric triggers
	MountPoint     string  `yaml:"mount_point,omitempty" json:"mountPoint,omitempty"`         // For disk_usage
	ThresholdPct   float64 `yaml:"threshold_pct,omitempty" json:"thresholdPct,omitempty"`     // Percentage threshold (0-100)
	ThresholdValue float64 `yaml:"threshold_value,omitempty" json:"thresholdValue,omitempty"` // Absolute value threshold
	Operator       string  `yaml:"operator,omitempty" json:"operator,omitempty"`              // lt, le, eq, ge, gt
	ProcessName    string  `yaml:"process_name,omitempty" json:"processName,omitempty"`       // For process_running
	ProcessCount   int     `yaml:"process_count,omitempty" json:"processCount,omitempty"`     // Min process count
	ListenAddress  string  `yaml:"listen_address,omitempty" json:"listenAddress,omitempty"`   // For port_listening (0.0.0.0, 127.0.0.1)

	// Firewall triggers
	Action    string `yaml:"action,omitempty" json:"action,omitempty"`       // accept, drop, reject
	Chain     string `yaml:"chain,omitempty" json:"chain,omitempty"`         // INPUT, OUTPUT, FORWARD
	Interface string `yaml:"interface,omitempty" json:"interface,omitempty"` // eth0, ens192, etc.

	// Cron triggers
	Schedule string `yaml:"schedule,omitempty" json:"schedule,omitempty"` // Cron expression to match
	Command  string `yaml:"command,omitempty" json:"command,omitempty"`   // Command pattern to match
}

// CheckpointTrigger defines a single trigger condition for a checkpoint
type CheckpointTrigger struct {
	Type   TriggerType  `yaml:"type" json:"type"`
	Target string       `yaml:"target" json:"target"` // VM name
	Match  TriggerMatch `yaml:"match" json:"match"`
	Script string       `yaml:"script,omitempty" json:"script,omitempty"` // For active_check type
}

// Checkpoint defines a graded checkpoint in a lab
type Checkpoint struct {
	ID             string              `yaml:"id" json:"id"`
	Description    string              `yaml:"description" json:"description"`
	Points         int                 `yaml:"points" json:"points"`
	Hint           string              `yaml:"hint,omitempty" json:"hint,omitempty"`   // Legacy single hint (backward compatible)
	Hints          []HintLevel         `yaml:"hints,omitempty" json:"hints,omitempty"` // Progressive hints
	DependsOn      []string            `yaml:"depends_on,omitempty" json:"dependsOn,omitempty"`
	TimeoutMinutes int                 `yaml:"timeout_minutes,omitempty" json:"timeoutMinutes,omitempty"`
	Required       bool                `yaml:"required,omitempty" json:"required,omitempty"`
	Order          int                 `yaml:"order,omitempty" json:"order,omitempty"`
	Triggers       []CheckpointTrigger `yaml:"triggers" json:"triggers"`
}

// GetHintCount returns the total number of available hints
func (c *Checkpoint) GetHintCount() int {
	if len(c.Hints) > 0 {
		return len(c.Hints)
	}
	if c.Hint != "" {
		return 1
	}
	return 0
}

// GetHintForLevel returns the hint for a specific level (1-indexed)
// Returns nil if level is invalid
func (c *Checkpoint) GetHintForLevel(level int) *HintLevel {
	if level < 1 {
		return nil
	}

	// Progressive hints take precedence
	if len(c.Hints) > 0 {
		for i := range c.Hints {
			if c.Hints[i].Level == level {
				return &c.Hints[i]
			}
		}
		// If no matching level found, try by index
		if level <= len(c.Hints) {
			return &c.Hints[level-1]
		}
		return nil
	}

	// Fallback to legacy single hint (level 1 only)
	if level == 1 && c.Hint != "" {
		return &HintLevel{
			Level:        1,
			Text:         c.Hint,
			PointPenalty: 0,
		}
	}

	return nil
}

// HasHints returns true if any hints are available
func (c *Checkpoint) HasHints() bool {
	return c.GetHintCount() > 0
}

// CheckpointConfig defines the overall checkpoint configuration for a lab
type CheckpointConfig struct {
	Enabled        bool `yaml:"enabled" json:"enabled"`
	PassThreshold  int  `yaml:"pass_threshold,omitempty" json:"passThreshold,omitempty"` // Percentage to pass
	AllowRetry     bool `yaml:"allow_retry,omitempty" json:"allowRetry,omitempty"`
	ShowHints      bool `yaml:"show_hints,omitempty" json:"showHints,omitempty"`
	RealTimeUpdate bool `yaml:"realtime_update,omitempty" json:"realtimeUpdate,omitempty"`
	GradeOnSubmit  bool `yaml:"grade_on_submit,omitempty" json:"gradeOnSubmit,omitempty"`
}

// CheckpointState represents the current state of a checkpoint in a session
type CheckpointState struct {
	CheckpointID  string           `json:"checkpointId"`
	Status        CheckpointStatus `json:"status"`
	Points        int              `json:"points"`
	EarnedPoints  int              `json:"earnedPoints"`
	PassedAt      *time.Time       `json:"passedAt,omitempty"`
	TriggeredBy   string           `json:"triggeredBy,omitempty"` // Event ID that triggered
	AttemptCount  int              `json:"attemptCount"`
	LastAttemptAt *time.Time       `json:"lastAttemptAt,omitempty"`
	Feedback      string           `json:"feedback,omitempty"`
}

// CheckpointProgress represents checkpoint progress for database operations
type CheckpointProgress struct {
	ID             string           `json:"id"`
	SessionID      string           `json:"sessionId"`
	CheckpointID   string           `json:"checkpointId"`
	Status         CheckpointStatus `json:"status"`
	Points         int              `json:"points"`
	EarnedPoints   int              `json:"earnedPoints"`
	PassedAt       *time.Time       `json:"passedAt,omitempty"`
	FailedAt       *time.Time       `json:"failedAt,omitempty"`
	TriggerEventID *string          `json:"triggerEventId,omitempty"`
	TriggerType    string           `json:"triggerType,omitempty"`
	AttemptCount   int              `json:"attemptCount"`
	LastAttemptAt  *time.Time       `json:"lastAttemptAt,omitempty"`
	Feedback       string           `json:"feedback,omitempty"`
	CreatedAt      time.Time        `json:"createdAt"`
	UpdatedAt      time.Time        `json:"updatedAt"`
}

// CheckpointHintProgress tracks progressive hint usage for checkpoints
type CheckpointHintProgress struct {
	ID                 string    `json:"id"`
	SessionID          string    `json:"sessionId"`
	CheckpointID       string    `json:"checkpointId"`
	HintLevelShown     int       `json:"hintLevelShown"`     // Highest hint level revealed (0 = none)
	HintPenaltyApplied int       `json:"hintPenaltyApplied"` // Total penalty from hints
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}
