package events

import (
	"encoding/json"
	"testing"
)

func TestNewAchievementJob(t *testing.T) {
	job := NewAchievementJob(
		"session-123",
		"user-456",
		"template-789",
		true,
		95.5,
		19,
		20,
	)

	if job.SessionID != "session-123" {
		t.Errorf("Expected session ID 'session-123', got '%s'", job.SessionID)
	}

	if job.UserID != "user-456" {
		t.Errorf("Expected user ID 'user-456', got '%s'", job.UserID)
	}

	if job.LabTemplateID != "template-789" {
		t.Errorf("Expected template ID 'template-789', got '%s'", job.LabTemplateID)
	}

	if !job.Passed {
		t.Error("Expected passed to be true")
	}

	if job.Percentage != 95.5 {
		t.Errorf("Expected percentage 95.5, got %f", job.Percentage)
	}

	if job.EarnedPoints != 19 {
		t.Errorf("Expected earned points 19, got %d", job.EarnedPoints)
	}

	if job.MaxPoints != 20 {
		t.Errorf("Expected max points 20, got %d", job.MaxPoints)
	}

	if job.Priority != JobPriorityNormal {
		t.Errorf("Expected normal priority, got %d", job.Priority)
	}

	if job.MaxRetry != 3 {
		t.Errorf("Expected max retry 3, got %d", job.MaxRetry)
	}

	if job.ID == "" {
		t.Error("Expected message ID to be set")
	}

	if job.Source != "session-handler" {
		t.Errorf("Expected source 'session-handler', got '%s'", job.Source)
	}
}

func TestNewGradeSyncJob(t *testing.T) {
	job := NewGradeSyncJob(
		"session-123",
		"user-456",
		18,
		20,
		90.0,
		"canvas-assign-1",
		"canvas-course-1",
		"canvas-user-1",
	)

	if job.SessionID != "session-123" {
		t.Errorf("Expected session ID 'session-123', got '%s'", job.SessionID)
	}

	if job.EarnedPoints != 18 {
		t.Errorf("Expected earned points 18, got %d", job.EarnedPoints)
	}

	if job.MaxPoints != 20 {
		t.Errorf("Expected max points 20, got %d", job.MaxPoints)
	}

	if job.Percentage != 90.0 {
		t.Errorf("Expected percentage 90.0, got %f", job.Percentage)
	}

	if job.CanvasAssignmentID != "canvas-assign-1" {
		t.Errorf("Expected canvas assignment ID, got '%s'", job.CanvasAssignmentID)
	}

	if job.Priority != JobPriorityHigh {
		t.Errorf("Expected high priority for grade sync, got %d", job.Priority)
	}

	if job.MaxRetry != 10 {
		t.Errorf("Expected max retry 10 for grade sync, got %d", job.MaxRetry)
	}
}

func TestNewEventProcessingJob(t *testing.T) {
	eventIDs := []int64{1, 2, 3, 4, 5}
	job := NewEventProcessingJob("pod-123", "session-456", eventIDs)

	if job.PodID != "pod-123" {
		t.Errorf("Expected pod ID 'pod-123', got '%s'", job.PodID)
	}

	if job.SessionID != "session-456" {
		t.Errorf("Expected session ID 'session-456', got '%s'", job.SessionID)
	}

	if len(job.EventIDs) != 5 {
		t.Errorf("Expected 5 event IDs, got %d", len(job.EventIDs))
	}

	if job.BatchSize != 100 {
		t.Errorf("Expected batch size 100, got %d", job.BatchSize)
	}

	if job.Priority != JobPriorityNormal {
		t.Errorf("Expected normal priority, got %d", job.Priority)
	}

	if job.MaxRetry != 5 {
		t.Errorf("Expected max retry 5, got %d", job.MaxRetry)
	}
}

func TestParseAchievementJob(t *testing.T) {
	original := NewAchievementJob("sess-1", "user-1", "tmpl-1", true, 100.0, 10, 10)
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal job: %v", err)
	}

	parsed, err := ParseAchievementJob(data)
	if err != nil {
		t.Fatalf("Failed to parse job: %v", err)
	}

	if parsed.SessionID != original.SessionID {
		t.Errorf("Expected session ID '%s', got '%s'", original.SessionID, parsed.SessionID)
	}

	if parsed.UserID != original.UserID {
		t.Errorf("Expected user ID '%s', got '%s'", original.UserID, parsed.UserID)
	}

	if parsed.Passed != original.Passed {
		t.Errorf("Expected passed %v, got %v", original.Passed, parsed.Passed)
	}
}

func TestParseAchievementJobInvalid(t *testing.T) {
	_, err := ParseAchievementJob([]byte("invalid json"))
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestParseGradeSyncJob(t *testing.T) {
	original := NewGradeSyncJob("sess-1", "user-1", 9, 10, 90.0, "assign-1", "course-1", "cuser-1")
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal job: %v", err)
	}

	parsed, err := ParseGradeSyncJob(data)
	if err != nil {
		t.Fatalf("Failed to parse job: %v", err)
	}

	if parsed.CanvasAssignmentID != original.CanvasAssignmentID {
		t.Errorf("Expected canvas assignment ID '%s', got '%s'", original.CanvasAssignmentID, parsed.CanvasAssignmentID)
	}

	if parsed.Percentage != original.Percentage {
		t.Errorf("Expected percentage %f, got %f", original.Percentage, parsed.Percentage)
	}
}

func TestParseGradeSyncJobInvalid(t *testing.T) {
	_, err := ParseGradeSyncJob([]byte("not json"))
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestParseEventProcessingJob(t *testing.T) {
	original := NewEventProcessingJob("pod-1", "sess-1", []int64{100, 200, 300})
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal job: %v", err)
	}

	parsed, err := ParseEventProcessingJob(data)
	if err != nil {
		t.Fatalf("Failed to parse job: %v", err)
	}

	if parsed.PodID != original.PodID {
		t.Errorf("Expected pod ID '%s', got '%s'", original.PodID, parsed.PodID)
	}

	if len(parsed.EventIDs) != len(original.EventIDs) {
		t.Errorf("Expected %d event IDs, got %d", len(original.EventIDs), len(parsed.EventIDs))
	}

	for i, id := range original.EventIDs {
		if parsed.EventIDs[i] != id {
			t.Errorf("Event ID mismatch at index %d: expected %d, got %d", i, id, parsed.EventIDs[i])
		}
	}
}

func TestParseEventProcessingJobInvalid(t *testing.T) {
	_, err := ParseEventProcessingJob([]byte("{broken"))
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestJobPriorityConstants(t *testing.T) {
	if JobPriorityLow >= JobPriorityNormal {
		t.Error("Expected low priority to be less than normal")
	}

	if JobPriorityNormal >= JobPriorityHigh {
		t.Error("Expected normal priority to be less than high")
	}
}

func TestAchievementJobWithEnrollment(t *testing.T) {
	job := NewAchievementJob("sess-1", "user-1", "tmpl-1", true, 100.0, 10, 10)

	enrollmentID := "enroll-123"
	moduleID := "module-456"
	job.EnrollmentID = &enrollmentID
	job.ModuleID = &moduleID

	data, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("Failed to marshal job: %v", err)
	}

	parsed, err := ParseAchievementJob(data)
	if err != nil {
		t.Fatalf("Failed to parse job: %v", err)
	}

	if parsed.EnrollmentID == nil || *parsed.EnrollmentID != enrollmentID {
		t.Error("Expected enrollment ID to be preserved")
	}

	if parsed.ModuleID == nil || *parsed.ModuleID != moduleID {
		t.Error("Expected module ID to be preserved")
	}
}

func TestJobResultSerialization(t *testing.T) {
	result := JobResult{
		MessageHeader:       NewMessageHeader("worker"),
		JobID:               "job-123",
		JobType:             "achievement",
		Success:             true,
		Duration:            150,
		AchievementsAwarded: 2,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal result: %v", err)
	}

	var parsed JobResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if parsed.JobID != result.JobID {
		t.Errorf("Expected job ID '%s', got '%s'", result.JobID, parsed.JobID)
	}

	if parsed.Success != result.Success {
		t.Errorf("Expected success %v, got %v", result.Success, parsed.Success)
	}

	if parsed.AchievementsAwarded != result.AchievementsAwarded {
		t.Errorf("Expected %d achievements awarded, got %d", result.AchievementsAwarded, parsed.AchievementsAwarded)
	}
}

func TestJobResultWithError(t *testing.T) {
	result := JobResult{
		MessageHeader: NewMessageHeader("worker"),
		JobID:         "job-456",
		JobType:       "gradesync",
		Success:       false,
		Error:         "Canvas API timeout",
		Retries:       3,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal result: %v", err)
	}

	var parsed JobResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if parsed.Error != "Canvas API timeout" {
		t.Errorf("Expected error message to be preserved, got '%s'", parsed.Error)
	}

	if parsed.Retries != 3 {
		t.Errorf("Expected 3 retries, got %d", parsed.Retries)
	}
}
