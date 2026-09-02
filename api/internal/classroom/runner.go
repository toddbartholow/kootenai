package classroom

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// ClassroomRunner orchestrates a classroom simulation, running one goroutine per student.
type ClassroomRunner struct {
	repo        Repository
	service     *Service
	llm         *AnthropicClient
	canvas      *CanvasClient
	labExec     *LabExecutor
	apiClient   *LabAPIClient
	pathwayRepo *repositories.PathwayRepo
	behavior    *BehaviorEngine
	scheduler   *Scheduler
	logger      *slog.Logger

	mu              sync.Mutex
	cancelFns       map[string]context.CancelFunc // simulationID → cancel
	stoppedStatuses map[string]string             // simulationID → status set by Stop/Pause
}

// NewClassroomRunner creates a new runner.
func NewClassroomRunner(
	repo Repository,
	service *Service,
	llm *AnthropicClient,
	canvas *CanvasClient,
	labExec *LabExecutor,
	logger *slog.Logger,
	opts ...RunnerOption,
) *ClassroomRunner {
	r := &ClassroomRunner{
		repo:            repo,
		service:         service,
		llm:             llm,
		canvas:          canvas,
		labExec:         labExec,
		behavior:        NewBehaviorEngine(),
		scheduler:       NewScheduler(),
		logger:          logger,
		cancelFns:       make(map[string]context.CancelFunc),
		stoppedStatuses: make(map[string]string),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// RunnerOption configures a ClassroomRunner.
type RunnerOption func(*ClassroomRunner)

// WithAPIClient sets the lab API client for pod/session operations.
func WithAPIClient(client *LabAPIClient) RunnerOption {
	return func(r *ClassroomRunner) {
		r.apiClient = client
	}
}

// WithPathwayRepo sets the pathway repository for fetching pathway modules and labs.
func WithPathwayRepo(repo *repositories.PathwayRepo) RunnerOption {
	return func(r *ClassroomRunner) {
		r.pathwayRepo = repo
	}
}

// Start begins running a simulation. Each AI student gets its own goroutine.
func (r *ClassroomRunner) Start(ctx context.Context, simulationID string) error {
	sim, err := r.repo.GetSimulation(ctx, simulationID)
	if err != nil {
		return fmt.Errorf("loading simulation: %w", err)
	}
	if sim == nil {
		return fmt.Errorf("simulation not found: %s", simulationID)
	}
	if sim.Status != "pending" && sim.Status != "paused" {
		return fmt.Errorf("simulation is %s, cannot start", sim.Status)
	}

	if err := r.repo.UpdateSimulationStatus(ctx, simulationID, "running"); err != nil {
		return fmt.Errorf("updating status to running: %w", err)
	}

	// Parse config for speed multiplier
	var cfg SimulationConfig
	if err := json.Unmarshal(sim.Config, &cfg); err != nil {
		r.logger.Warn("Failed to parse simulation config, using defaults", "error", err, "id", simulationID)
	}
	if cfg.SpeedMultiplier <= 0 {
		cfg.SpeedMultiplier = 1.0
	}
	if cfg.MaxConcurrentPods <= 0 {
		cfg.MaxConcurrentPods = 10
	}

	// Use background context — the HTTP request context dies when the response is sent,
	// but student goroutines must continue running after the handler returns.
	runCtx, cancel := context.WithCancel(context.Background())
	r.mu.Lock()
	r.cancelFns[simulationID] = cancel
	delete(r.stoppedStatuses, simulationID)
	r.mu.Unlock()

	// Create pod semaphore to limit concurrent pods
	podSemaphore := make(chan struct{}, cfg.MaxConcurrentPods)

	var wg sync.WaitGroup

	for i := range sim.Students {
		student := sim.Students[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.runStudent(runCtx, sim, &student, cfg, podSemaphore)
		}()
	}

	// Wait for all students in a separate goroutine, then mark complete
	go func() {
		wg.Wait()

		r.mu.Lock()
		delete(r.cancelFns, simulationID)
		// Check if Stop/Pause already set the final status
		stoppedStatus, wasStopped := r.stoppedStatuses[simulationID]
		delete(r.stoppedStatuses, simulationID)
		r.mu.Unlock()

		// If Stop or Pause already wrote the status, don't overwrite it
		if wasStopped {
			r.logger.Info("Simulation finished", "id", simulationID, "status", stoppedStatus)
			return
		}

		finalStatus := "completed"
		// Bound the terminal DB write — runCtx is already cancelled at this
		// point, so we need a fresh context, but a stuck DB must not hang the
		// goroutine forever.
		statusCtx, statusCancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := r.repo.UpdateSimulationStatus(statusCtx, simulationID, finalStatus); err != nil {
			r.logger.Error("Failed to update simulation status", "error", err, "id", simulationID)
		}
		statusCancel()

		r.logger.Info("Simulation finished", "id", simulationID, "status", finalStatus)
	}()

	return nil
}

// Stop cancels a running simulation.
func (r *ClassroomRunner) Stop(simulationID string) error {
	r.mu.Lock()
	cancel, ok := r.cancelFns[simulationID]
	if ok {
		r.stoppedStatuses[simulationID] = "completed"
	}
	r.mu.Unlock()

	if !ok {
		return fmt.Errorf("simulation %s is not running", simulationID)
	}

	cancel()

	ctx, cancelCtx := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelCtx()
	if err := r.repo.UpdateSimulationStatus(ctx, simulationID, "completed"); err != nil {
		return fmt.Errorf("updating status: %w", err)
	}

	return nil
}

// Pause stops a simulation but marks it as paused (can resume later).
func (r *ClassroomRunner) Pause(simulationID string) error {
	r.mu.Lock()
	cancel, ok := r.cancelFns[simulationID]
	if ok {
		r.stoppedStatuses[simulationID] = "paused"
	}
	r.mu.Unlock()

	if !ok {
		return fmt.Errorf("simulation %s is not running", simulationID)
	}

	cancel()

	ctx, cancelCtx := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelCtx()
	if err := r.repo.UpdateSimulationStatus(ctx, simulationID, "paused"); err != nil {
		return fmt.Errorf("updating status: %w", err)
	}

	return nil
}

// IsRunning checks if a simulation is currently running.
func (r *ClassroomRunner) IsRunning(simulationID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.cancelFns[simulationID]
	return ok
}

// runStudent runs the activity loop for a single AI student.
func (r *ClassroomRunner) runStudent(ctx context.Context, sim *Simulation, student *AIStudent, cfg SimulationConfig, podSemaphore chan struct{}) {
	profiles := DefaultProfiles()
	profile, ok := profiles[student.Personality]
	if !ok {
		r.logger.Error("Unknown personality", "student", student.Name, "personality", student.Personality)
		return
	}

	state := r.behavior.InitialState(student.Personality)

	r.logger.Info("Student starting simulation",
		"student", student.Name,
		"personality", student.Personality,
		"simulationId", sim.ID,
	)

	// Log start activity
	if err := r.repo.CreateActivity(ctx, &Activity{
		SimulationID: sim.ID,
		StudentID:    student.ID,
		ActivityType: "lab_start",
		Status:       "completed",
	}); err != nil {
		r.logger.Error("Failed to record start activity", "error", err, "student", student.Name)
	}

	// If pathway repo and API client are available, run real labs
	if r.pathwayRepo != nil && r.apiClient != nil && sim.PathwayID != nil && *sim.PathwayID != "" {
		r.runStudentPathway(ctx, sim, student, profile, &state, cfg, podSemaphore)
	} else {
		// Fallback: placeholder activities
		r.runStudentPlaceholder(ctx, sim, student, profile, &state, cfg)
	}

	r.logger.Info("Student completed simulation", "student", student.Name)
}

// runStudentPathway iterates through pathway modules and labs, executing each one.
func (r *ClassroomRunner) runStudentPathway(
	ctx context.Context,
	sim *Simulation,
	student *AIStudent,
	profile PersonalityProfile,
	state *BehaviorState,
	cfg SimulationConfig,
	podSemaphore chan struct{},
) {
	pathway, err := r.pathwayRepo.GetWithModules(ctx, *sim.PathwayID)
	if err != nil {
		r.logger.Error("Failed to fetch pathway", "error", err, "pathwayId", *sim.PathwayID)
		return
	}
	if pathway == nil {
		r.logger.Error("Pathway not found", "pathwayId", *sim.PathwayID)
		return
	}

	// Sort modules by display order
	modules := pathway.Modules
	sort.Slice(modules, func(i, j int) bool {
		return modules[i].DisplayOrder < modules[j].DisplayOrder
	})

	var labSummaries []LabSummary

	for _, module := range modules {
		select {
		case <-ctx.Done():
			return
		default:
		}

		labs, err := r.pathwayRepo.ListModuleLabs(ctx, module.ID)
		if err != nil {
			r.logger.Error("Failed to list module labs", "error", err, "moduleId", module.ID)
			continue
		}

		// Sort labs by display order
		sort.Slice(labs, func(i, j int) bool {
			return labs[i].DisplayOrder < labs[j].DisplayOrder
		})

		for _, lab := range labs {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Check energy/skip logic
			if r.behavior.ShouldSkipActivity(profile.Traits, *state) {
				r.logger.Info("Student skipping lab", "student", student.Name, "lab", lab.LabName, "energy", state.Energy)
				*state = r.behavior.UpdateAfterActivity(*state, profile.Traits)
				continue
			}

			// Personality-driven delay
			delay := r.scheduler.ActivityDelay(profile, *state, cfg.SpeedMultiplier)
			select {
			case <-ctx.Done():
				return
			case <-sleepContext(ctx, delay):
			}

			// Execute the lab
			summary := r.executeLab(ctx, sim, student, profile, lab, podSemaphore)
			labSummaries = append(labSummaries, summary)

			// Update state
			*state = r.behavior.UpdateAfterActivity(*state, profile.Traits)
			stateJSON := StateToJSON(*state)
			if err := r.repo.UpdateStudentState(ctx, student.ID, stateJSON); err != nil {
				r.logger.Error("Failed to update student state", "error", err, "student", student.ID)
			}
		}
	}

	// Generate pathway-level feedback
	if r.llm != nil && len(labSummaries) > 0 {
		pwFeedback, err := r.llm.GeneratePathwayFeedback(ctx, profile, pathway.Name, labSummaries)
		if err != nil {
			r.logger.Warn("Failed to generate pathway feedback", "error", err, "student", student.Name)
		} else {
			detailsJSON, _ := json.Marshal(pwFeedback)
			if err := r.repo.CreateFeedback(ctx, &Feedback{
				SimulationID: sim.ID,
				StudentID:    student.ID,
				PathwayID:    sim.PathwayID,
				FeedbackType: "pathway",
				Summary:      pwFeedback.Summary,
				Rating:       &pwFeedback.Rating,
				Details:      detailsJSON,
			}); err != nil {
				r.logger.Error("Failed to record pathway feedback", "error", err, "student", student.Name)
			}
		}
	}
}

// executeLab handles the full lifecycle: create pod, run lab, collect feedback, cleanup.
func (r *ClassroomRunner) executeLab(
	ctx context.Context,
	sim *Simulation,
	student *AIStudent,
	profile PersonalityProfile,
	lab *models.ModuleLab,
	podSemaphore chan struct{},
) LabSummary {
	summary := LabSummary{
		LabName: lab.LabName,
	}

	// Log lab start
	if err := r.repo.CreateActivity(ctx, &Activity{
		SimulationID: sim.ID,
		StudentID:    student.ID,
		ActivityType: "lab_start",
		TargetID:     &lab.LabTemplateID,
		TargetName:   &lab.LabName,
		Status:       "started",
	}); err != nil {
		r.logger.Error("Failed to record lab start activity", "error", err, "lab", lab.LabName, "student", student.Name)
	}

	// Acquire pod semaphore
	select {
	case podSemaphore <- struct{}{}:
	case <-ctx.Done():
		return summary
	}
	defer func() { <-podSemaphore }()

	// Create pod
	podID, err := r.apiClient.CreatePod(ctx, lab.LabTemplateID, student.ID)
	if err != nil {
		r.storeInfraFeedback(ctx, sim, student, lab, "pod_creation_failed", err.Error())
		r.logger.Warn("Failed to create pod", "error", err, "lab", lab.LabName, "student", student.Name)
		return summary
	}

	// Ensure pod cleanup
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if deleteErr := r.apiClient.DeletePod(cleanupCtx, podID); deleteErr != nil {
			r.logger.Warn("Failed to delete pod", "error", deleteErr, "podId", podID)
		}
	}()

	// Wait for pod to be ready
	_, err = r.apiClient.WaitForPodReady(ctx, podID, 5*time.Minute)
	if err != nil {
		r.storeInfraFeedback(ctx, sim, student, lab, "pod_timeout", err.Error())
		r.logger.Warn("Pod not ready", "error", err, "podId", podID, "lab", lab.LabName)
		return summary
	}

	// Create session
	sessionID, err := r.apiClient.CreateSession(ctx, podID, student.ID, lab.LabTemplateID)
	if err != nil {
		r.storeInfraFeedback(ctx, sim, student, lab, "session_creation_failed", err.Error())
		r.logger.Warn("Failed to create session", "error", err, "lab", lab.LabName)
		return summary
	}

	// Execute lab via labtest
	var execResult *ExecuteLabResult
	if r.labExec != nil {
		execResult, err = r.labExec.ExecuteLab(ctx, lab.LabTemplateID, podID, "student")
		if err != nil {
			r.logger.Warn("Lab execution returned error", "error", err, "lab", lab.LabName)
			// execResult may still have partial data
		}
	}
	if execResult == nil {
		execResult = &ExecuteLabResult{
			LabName: lab.LabName,
			PodID:   podID,
		}
	}

	// Fetch session progress for checkpoint data
	progress, err := r.apiClient.GetSessionProgress(ctx, sessionID)
	if err != nil {
		r.logger.Warn("Failed to get session progress", "error", err, "sessionId", sessionID)
	} else {
		execResult.CheckpointsTotal = len(progress.Checkpoints)
		passed := 0
		for _, cp := range progress.Checkpoints {
			if cp.Status == "passed" {
				passed++
			} else {
				execResult.FailedCheckpoints = append(execResult.FailedCheckpoints, cp.Name)
			}
		}
		execResult.CheckpointsPassed = passed
	}

	// Build summary
	summary.Success = execResult.Success
	summary.DurationMs = execResult.DurationMs
	summary.CheckpointsPassed = execResult.CheckpointsPassed
	summary.CheckpointsTotal = execResult.CheckpointsTotal

	// Record activity
	score := 0
	if execResult.CheckpointsTotal > 0 {
		score = (execResult.CheckpointsPassed * 100) / execResult.CheckpointsTotal
	}
	if err := r.repo.CreateActivity(ctx, &Activity{
		SimulationID: sim.ID,
		StudentID:    student.ID,
		ActivityType: "lab_complete",
		TargetID:     &lab.LabTemplateID,
		TargetName:   &lab.LabName,
		Status:       "completed",
		Score:        intPtr(score),
		MaxScore:     intPtr(100),
	}); err != nil {
		r.logger.Error("Failed to record lab complete activity", "error", err, "lab", lab.LabName, "student", student.Name)
	}

	// Generate lab feedback via LLM
	if r.llm != nil {
		var checkpointDescs []string
		if progress != nil {
			for _, cp := range progress.Checkpoints {
				checkpointDescs = append(checkpointDescs, cp.Name)
			}
		}

		labFeedback, err := r.llm.GenerateLabFeedback(ctx, profile, lab.LabName, lab.LabDescription, execResult, checkpointDescs, nil)
		if err != nil {
			r.logger.Warn("Failed to generate lab feedback", "error", err, "lab", lab.LabName)
		} else {
			detailsJSON, _ := json.Marshal(labFeedback)
			durationMs := int(execResult.DurationMs)
			if err := r.repo.CreateFeedback(ctx, &Feedback{
				SimulationID:        sim.ID,
				StudentID:           student.ID,
				LabTemplateID:       &lab.LabTemplateID,
				PathwayID:           sim.PathwayID,
				SessionID:           &sessionID,
				FeedbackType:        "lab_quality",
				Summary:             labFeedback.Summary,
				Rating:              &labFeedback.Rating,
				Details:             detailsJSON,
				ExecutionDurationMs: &durationMs,
				CheckpointsPassed:   &execResult.CheckpointsPassed,
				CheckpointsTotal:    &execResult.CheckpointsTotal,
				Score:               intPtr(score),
				MaxScore:            intPtr(100),
				ExecutionLog:        &execResult.Output,
			}); err != nil {
				r.logger.Error("Failed to record lab feedback", "error", err, "lab", lab.LabName, "student", student.Name)
			}
			summary.Rating = labFeedback.Rating
		}
	}

	r.logger.Info("Lab completed",
		"student", student.Name,
		"lab", lab.LabName,
		"checkpoints", fmt.Sprintf("%d/%d", execResult.CheckpointsPassed, execResult.CheckpointsTotal),
		"durationMs", execResult.DurationMs,
	)

	return summary
}

// storeInfraFeedback stores infrastructure-type feedback when something goes wrong.
func (r *ClassroomRunner) storeInfraFeedback(ctx context.Context, sim *Simulation, student *AIStudent, lab *models.ModuleLab, issueType, errorMsg string) {
	details, _ := json.Marshal(map[string]string{
		"issueType":    issueType,
		"errorMessage": errorMsg,
		"labName":      lab.LabName,
	})
	if err := r.repo.CreateFeedback(ctx, &Feedback{
		SimulationID:  sim.ID,
		StudentID:     student.ID,
		LabTemplateID: &lab.LabTemplateID,
		PathwayID:     sim.PathwayID,
		FeedbackType:  "infrastructure",
		Summary:       fmt.Sprintf("%s: %s", issueType, errorMsg),
		Details:       details,
	}); err != nil {
		r.logger.Error("Failed to record infra feedback", "error", err, "student", student.Name, "issueType", issueType)
	}
}

// runStudentPlaceholder runs the original placeholder activity loop when no pathway is configured.
func (r *ClassroomRunner) runStudentPlaceholder(ctx context.Context, sim *Simulation, student *AIStudent, profile PersonalityProfile, state *BehaviorState, cfg SimulationConfig) {
	for activityNum := 0; activityNum < 5; activityNum++ {
		select {
		case <-ctx.Done():
			r.logger.Info("Student stopped", "student", student.Name)
			return
		default:
		}

		if r.behavior.ShouldSkipActivity(profile.Traits, *state) {
			r.logger.Info("Student skipping activity", "student", student.Name, "energy", state.Energy)
			continue
		}

		delay := r.scheduler.ActivityDelay(profile, *state, cfg.SpeedMultiplier)
		select {
		case <-ctx.Done():
			return
		case <-sleepContext(ctx, delay):
		}

		quality := r.behavior.QualityModifier(profile.Traits, *state)
		score := r.behavior.ScoreForQuiz(profile.BehavioralConfig, quality)

		if err := r.repo.CreateActivity(ctx, &Activity{
			SimulationID: sim.ID,
			StudentID:    student.ID,
			ActivityType: "lab_complete",
			TargetName:   strPtr(fmt.Sprintf("Activity %d", activityNum+1)),
			Status:       "completed",
			Score:        intPtr(score),
			MaxScore:     intPtr(100),
		}); err != nil {
			r.logger.Error("Failed to record placeholder activity", "error", err, "student", student.Name)
		}

		*state = r.behavior.UpdateAfterActivity(*state, profile.Traits)
		stateJSON := StateToJSON(*state)
		if err := r.repo.UpdateStudentState(ctx, student.ID, stateJSON); err != nil {
			r.logger.Error("Failed to update student state", "error", err, "student", student.ID)
		}

		r.logger.Info("Student completed activity",
			"student", student.Name,
			"activity", activityNum+1,
			"score", score,
			"energy", state.Energy,
			"stress", state.Stress,
		)
	}
}

// sleepContext sleeps for the given duration or until context is canceled.
func sleepContext(ctx context.Context, d time.Duration) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		defer close(ch)
		select {
		case <-ctx.Done():
		case <-time.After(d):
		}
	}()
	return ch
}

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }
