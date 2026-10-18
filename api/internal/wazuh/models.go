package wazuh

import (
	"time"
)

// AgentRegistration represents a request to register a new Wazuh agent
type AgentRegistration struct {
	Name    string `json:"name"`
	IP      string `json:"ip"`
	GroupID string `json:"group,omitempty"`
	PodID   string `json:"pod_id"` // Links agent to a lab pod
}

// AgentInfo represents information about a registered agent
type AgentInfo struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	IP            string    `json:"ip"`
	Status        string    `json:"status"`
	Group         string    `json:"group,omitempty"`
	OS            AgentOS   `json:"os,omitempty"`
	Version       string    `json:"version,omitempty"`
	RegisteredAt  time.Time `json:"dateAdd,omitempty"`
	LastKeepAlive time.Time `json:"lastKeepAlive,omitempty"`
}

// AgentOS contains agent operating system information
type AgentOS struct {
	Platform string `json:"platform,omitempty"`
	Name     string `json:"name,omitempty"`
	Version  string `json:"version,omitempty"`
	Arch     string `json:"arch,omitempty"`
}

// AgentKey represents the key returned when registering an agent
type AgentKey struct {
	ID  string `json:"id"`
	Key string `json:"key"`
}

// CheckpointMatcher defines how to match Wazuh events to lab checkpoints
type CheckpointMatcher struct {
	// CheckpointID is the ID of the checkpoint in the lab template
	CheckpointID string `json:"checkpoint_id"`

	// RuleIDs matches specific Wazuh rule IDs
	RuleIDs []string `json:"rule_ids,omitempty"`

	// RuleGroups matches any rule in these groups
	RuleGroups []string `json:"rule_groups,omitempty"`

	// MinLevel requires rule level >= this value
	MinLevel int `json:"min_level,omitempty"`

	// FilePaths matches FIM events on these paths (glob patterns supported)
	FilePaths []string `json:"file_paths,omitempty"`

	// FileEvents matches specific FIM event types (added, modified, deleted)
	FileEvents []string `json:"file_events,omitempty"`

	// ContentMatch requires the file to contain this regex pattern
	ContentMatch string `json:"content_match,omitempty"`

	// LogMatch matches against full_log with regex
	LogMatch string `json:"log_match,omitempty"`

	// RequireAll if true, all conditions must match; if false, any condition triggers
	RequireAll bool `json:"require_all,omitempty"`
}

// WebhookPayload is the structure sent by Wazuh integrator
type WebhookPayload struct {
	Alert Alert `json:"alert"`
}
