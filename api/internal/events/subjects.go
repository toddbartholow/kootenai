// Package events defines NATS subjects and message types for the event pipeline
package events

// -----------------------------------------------------------------------------
// NATS Subject Hierarchy
// -----------------------------------------------------------------------------
//
// Subject structure for lab platform events:
//
//   labs.events.{pod_id}.{vm_name}.{event_type}
//   labs.checkpoints.{pod_id}.{checkpoint_id}
//   labs.sessions.{session_id}.{action}
//   labs.grades.{session_id}
//
// Examples:
//   labs.events.abc123.workstation.syscheck     - File integrity event
//   labs.events.abc123.workstation.audit        - Command execution event
//   labs.checkpoints.abc123.enable-ufw          - Checkpoint state change
//   labs.sessions.xyz789.started                - Session lifecycle
//   labs.grades.xyz789                          - Grade update
//

const (
	// StreamName is the NATS JetStream stream name for lab events
	StreamName = "LABS"

	// Subject prefixes
	SubjectPrefixEvents      = "labs.events"
	SubjectPrefixCheckpoints = "labs.checkpoints"
	SubjectPrefixSessions    = "labs.sessions"
	SubjectPrefixGrades      = "labs.grades"
	SubjectPrefixPods        = "labs.pods"
	SubjectPrefixJobs        = "labs.jobs"

	// Wildcard subjects for consumers
	SubjectAllEvents      = "labs.events.>"
	SubjectAllCheckpoints = "labs.checkpoints.>"
	SubjectAllSessions    = "labs.sessions.>"
	SubjectAllGrades      = "labs.grades.>"
	SubjectAllPods        = "labs.pods.>"
	SubjectAllJobs        = "labs.jobs.>"

	// Job subjects
	SubjectJobsAchievements = "labs.jobs.achievements"
	SubjectJobsGradeSync    = "labs.jobs.gradesync"
	SubjectJobsEvents       = "labs.jobs.events"

	// Consumer names
	ConsumerEventStore         = "event-store"
	ConsumerCheckpointEval     = "checkpoint-evaluator"
	ConsumerWebSocketBroadcast = "websocket-broadcast"
	ConsumerGradeSync          = "grade-sync"
	ConsumerAchievementWorker  = "achievement-worker"
	ConsumerEventProcessor     = "event-processor"
)

// Event types from Wazuh
const (
	EventTypeSyscheck      = "syscheck"      // File integrity monitoring
	EventTypeAudit         = "audit"         // Command/syscall auditing
	EventTypePackage       = "package"       // Package installation
	EventTypeService       = "service"       // Service state changes
	EventTypeAuth          = "auth"          // Authentication events
	EventTypeSSH           = "ssh"           // SSH connection events
	EventTypeFirewall      = "firewall"      // Firewall rule changes
	EventTypeNetwork       = "network"       // Network connections
	EventTypeUser          = "user"          // User account changes
	EventTypeVulnerability = "vulnerability" // Vulnerability scan results
	EventTypeCustom        = "custom"        // Custom check results
	EventTypeGeneric       = "generic"       // Unclassified events
	// System metric event types
	EventTypeDiskUsage   = "disk_usage"   // Disk space monitoring
	EventTypeCPULoad     = "cpu_load"     // CPU load monitoring
	EventTypeMemoryUsage = "memory_usage" // Memory usage monitoring
	EventTypeProcess     = "process"      // Process state monitoring
	EventTypePortListen  = "port_listen"  // Port listening status
	EventTypeCron        = "cron"         // Cron job events
)

// Session actions
const (
	SessionActionStarted   = "started"
	SessionActionEnded     = "ended"
	SessionActionPaused    = "paused"
	SessionActionResumed   = "resumed"
	SessionActionReset     = "reset"
	SessionActionSubmitted = "submitted"
)

// Checkpoint actions
const (
	CheckpointActionPassed  = "passed"
	CheckpointActionFailed  = "failed"
	CheckpointActionReset   = "reset"
	CheckpointActionSkipped = "skipped"
)

// Hint actions
const (
	SubjectPrefixHints = "labs.hints"
	SubjectAllHints    = "labs.hints.>"
)

// BuildEventSubject constructs a NATS subject for VM events
func BuildEventSubject(podID, vmName, eventType string) string {
	return SubjectPrefixEvents + "." + podID + "." + vmName + "." + eventType
}

// BuildCheckpointSubject constructs a NATS subject for checkpoint updates
func BuildCheckpointSubject(podID, checkpointID string) string {
	return SubjectPrefixCheckpoints + "." + podID + "." + checkpointID
}

// BuildSessionSubject constructs a NATS subject for session events
func BuildSessionSubject(sessionID, action string) string {
	return SubjectPrefixSessions + "." + sessionID + "." + action
}

// BuildGradeSubject constructs a NATS subject for grade updates
func BuildGradeSubject(sessionID string) string {
	return SubjectPrefixGrades + "." + sessionID
}

// BuildPodEventsSubject constructs a wildcard subject for all events from a pod
func BuildPodEventsSubject(podID string) string {
	return SubjectPrefixEvents + "." + podID + ".>"
}

// BuildPodCheckpointsSubject constructs a wildcard subject for all checkpoints in a pod
func BuildPodCheckpointsSubject(podID string) string {
	return SubjectPrefixCheckpoints + "." + podID + ".>"
}

// BuildPodProvisionSubject constructs a NATS subject for pod provisioning events
func BuildPodProvisionSubject(podID string) string {
	return SubjectPrefixPods + "." + podID + ".provision"
}

// BuildPodStatusSubject constructs a wildcard subject for all status updates for a pod
func BuildPodStatusSubject(podID string) string {
	return SubjectPrefixPods + "." + podID + ".>"
}
