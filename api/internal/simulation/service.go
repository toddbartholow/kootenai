// Package simulation provides test student simulation for pathway progression
package simulation

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/google/uuid"

	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// SimulationConfig configures a simulation run
type SimulationConfig struct {
	// StudentProfile determines performance characteristics
	StudentProfile StudentProfile `json:"studentProfile"`
	// Speed controls simulation pacing (0 = instant, otherwise milliseconds between steps)
	Speed int `json:"speed"`
	// ModulesToComplete limits how many modules to complete (0 = all)
	ModulesToComplete int `json:"modulesToComplete"`
}

// StudentProfile defines performance characteristics for simulated students
type StudentProfile string

const (
	ProfileExcellent  StudentProfile = "excellent"  // 95-100% scores
	ProfileGood       StudentProfile = "good"       // 80-94% scores
	ProfileAverage    StudentProfile = "average"    // 65-79% scores
	ProfileStruggling StudentProfile = "struggling" // 50-70% scores, may fail
)

// SimulationResult contains the results of a simulation run
type SimulationResult struct {
	StudentID        string              `json:"studentId"`
	StudentName      string              `json:"studentName"`
	PathwayID        string              `json:"pathwayId"`
	PathwayName      string              `json:"pathwayName"`
	EnrollmentID     string              `json:"enrollmentId"`
	Status           string              `json:"status"`
	ModulesCompleted int                 `json:"modulesCompleted"`
	TotalModules     int                 `json:"totalModules"`
	EarnedPoints     int                 `json:"earnedPoints"`
	MaxPoints        int                 `json:"maxPoints"`
	Percentage       float64             `json:"percentage"`
	Achievements     []AchievementResult `json:"achievements"`
	LabResults       []LabResult         `json:"labResults"`
	Duration         time.Duration       `json:"duration"`
}

// AchievementResult represents an achievement earned during simulation
type AchievementResult struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Tier   string `json:"tier"`
	Points int    `json:"points"`
}

// LabResult represents a lab completion during simulation
type LabResult struct {
	ModuleName  string    `json:"moduleName"`
	LabName     string    `json:"labName"`
	Score       int       `json:"score"`
	MaxPoints   int       `json:"maxPoints"`
	Percentage  float64   `json:"percentage"`
	Passed      bool      `json:"passed"`
	CompletedAt time.Time `json:"completedAt"`
}

// Service provides simulation capabilities
type Service struct {
	db              *sql.DB
	enrollmentRepo  repositories.EnrollmentRepository
	pathwayRepo     *repositories.PathwayRepo
	achievementRepo *repositories.AchievementRepo
	logger          *slog.Logger
}

// NewService creates a new simulation service
func NewService(
	db *sql.DB,
	enrollmentRepo repositories.EnrollmentRepository,
	pathwayRepo *repositories.PathwayRepo,
	achievementRepo *repositories.AchievementRepo,
	logger *slog.Logger,
) *Service {
	return &Service{
		db:              db,
		enrollmentRepo:  enrollmentRepo,
		pathwayRepo:     pathwayRepo,
		achievementRepo: achievementRepo,
		logger:          logger,
	}
}

// CreateTestStudent creates a new test student user
func (s *Service) CreateTestStudent(ctx context.Context, name, email string) (*models.User, error) {
	id := uuid.New().String()
	// Generate a unique external_id and username for the test student
	externalID := fmt.Sprintf("test-student-%s", id[:8])
	username := fmt.Sprintf("test_%d", time.Now().UnixNano())

	query := `
		INSERT INTO users (id, external_id, username, email, display_name, role, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 'student', true, NOW(), NOW())
		RETURNING id, email, display_name, role, is_active, created_at, updated_at`

	var user models.User
	err := s.db.QueryRowContext(ctx, query, id, externalID, username, email, name).Scan(
		&user.ID, &user.Email, &user.DisplayName, &user.Role, &user.IsActive,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("creating test student: %w", err)
	}

	s.logger.Info("Created test student", "id", user.ID, "name", name, "email", email)
	return &user, nil
}

// ListTestStudents lists all test students (emails containing "test" or "simulation")
func (s *Service) ListTestStudents(ctx context.Context) ([]*models.User, error) {
	query := `
		SELECT id, email, display_name, role, is_active, created_at, updated_at
		FROM users
		WHERE email LIKE '%test%' OR email LIKE '%simulation%' OR display_name LIKE '%Test%'
		ORDER BY created_at DESC`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing test students: %w", err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		var user models.User
		if err := rows.Scan(
			&user.ID, &user.Email, &user.DisplayName, &user.Role, &user.IsActive,
			&user.CreatedAt, &user.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning user: %w", err)
		}
		users = append(users, &user)
	}

	return users, rows.Err()
}

// SimulatePathwayProgression simulates a student progressing through a pathway
func (s *Service) SimulatePathwayProgression(
	ctx context.Context,
	userID, pathwayID string,
	config SimulationConfig,
) (*SimulationResult, error) {
	startTime := time.Now()

	// Get pathway details
	pathway, err := s.pathwayRepo.GetByID(ctx, pathwayID)
	if err != nil {
		return nil, fmt.Errorf("getting pathway: %w", err)
	}
	if pathway == nil {
		return nil, fmt.Errorf("pathway not found: %s", pathwayID)
	}

	// Get user details
	var userName string
	err = s.db.QueryRowContext(ctx, "SELECT display_name FROM users WHERE id = $1", userID).Scan(&userName)
	if err != nil {
		return nil, fmt.Errorf("getting user: %w", err)
	}

	// Check if already enrolled, if not enroll
	enrollment, err := s.enrollmentRepo.GetByUserAndPathway(ctx, userID, pathwayID)
	if err != nil {
		return nil, fmt.Errorf("checking enrollment: %w", err)
	}

	if enrollment == nil {
		// Create new enrollment
		enrollment = &models.PathwayEnrollment{
			ID:        uuid.New().String(),
			UserID:    userID,
			PathwayID: pathwayID,
			Status:    models.EnrollmentStatusEnrolled,
		}

		if err := s.enrollmentRepo.Create(ctx, enrollment); err != nil {
			return nil, fmt.Errorf("creating enrollment: %w", err)
		}

		if err := s.enrollmentRepo.InitializeModuleProgress(ctx, enrollment.ID, pathwayID); err != nil {
			return nil, fmt.Errorf("initializing progress: %w", err)
		}

		// Refresh enrollment
		enrollment, err = s.enrollmentRepo.GetByID(ctx, enrollment.ID)
		if err != nil {
			return nil, fmt.Errorf("refreshing enrollment: %w", err)
		}
	}

	s.logger.Info("Starting simulation",
		"user", userName,
		"pathway", pathway.Name,
		"profile", config.StudentProfile,
	)

	result := &SimulationResult{
		StudentID:    userID,
		StudentName:  userName,
		PathwayID:    pathwayID,
		PathwayName:  pathway.Name,
		EnrollmentID: enrollment.ID,
		Achievements: []AchievementResult{},
		LabResults:   []LabResult{},
	}

	// Get modules with their labs
	modules, err := s.pathwayRepo.ListModules(ctx, pathwayID)
	if err != nil {
		return nil, fmt.Errorf("listing modules: %w", err)
	}

	modulesToComplete := len(modules)
	if config.ModulesToComplete > 0 && config.ModulesToComplete < modulesToComplete {
		modulesToComplete = config.ModulesToComplete
	}

	// Progress through each module
	for i, module := range modules {
		if i >= modulesToComplete {
			break
		}

		// Unlock module if needed
		if err := s.enrollmentRepo.UnlockModule(ctx, enrollment.ID, module.ID); err != nil {
			s.logger.Warn("Failed to unlock module", "module", module.Name, "error", err)
		}

		// Get labs for this module
		labs, err := s.pathwayRepo.ListModuleLabs(ctx, module.ID)
		if err != nil {
			return nil, fmt.Errorf("listing labs for module %s: %w", module.Name, err)
		}

		s.logger.Info("Processing module", "module", module.Name, "labCount", len(labs))

		for _, lab := range labs {
			// Simulate lab completion
			labResult, err := s.simulateLabCompletion(ctx, enrollment.ID, module, lab, config)
			if err != nil {
				return nil, fmt.Errorf("simulating lab %s: %w", lab.LabName, err)
			}
			result.LabResults = append(result.LabResults, *labResult)

			// Check for achievements
			newAchievements, err := s.checkAndAwardAchievements(ctx, userID)
			if err != nil {
				s.logger.Warn("Failed to check achievements", "error", err)
			}
			result.Achievements = append(result.Achievements, newAchievements...)

			// Apply speed delay (capped at 5 seconds to prevent goroutine starvation)
			if config.Speed > 0 {
				delay := config.Speed
				if delay > 5000 {
					delay = 5000
				}
				time.Sleep(time.Duration(delay) * time.Millisecond)
			}
		}

		// Recalculate module progress
		if err := s.enrollmentRepo.RecalculateModuleProgress(ctx, enrollment.ID, module.ID); err != nil {
			s.logger.Warn("Failed to recalculate module progress", "error", err)
		}

		// Recalculate enrollment progress
		if err := s.enrollmentRepo.RecalculateEnrollmentProgress(ctx, enrollment.ID); err != nil {
			s.logger.Warn("Failed to recalculate enrollment progress", "error", err)
		}
	}

	// Get final enrollment state
	finalEnrollment, err := s.enrollmentRepo.GetByID(ctx, enrollment.ID)
	if err != nil {
		return nil, fmt.Errorf("getting final enrollment state: %w", err)
	}

	result.Status = string(finalEnrollment.Status)
	result.ModulesCompleted = finalEnrollment.CompletedModules
	result.TotalModules = finalEnrollment.TotalModules
	result.EarnedPoints = finalEnrollment.EarnedPoints
	result.MaxPoints = finalEnrollment.MaxPoints
	result.Percentage = finalEnrollment.Percentage
	result.Duration = time.Since(startTime)

	s.logger.Info("Simulation complete",
		"user", userName,
		"pathway", pathway.Name,
		"status", result.Status,
		"percentage", result.Percentage,
		"achievements", len(result.Achievements),
		"duration", result.Duration,
	)

	return result, nil
}

// simulateLabCompletion simulates completing a single lab
func (s *Service) simulateLabCompletion(
	ctx context.Context,
	enrollmentID string,
	module *models.PathwayModule,
	lab *models.ModuleLab,
	config SimulationConfig,
) (*LabResult, error) {
	// Calculate score based on profile
	score, maxPoints := s.calculateScore(lab.LabMaxPoints, config.StudentProfile)
	percentage := float64(score) / float64(maxPoints) * 100
	passed := percentage >= 70 // Default pass threshold

	// For simulations, we directly update lab progress without creating actual sessions
	// (sessions require pods, which are real lab environments)
	if err := s.recordSimulatedLabProgress(ctx, enrollmentID, module.ID, lab.LabTemplateID, score, maxPoints, passed); err != nil {
		return nil, fmt.Errorf("recording simulated lab progress: %w", err)
	}

	s.logger.Info("Completed lab",
		"lab", lab.LabName,
		"score", score,
		"maxPoints", maxPoints,
		"passed", passed,
	)

	return &LabResult{
		ModuleName:  module.Name,
		LabName:     lab.LabName,
		Score:       score,
		MaxPoints:   maxPoints,
		Percentage:  percentage,
		Passed:      passed,
		CompletedAt: time.Now(),
	}, nil
}

// calculateScore calculates a score based on student profile
func (s *Service) calculateScore(maxPoints int, profile StudentProfile) (score, max int) {
	var minPct, maxPct float64

	switch profile {
	case ProfileExcellent:
		minPct, maxPct = 0.95, 1.0
	case ProfileGood:
		minPct, maxPct = 0.80, 0.94
	case ProfileAverage:
		minPct, maxPct = 0.65, 0.79
	case ProfileStruggling:
		minPct, maxPct = 0.50, 0.70
	default:
		minPct, maxPct = 0.70, 0.85
	}

	// Random percentage within the range
	// #nosec G404 -- math/rand is acceptable here as this is for generating simulated
	// student scores for demo/testing purposes, not for any security-sensitive operation.
	pct := minPct + rand.Float64()*(maxPct-minPct)
	score = int(float64(maxPoints) * pct)

	return score, maxPoints
}

// recordSimulatedLabProgress directly updates lab_progress without creating sessions
// This is used for simulations where we don't have real pods/sessions
func (s *Service) recordSimulatedLabProgress(ctx context.Context, enrollmentID, moduleID, labTemplateID string, score, maxPoints int, passed bool) error {
	now := time.Now()

	// Upsert lab progress directly (without session reference)
	query := `
		INSERT INTO lab_progress (
			id, enrollment_id, module_id, lab_template_id,
			attempt_count, best_score, max_points,
			passed, first_attempt_at, completed_at
		)
		VALUES (
			uuid_generate_v4(),
			$1, $2, $3,
			1, $4, $5,
			$6, $7::timestamptz, CASE WHEN $6 THEN $7::timestamptz ELSE NULL END
		)
		ON CONFLICT (enrollment_id, lab_template_id)
		DO UPDATE SET
			attempt_count = lab_progress.attempt_count + 1,
			best_score = GREATEST(lab_progress.best_score, $4),
			passed = lab_progress.passed OR $6,
			first_attempt_at = COALESCE(lab_progress.first_attempt_at, $7::timestamptz),
			completed_at = CASE
				WHEN $6 AND lab_progress.completed_at IS NULL THEN $7::timestamptz
				ELSE lab_progress.completed_at
			END`

	_, err := s.db.ExecContext(ctx, query,
		enrollmentID, moduleID, labTemplateID, score, maxPoints, passed, now)
	if err != nil {
		return fmt.Errorf("upserting lab progress: %w", err)
	}

	// Update enrollment last activity
	_, err = s.db.ExecContext(ctx,
		`UPDATE pathway_enrollments SET last_activity_at = $2, started_at = COALESCE(started_at, $2) WHERE id = $1`,
		enrollmentID, now)
	if err != nil {
		return fmt.Errorf("updating enrollment activity: %w", err)
	}

	return nil
}

// checkAndAwardAchievements checks for new achievements
func (s *Service) checkAndAwardAchievements(ctx context.Context, userID string) ([]AchievementResult, error) {
	var results []AchievementResult

	// Get all achievements the user doesn't have yet
	query := `
		SELECT a.id, a.name, a.tier, a.points, a.criteria, a.type
		FROM achievements a
		WHERE a.is_active = true
		AND NOT EXISTS (
			SELECT 1 FROM user_achievements ua
			WHERE ua.user_id = $1 AND ua.achievement_id = a.id
		)`

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type achievementCheck struct {
		ID       string
		Name     string
		Tier     string
		Points   int
		Criteria []byte
		Type     string
	}

	var toCheck []achievementCheck
	for rows.Next() {
		var ac achievementCheck
		if err := rows.Scan(&ac.ID, &ac.Name, &ac.Tier, &ac.Points, &ac.Criteria, &ac.Type); err != nil {
			return nil, err
		}
		toCheck = append(toCheck, ac)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating achievements: %w", err)
	}

	// Check each achievement
	for _, ac := range toCheck {
		earned, err := s.checkAchievementCriteria(ctx, userID, ac.Type, ac.Criteria)
		if err != nil {
			s.logger.Warn("Failed to check achievement", "achievement", ac.Name, "error", err)
			continue
		}

		if earned {
			// Award the achievement
			if err := s.awardAchievement(ctx, userID, ac.ID); err != nil {
				s.logger.Warn("Failed to award achievement", "achievement", ac.Name, "error", err)
				continue
			}

			results = append(results, AchievementResult{
				ID:     ac.ID,
				Name:   ac.Name,
				Tier:   ac.Tier,
				Points: ac.Points,
			})

			s.logger.Info("Achievement earned", "user", userID, "achievement", ac.Name)
		}
	}

	return results, nil
}

// checkAchievementCriteria checks if a user meets achievement criteria
func (s *Service) checkAchievementCriteria(ctx context.Context, userID, achievementType string, criteria []byte) (bool, error) {
	switch achievementType {
	case "lab_completion":
		// Check how many labs completed (using lab_progress via enrollments)
		var count int
		err := s.db.QueryRowContext(ctx,
			`SELECT COUNT(DISTINCT lp.lab_template_id)
			 FROM lab_progress lp
			 JOIN pathway_enrollments pe ON pe.id = lp.enrollment_id
			 WHERE pe.user_id = $1 AND lp.passed = true`,
			userID,
		).Scan(&count)
		if err != nil {
			return false, err
		}
		return count >= 1, nil

	case "milestone":
		// Check total labs completed
		var count int
		err := s.db.QueryRowContext(ctx,
			`SELECT COUNT(DISTINCT lp.lab_template_id)
			 FROM lab_progress lp
			 JOIN pathway_enrollments pe ON pe.id = lp.enrollment_id
			 WHERE pe.user_id = $1 AND lp.passed = true`,
			userID,
		).Scan(&count)
		if err != nil {
			return false, err
		}
		// Different milestones require different counts
		// For now, check against common thresholds
		return count >= 5 || count >= 10 || count >= 25, nil

	case "perfect_score":
		// Check if any lab has 100% score
		var count int
		err := s.db.QueryRowContext(ctx,
			`SELECT COUNT(*)
			 FROM lab_progress lp
			 JOIN pathway_enrollments pe ON pe.id = lp.enrollment_id
			 WHERE pe.user_id = $1 AND lp.best_score >= lp.max_points AND lp.max_points > 0`,
			userID,
		).Scan(&count)
		if err != nil {
			return false, err
		}
		return count >= 1, nil

	case "pathway":
		// Check pathway module completion
		var completedModules int
		err := s.db.QueryRowContext(ctx,
			`SELECT COALESCE(MAX(completed_modules), 0) FROM pathway_enrollments WHERE user_id = $1`,
			userID,
		).Scan(&completedModules)
		if err != nil {
			return false, err
		}
		// Different pathway achievements require different module counts
		return completedModules >= 2 || completedModules >= 6 || completedModules >= 8, nil

	case "pathway_completion":
		// Check if pathway is fully completed
		var count int
		err := s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM pathway_enrollments WHERE user_id = $1 AND status = 'completed'`,
			userID,
		).Scan(&count)
		if err != nil {
			return false, err
		}
		return count >= 1, nil

	default:
		return false, nil
	}
}

// awardAchievement awards an achievement to a user
func (s *Service) awardAchievement(ctx context.Context, userID, achievementID string) error {
	query := `
		INSERT INTO user_achievements (id, user_id, achievement_id, earned_at, notified)
		VALUES ($1, $2, $3, NOW(), false)
		ON CONFLICT (user_id, achievement_id) DO NOTHING`

	_, err := s.db.ExecContext(ctx, query, uuid.New().String(), userID, achievementID)
	return err
}

// ResetStudentProgress resets all progress for a student (for re-running simulations)
func (s *Service) ResetStudentProgress(ctx context.Context, userID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Delete user achievements
	if _, err := tx.ExecContext(ctx, "DELETE FROM user_achievements WHERE user_id = $1", userID); err != nil {
		return fmt.Errorf("deleting achievements: %w", err)
	}

	// Get enrollment IDs
	rows, err := tx.QueryContext(ctx, "SELECT id FROM pathway_enrollments WHERE user_id = $1", userID)
	if err != nil {
		return fmt.Errorf("querying enrollments: %w", err)
	}

	var enrollmentIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		enrollmentIDs = append(enrollmentIDs, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterating enrollments: %w", err)
	}
	rows.Close()

	// Delete lab progress, module progress, and enrollments
	for _, eid := range enrollmentIDs {
		if _, err := tx.ExecContext(ctx, "DELETE FROM lab_progress WHERE enrollment_id = $1", eid); err != nil {
			return fmt.Errorf("deleting lab progress: %w", err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM module_progress WHERE enrollment_id = $1", eid); err != nil {
			return fmt.Errorf("deleting module progress: %w", err)
		}
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM pathway_enrollments WHERE user_id = $1", userID); err != nil {
		return fmt.Errorf("deleting enrollments: %w", err)
	}

	s.logger.Info("Reset student progress", "userID", userID)

	return tx.Commit()
}

// GetStudentProgress gets comprehensive progress for a student
func (s *Service) GetStudentProgress(ctx context.Context, userID string) (map[string]any, error) {
	result := make(map[string]any)

	// Get user info
	var userName, email string
	err := s.db.QueryRowContext(ctx,
		"SELECT display_name, email FROM users WHERE id = $1",
		userID,
	).Scan(&userName, &email)
	if err != nil {
		return nil, fmt.Errorf("getting user: %w", err)
	}
	result["user"] = map[string]string{"id": userID, "name": userName, "email": email}

	// Get enrollments with progress
	enrollments, err := s.enrollmentRepo.List(ctx, models.EnrollmentListOptions{UserID: userID})
	if err != nil {
		return nil, fmt.Errorf("listing enrollments: %w", err)
	}

	var enrichedEnrollments []map[string]any
	for _, e := range enrollments {
		pathway, _ := s.pathwayRepo.GetByID(ctx, e.PathwayID)
		pathwayName := ""
		if pathway != nil {
			pathwayName = pathway.Name
		}

		enrichedEnrollments = append(enrichedEnrollments, map[string]any{
			"id":               e.ID,
			"pathwayId":        e.PathwayID,
			"pathwayName":      pathwayName,
			"status":           e.Status,
			"completedModules": e.CompletedModules,
			"totalModules":     e.TotalModules,
			"earnedPoints":     e.EarnedPoints,
			"maxPoints":        e.MaxPoints,
			"percentage":       e.Percentage,
			"enrolledAt":       e.EnrolledAt,
			"completedAt":      e.CompletedAt,
		})
	}
	result["enrollments"] = enrichedEnrollments

	// Get achievements
	var achievements []map[string]any
	rows, err := s.db.QueryContext(ctx,
		`SELECT a.id, a.name, a.tier, a.points, ua.earned_at
		 FROM user_achievements ua
		 JOIN achievements a ON a.id = ua.achievement_id
		 WHERE ua.user_id = $1
		 ORDER BY ua.earned_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying achievements: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id, name, tier string
		var points int
		var earnedAt time.Time
		if err := rows.Scan(&id, &name, &tier, &points, &earnedAt); err != nil {
			return nil, err
		}
		achievements = append(achievements, map[string]any{
			"id":       id,
			"name":     name,
			"tier":     tier,
			"points":   points,
			"earnedAt": earnedAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating achievements: %w", err)
	}
	result["achievements"] = achievements

	// Get lab progress summary (labs attempted and passed)
	var totalLabs, passedLabs int
	var totalPoints, maxTotalPoints int
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*), COUNT(*) FILTER (WHERE lp.passed), COALESCE(SUM(lp.best_score), 0), COALESCE(SUM(lp.max_points), 0)
		 FROM lab_progress lp
		 JOIN pathway_enrollments pe ON pe.id = lp.enrollment_id
		 WHERE pe.user_id = $1 AND lp.attempt_count > 0`,
		userID,
	).Scan(&totalLabs, &passedLabs, &totalPoints, &maxTotalPoints)
	if err != nil {
		return nil, err
	}
	result["labSummary"] = map[string]any{
		"totalLabs":   totalLabs,
		"passedLabs":  passedLabs,
		"totalPoints": totalPoints,
		"maxPoints":   maxTotalPoints,
	}

	return result, nil
}
