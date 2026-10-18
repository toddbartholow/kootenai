package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/achievements"
	"github.com/toddbartholow/kootenai/api/internal/checkpoint"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/events"
	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/testutil/mocks"
)

// TestUserLabFlow_EndToEnd simulates the complete flow:
// 1. Create a new user
// 2. Create a pod based on a lab template
// 3. Start a session connecting user to pod
// 4. Complete checkpoints (simulate Wazuh events)
// 5. Submit the lab
// 6. Check grades and achievements
func TestUserLabFlow_EndToEnd(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// =========================================================================
	// Step 1: Create repositories and services
	// =========================================================================
	userRepo := mocks.NewFakeUserRepository()
	podRepo := mocks.NewFakePodRepository()
	sessionRepo := mocks.NewFakeSessionRepository()
	templateRepo := mocks.NewFakeLabTemplateRepository()
	achievementRepo := newMockAchievementRepo()

	// Create evaluator for checkpoint tracking
	evaluator := checkpoint.NewEvaluator(logger)

	// Create achievement service
	sessionRepoAdapter := &sessionRepoListAdapter{FakeSessionRepository: sessionRepo}
	achievementService := achievements.NewService(achievementRepo, sessionRepoAdapter, logger)

	// =========================================================================
	// Step 2: Create a new user
	// =========================================================================
	t.Log("Step 1: Creating new user...")
	newUser := &models.User{
		ID:          "user-test-001",
		ExternalID:  "ext-test-001",
		Username:    "testuser",
		Email:       "testuser@example.com",
		DisplayName: "Test User",
		Role:        "student",
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err := userRepo.Create(ctx, newUser)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Verify user was created
	createdUser, err := userRepo.GetByID(ctx, newUser.ID)
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}
	if createdUser.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", createdUser.Username)
	}
	t.Logf("   Created user: %s (%s)", createdUser.DisplayName, createdUser.ID)

	// =========================================================================
	// Step 3: Create a lab template with checkpoints
	// =========================================================================
	t.Log("Step 2: Creating lab template...")
	labTemplate := &models.LabTemplate{
		APIVersion: "v1",
		Kind:       "LabTemplate",
		Metadata: models.LabMetadata{
			Name:        "Linux Basics 101",
			Description: "Learn basic Linux commands",
			Duration:    "30m",
			Difficulty:  "beginner",
		},
		Spec: models.LabSpec{
			Platform: models.PlatformProxmox,
			VMs: []models.VMSpec{
				{
					Name:     "linux-vm",
					Template: "ubuntu-22.04",
					Resources: models.ResourceSpec{
						CPU:    2,
						Memory: 2048,
					},
					WazuhAgent: true,
				},
			},
			Checkpoints: &models.CheckpointConfig{
				Enabled:       true,
				PassThreshold: 70,
				ShowHints:     true,
			},
			Objectives: []models.Checkpoint{
				{
					ID:          "cp-create-file",
					Description: "Create a file named hello.txt",
					Points:      30,
					Hint:        "Use the touch or echo command",
					Triggers: []models.CheckpointTrigger{
						{
							Type:   models.TriggerTypeFileExists,
							Target: "linux-vm",
							Match:  models.TriggerMatch{Path: "/home/student/hello.txt"},
						},
					},
				},
				{
					ID:          "cp-install-nginx",
					Description: "Install nginx web server",
					Points:      40,
					Hint:        "Use apt install nginx",
					Triggers: []models.CheckpointTrigger{
						{
							Type:   models.TriggerTypePackage,
							Target: "linux-vm",
							Match:  models.TriggerMatch{Package: "nginx", State: "installed"},
						},
					},
				},
				{
					ID:          "cp-start-nginx",
					Description: "Start and enable nginx service",
					Points:      30,
					DependsOn:   []string{"cp-install-nginx"},
					Triggers: []models.CheckpointTrigger{
						{
							Type:   models.TriggerTypeService,
							Target: "linux-vm",
							Match:  models.TriggerMatch{Name: "nginx", State: "active"},
						},
					},
				},
			},
		},
	}

	// Create template record for database
	templateRecord, err := models.NewLabTemplateRecord(labTemplate)
	if err != nil {
		t.Fatalf("Failed to create template record: %v", err)
	}
	templateRecord.ID = "template-linux-101"

	err = templateRepo.Create(ctx, templateRecord)
	if err != nil {
		t.Fatalf("Failed to store template: %v", err)
	}

	// Register template with evaluator
	err = evaluator.RegisterTemplate(labTemplate)
	if err != nil {
		t.Fatalf("Failed to register template: %v", err)
	}
	t.Logf("   Created lab: %s (max points: %d)", labTemplate.Metadata.Name, templateRecord.MaxPoints)

	// =========================================================================
	// Step 4: Create a pod for the user
	// =========================================================================
	t.Log("Step 3: Creating pod for user...")
	expiresAt := time.Now().Add(2 * time.Hour)
	pod := &models.Pod{
		ID:            "pod-test-001",
		LabTemplateID: templateRecord.ID,
		LabTemplate:   labTemplate.Metadata.Name,
		Platform:      models.PlatformProxmox,
		OwnerID:       newUser.ID,
		Owner:         newUser.Username,
		Status:        models.PodStatusRunning,
		VMs: []models.PodVM{
			{
				Name:       "linux-vm",
				PlatformID: "100",
				Platform:   models.PlatformProxmox,
				Node:       "pve",
				Status:     "running",
				IPAddress:  "192.168.100.10",
			},
		},
		Networks: []models.PodNetwork{
			{
				Name:   "lab-network",
				VLAN:   100,
				Subnet: "192.168.100.0/24",
			},
		},
		CreatedAt: time.Now(),
		ExpiresAt: &expiresAt,
		Metadata:  map[string]string{"purpose": "learning"},
	}

	err = podRepo.Create(ctx, pod)
	if err != nil {
		t.Fatalf("Failed to create pod: %v", err)
	}
	t.Logf("   Created pod: %s (status: %s)", pod.ID, pod.Status)

	// =========================================================================
	// Step 5: Start a lab session
	// =========================================================================
	t.Log("Step 4: Starting lab session...")
	session := &models.Session{
		ID:               "session-test-001",
		PodID:            pod.ID,
		UserID:           newUser.ID,
		LabTemplateID:    templateRecord.ID,
		Status:           "active",
		MaxPoints:        templateRecord.MaxPoints,
		PassingThreshold: templateRecord.PassThreshold,
		Metadata:         map[string]string{"source": "test"},
	}

	err = sessionRepo.Create(ctx, session)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Start session in evaluator
	err = evaluator.StartSession(session.ID, pod.ID, newUser.ID, labTemplate.Metadata.Name)
	if err != nil {
		t.Fatalf("Failed to start evaluator session: %v", err)
	}
	t.Logf("   Started session: %s", session.ID)

	// =========================================================================
	// Step 6: Simulate checkpoint completion via Wazuh events
	// =========================================================================
	t.Log("Step 5: Simulating checkpoint completion...")

	// Event 1: Create file /home/student/hello.txt
	fileEvent := &events.VMEvent{
		PodID:     pod.ID,
		VMName:    "linux-vm",
		EventType: events.EventTypeSyscheck,
		Data: json.RawMessage(`{
			"path": "/home/student/hello.txt",
			"event": "added"
		}`),
	}
	matchedCheckpoints, err := evaluator.EvaluateEvent(ctx, fileEvent, session.ID)
	if err != nil {
		t.Fatalf("Failed to evaluate file event: %v", err)
	}
	t.Logf("   Checkpoint 1 (create file): matched %v", matchedCheckpoints)

	// Event 2: Install nginx package
	packageEvent := &events.VMEvent{
		PodID:     pod.ID,
		VMName:    "linux-vm",
		EventType: events.EventTypePackage,
		Data: json.RawMessage(`{
			"package": "nginx",
			"version": "1.18.0",
			"action": "install"
		}`),
	}
	matchedCheckpoints, err = evaluator.EvaluateEvent(ctx, packageEvent, session.ID)
	if err != nil {
		t.Fatalf("Failed to evaluate package event: %v", err)
	}
	t.Logf("   Checkpoint 2 (install nginx): matched %v", matchedCheckpoints)

	// Event 3: Start nginx service
	serviceEvent := &events.VMEvent{
		PodID:     pod.ID,
		VMName:    "linux-vm",
		EventType: events.EventTypeService,
		Data: json.RawMessage(`{
			"unit": "nginx",
			"state": "active"
		}`),
	}
	matchedCheckpoints, err = evaluator.EvaluateEvent(ctx, serviceEvent, session.ID)
	if err != nil {
		t.Fatalf("Failed to evaluate service event: %v", err)
	}
	t.Logf("   Checkpoint 3 (start nginx): matched %v", matchedCheckpoints)

	// =========================================================================
	// Step 7: Check session progress
	// =========================================================================
	t.Log("Step 6: Checking session progress...")
	progress, err := evaluator.GetSessionProgress(session.ID)
	if err != nil {
		t.Fatalf("Failed to get session progress: %v", err)
	}
	t.Logf("   Progress: %d/%d points earned", progress.EarnedPoints, progress.MaxPoints)

	// =========================================================================
	// Step 8: Submit the lab and calculate grades
	// =========================================================================
	t.Log("Step 7: Submitting lab...")

	// Update session with final scores
	session.EarnedPoints = progress.EarnedPoints
	session.Percentage = float64(progress.EarnedPoints) / float64(progress.MaxPoints) * 100
	session.Passed = session.Percentage >= float64(session.PassingThreshold)
	session.Status = "submitted"
	now := time.Now()
	session.EndedAt = &now

	err = sessionRepo.Update(ctx, session)
	if err != nil {
		t.Fatalf("Failed to update session: %v", err)
	}

	// Also add to the list adapter for achievement checking
	sessionRepoAdapter.sessions = append(sessionRepoAdapter.sessions, session)

	t.Logf("   Submitted: earned %d/%d points (%.1f%%)", session.EarnedPoints, session.MaxPoints, session.Percentage)
	t.Logf("   Passed: %v (threshold: %d%%)", session.Passed, session.PassingThreshold)

	// =========================================================================
	// Step 9: Check for achievements
	// =========================================================================
	t.Log("Step 8: Checking achievements...")

	// Add some achievements to the repository
	testAchievements := []*models.Achievement{
		{
			ID:          "ach-first-lab",
			Name:        "First Steps",
			Description: "Complete your first lab",
			Type:        models.AchievementTypeLabCompletion,
			Tier:        models.AchievementTierBronze,
			Points:      10,
			IsActive:    true,
			Criteria:    models.AchievementCriteria{TotalLabs: 1},
		},
		{
			ID:          "ach-perfect",
			Name:        "Perfectionist",
			Description: "Score 100% on any lab",
			Type:        models.AchievementTypePerfectScore,
			Tier:        models.AchievementTierGold,
			Points:      100,
			IsActive:    true,
			Criteria:    models.AchievementCriteria{RequirePerfect: true},
		},
	}
	achievementRepo.achievements = testAchievements

	awarded, err := achievementService.CheckAndAwardAchievements(ctx, session, newUser.ID)
	if err != nil {
		t.Fatalf("Failed to check achievements: %v", err)
	}

	t.Logf("   Achievements awarded: %d", len(awarded))
	for _, a := range awarded {
		t.Logf("     - %s", a.AchievementID)
	}

	// =========================================================================
	// Step 10: Verify final grades
	// =========================================================================
	t.Log("Step 9: Verifying final grades...")

	// Get updated session
	finalSession, err := sessionRepo.GetByID(ctx, session.ID)
	if err != nil {
		t.Fatalf("Failed to get final session: %v", err)
	}

	// Build grade report
	gradeReport := models.GradeReport{
		SessionID:   finalSession.ID,
		UserID:      newUser.ID,
		LabTemplate: labTemplate.Metadata.Name,
		TotalPoints: finalSession.EarnedPoints,
		MaxPoints:   finalSession.MaxPoints,
		Percentage:  finalSession.Percentage,
		Passed:      finalSession.Passed,
		CompletedAt: *finalSession.EndedAt,
		Duration:    finalSession.EndedAt.Sub(finalSession.StartedAt),
	}

	t.Log("")
	t.Log("=== GRADE REPORT ===")
	t.Logf("Student:     %s (%s)", newUser.DisplayName, newUser.Email)
	t.Logf("Lab:         %s", gradeReport.LabTemplate)
	t.Logf("Score:       %d / %d points", gradeReport.TotalPoints, gradeReport.MaxPoints)
	t.Logf("Percentage:  %.1f%%", gradeReport.Percentage)
	t.Logf("Result:      %s", map[bool]string{true: "PASSED", false: "FAILED"}[gradeReport.Passed])
	t.Logf("Duration:    %s", gradeReport.Duration.Round(time.Second))
	t.Logf("Achievements: %d earned", len(awarded))
	t.Log("====================")

	// =========================================================================
	// Assertions
	// =========================================================================
	t.Log("Step 10: Running assertions...")

	// User should exist
	if createdUser == nil {
		t.Error("User should exist")
	}

	// Session should be submitted
	if finalSession.Status != "submitted" {
		t.Errorf("Expected session status 'submitted', got '%s'", finalSession.Status)
	}

	// Session should have ended
	if finalSession.EndedAt == nil {
		t.Error("Session should have an end time")
	}

	// Score should reflect completed checkpoints
	if finalSession.EarnedPoints < 0 || finalSession.EarnedPoints > finalSession.MaxPoints {
		t.Errorf("Earned points (%d) should be between 0 and max (%d)",
			finalSession.EarnedPoints, finalSession.MaxPoints)
	}

	// Pass status should match threshold
	expectedPass := finalSession.Percentage >= float64(finalSession.PassingThreshold)
	if finalSession.Passed != expectedPass {
		t.Errorf("Pass status mismatch: got %v, expected %v", finalSession.Passed, expectedPass)
	}

	t.Log("All assertions passed!")
}

// TestUserLabFlow_PartialCompletion tests a student who doesn't complete all checkpoints
func TestUserLabFlow_PartialCompletion(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	userRepo := mocks.NewFakeUserRepository()
	sessionRepo := mocks.NewFakeSessionRepository()
	evaluator := checkpoint.NewEvaluator(logger)

	// Create user
	user := &models.User{
		ID:       "user-partial-001",
		Username: "partial_student",
		Role:     "student",
		IsActive: true,
	}
	_ = userRepo.Create(ctx, user)

	// Create lab template with 3 checkpoints (30, 40, 30 points)
	labTemplate := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "Partial Test Lab"},
		Spec: models.LabSpec{
			Checkpoints: &models.CheckpointConfig{
				Enabled:       true,
				PassThreshold: 70, // Need 70 out of 100
			},
			Objectives: []models.Checkpoint{
				{ID: "cp-1", Points: 30, Description: "Task 1",
					Triggers: []models.CheckpointTrigger{
						{Type: models.TriggerTypeFileExists, Target: "vm", Match: models.TriggerMatch{Path: "/task1"}},
					}},
				{ID: "cp-2", Points: 40, Description: "Task 2",
					Triggers: []models.CheckpointTrigger{
						{Type: models.TriggerTypeFileExists, Target: "vm", Match: models.TriggerMatch{Path: "/task2"}},
					}},
				{ID: "cp-3", Points: 30, Description: "Task 3",
					Triggers: []models.CheckpointTrigger{
						{Type: models.TriggerTypeFileExists, Target: "vm", Match: models.TriggerMatch{Path: "/task3"}},
					}},
			},
		},
	}
	_ = evaluator.RegisterTemplate(labTemplate)

	// Create session
	session := &models.Session{
		ID:               "session-partial-001",
		PodID:            "pod-partial-001",
		UserID:           user.ID,
		MaxPoints:        100,
		PassingThreshold: 70,
		Status:           "active",
	}
	_ = sessionRepo.Create(ctx, session)
	_ = evaluator.StartSession(session.ID, session.PodID, user.ID, "Partial Test Lab")

	// Only complete 2 of 3 checkpoints (70 points = exactly at threshold)
	event1 := &events.VMEvent{
		PodID:     session.PodID,
		VMName:    "vm",
		EventType: events.EventTypeSyscheck,
		Data:      json.RawMessage(`{"path": "/task1", "event": "added"}`),
	}
	_, _ = evaluator.EvaluateEvent(ctx, event1, session.ID)

	event2 := &events.VMEvent{
		PodID:     session.PodID,
		VMName:    "vm",
		EventType: events.EventTypeSyscheck,
		Data:      json.RawMessage(`{"path": "/task2", "event": "added"}`),
	}
	_, _ = evaluator.EvaluateEvent(ctx, event2, session.ID)

	// Get progress
	progress, _ := evaluator.GetSessionProgress(session.ID)

	// Calculate grade
	session.EarnedPoints = progress.EarnedPoints
	session.Percentage = float64(progress.EarnedPoints) / float64(progress.MaxPoints) * 100
	session.Passed = session.Percentage >= float64(session.PassingThreshold)

	t.Logf("Partial completion: %d/%d points (%.1f%%)", session.EarnedPoints, session.MaxPoints, session.Percentage)
	t.Logf("Passed: %v", session.Passed)

	// Should have 70 points and pass
	if session.EarnedPoints != 70 {
		t.Errorf("Expected 70 points, got %d", session.EarnedPoints)
	}
	if !session.Passed {
		t.Error("Expected to pass with 70%")
	}
}

// TestUserLabFlow_FailedLab tests a student who doesn't meet the passing threshold
func TestUserLabFlow_FailedLab(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	evaluator := checkpoint.NewEvaluator(logger)

	// Create lab template
	labTemplate := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "Fail Test Lab"},
		Spec: models.LabSpec{
			Checkpoints: &models.CheckpointConfig{
				Enabled:       true,
				PassThreshold: 70,
			},
			Objectives: []models.Checkpoint{
				{ID: "cp-1", Points: 50, Description: "Task 1",
					Triggers: []models.CheckpointTrigger{
						{Type: models.TriggerTypeFileExists, Target: "vm", Match: models.TriggerMatch{Path: "/task1"}},
					}},
				{ID: "cp-2", Points: 50, Description: "Task 2",
					Triggers: []models.CheckpointTrigger{
						{Type: models.TriggerTypeFileExists, Target: "vm", Match: models.TriggerMatch{Path: "/task2"}},
					}},
			},
		},
	}
	_ = evaluator.RegisterTemplate(labTemplate)

	// Start session
	_ = evaluator.StartSession("session-fail", "pod-fail", "user-fail", "Fail Test Lab")

	// Only complete 1 checkpoint (50 points)
	event1 := &events.VMEvent{
		PodID:     "pod-fail",
		VMName:    "vm",
		EventType: events.EventTypeSyscheck,
		Data:      json.RawMessage(`{"path": "/task1", "event": "added"}`),
	}
	_, _ = evaluator.EvaluateEvent(ctx, event1, "session-fail")

	// Get progress
	progress, _ := evaluator.GetSessionProgress("session-fail")

	// Calculate grade
	percentage := float64(progress.EarnedPoints) / float64(progress.MaxPoints) * 100
	passed := percentage >= 70

	t.Logf("Failed lab: %d/%d points (%.1f%%)", progress.EarnedPoints, progress.MaxPoints, percentage)
	t.Logf("Passed: %v", passed)

	// Should have 50 points and fail
	if progress.EarnedPoints != 50 {
		t.Errorf("Expected 50 points, got %d", progress.EarnedPoints)
	}
	if passed {
		t.Error("Expected to fail with 50%")
	}
}

// TestUserLabFlow_MultipleUsers tests multiple users taking the same lab
func TestUserLabFlow_MultipleUsers(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	evaluator := checkpoint.NewEvaluator(logger)

	// Create lab template
	labTemplate := &models.LabTemplate{
		Metadata: models.LabMetadata{Name: "Multi-User Lab"},
		Spec: models.LabSpec{
			Objectives: []models.Checkpoint{
				{ID: "cp-1", Points: 100, Description: "Complete task",
					Triggers: []models.CheckpointTrigger{
						{Type: models.TriggerTypeFileExists, Target: "vm", Match: models.TriggerMatch{Path: "/done"}},
					}},
			},
		},
	}
	_ = evaluator.RegisterTemplate(labTemplate)

	// User 1: Completes the lab
	_ = evaluator.StartSession("session-u1", "pod-u1", "user-1", "Multi-User Lab")
	event1 := &events.VMEvent{
		PodID:     "pod-u1",
		VMName:    "vm",
		EventType: events.EventTypeSyscheck,
		Data:      json.RawMessage(`{"path": "/done", "event": "added"}`),
	}
	_, _ = evaluator.EvaluateEvent(ctx, event1, "session-u1")

	// User 2: Does not complete
	_ = evaluator.StartSession("session-u2", "pod-u2", "user-2", "Multi-User Lab")

	// User 3: Completes the lab
	_ = evaluator.StartSession("session-u3", "pod-u3", "user-3", "Multi-User Lab")
	event3 := &events.VMEvent{
		PodID:     "pod-u3",
		VMName:    "vm",
		EventType: events.EventTypeSyscheck,
		Data:      json.RawMessage(`{"path": "/done", "event": "added"}`),
	}
	_, _ = evaluator.EvaluateEvent(ctx, event3, "session-u3")

	// Check progress for each user
	prog1, _ := evaluator.GetSessionProgress("session-u1")
	prog2, _ := evaluator.GetSessionProgress("session-u2")
	prog3, _ := evaluator.GetSessionProgress("session-u3")

	t.Logf("User 1: %d/%d points", prog1.EarnedPoints, prog1.MaxPoints)
	t.Logf("User 2: %d/%d points", prog2.EarnedPoints, prog2.MaxPoints)
	t.Logf("User 3: %d/%d points", prog3.EarnedPoints, prog3.MaxPoints)

	// User 1 and 3 should have 100 points, User 2 should have 0
	if prog1.EarnedPoints != 100 {
		t.Errorf("User 1: expected 100 points, got %d", prog1.EarnedPoints)
	}
	if prog2.EarnedPoints != 0 {
		t.Errorf("User 2: expected 0 points, got %d", prog2.EarnedPoints)
	}
	if prog3.EarnedPoints != 100 {
		t.Errorf("User 3: expected 100 points, got %d", prog3.EarnedPoints)
	}
}

// sessionRepoListAdapter adapts FakeSessionRepository for the achievements service
type sessionRepoListAdapter struct {
	*mocks.FakeSessionRepository
	sessions []*models.Session
}

func (a *sessionRepoListAdapter) List(ctx context.Context, filter repositories.SessionFilter) ([]*models.Session, error) {
	if filter.UserID != "" {
		var result []*models.Session
		for _, s := range a.sessions {
			if s.UserID == filter.UserID {
				result = append(result, s)
			}
		}
		return result, nil
	}
	return a.sessions, nil
}
