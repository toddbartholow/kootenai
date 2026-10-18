package classroom

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
)

// Service provides classroom simulation business logic.
type Service struct {
	repo   Repository
	logger *slog.Logger
}

// NewService creates a new classroom service.
func NewService(repo Repository, logger *slog.Logger) *Service {
	return &Service{
		repo:   repo,
		logger: logger,
	}
}

// CreateSimulationRequest holds parameters for creating a new simulation.
type CreateSimulationRequest struct {
	Name           string           `json:"name"`
	PathwayID      *string          `json:"pathwayId,omitempty"`
	CanvasCourseID *string          `json:"canvasCourseId,omitempty"`
	Config         SimulationConfig `json:"config"`
}

// CreateSimulation creates a new simulation and auto-generates the AI student roster.
func (s *Service) CreateSimulation(ctx context.Context, req CreateSimulationRequest) (*Simulation, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("simulation name is required")
	}

	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		return nil, fmt.Errorf("marshaling config: %w", err)
	}

	sim := &Simulation{
		Name:           req.Name,
		Status:         "pending",
		PathwayID:      req.PathwayID,
		CanvasCourseID: req.CanvasCourseID,
		Config:         configJSON,
	}

	if err := s.repo.CreateSimulation(ctx, sim); err != nil {
		return nil, fmt.Errorf("creating simulation: %w", err)
	}

	// Auto-generate students based on personality mix
	students, err := s.generateStudentRoster(ctx, sim.ID, req.Config)
	if err != nil {
		return nil, fmt.Errorf("generating student roster: %w", err)
	}
	sim.Students = students

	s.logger.Info("Created classroom simulation",
		"id", sim.ID,
		"name", sim.Name,
		"studentCount", len(students),
	)

	return sim, nil
}

// GetSimulation returns a simulation by ID with students loaded.
func (s *Service) GetSimulation(ctx context.Context, id string) (*Simulation, error) {
	sim, err := s.repo.GetSimulation(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting simulation: %w", err)
	}
	return sim, nil
}

// ListSimulations returns all simulations.
func (s *Service) ListSimulations(ctx context.Context) ([]Simulation, error) {
	sims, err := s.repo.ListSimulations(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing simulations: %w", err)
	}
	return sims, nil
}

// DeleteSimulation deletes a simulation and all associated data (cascading).
func (s *Service) DeleteSimulation(ctx context.Context, id string) error {
	return s.repo.DeleteSimulation(ctx, id)
}

// GetStudent returns a single AI student by ID.
func (s *Service) GetStudent(ctx context.Context, id string) (*AIStudent, error) {
	return s.repo.GetStudent(ctx, id)
}

// ListActivities returns paginated activities for a simulation.
func (s *Service) ListActivities(ctx context.Context, simulationID string, limit, offset int) ([]Activity, int, error) {
	return s.repo.ListActivities(ctx, simulationID, limit, offset)
}

// ListFeedback returns paginated feedback for a simulation.
func (s *Service) ListFeedback(ctx context.Context, simID, feedbackType, labID string, limit, offset int) ([]Feedback, int, error) {
	return s.repo.ListFeedback(ctx, simID, FeedbackFilter{
		FeedbackType:  feedbackType,
		LabTemplateID: labID,
		Limit:         limit,
		Offset:        offset,
	})
}

// GetFeedbackSummary returns aggregated feedback stats for a simulation.
func (s *Service) GetFeedbackSummary(ctx context.Context, simID string) (*FeedbackSummary, error) {
	return s.repo.GetFeedbackSummary(ctx, simID)
}

// generateStudentRoster creates AI students based on the personality mix config.
func (s *Service) generateStudentRoster(ctx context.Context, simulationID string, cfg SimulationConfig) ([]AIStudent, error) {
	profiles := DefaultProfiles()

	// If no personality mix specified, distribute evenly
	mix := cfg.PersonalityMix
	if len(mix) == 0 {
		total := cfg.StudentCount
		if total <= 0 {
			total = 25
		}
		each := total / 3
		remainder := total - each*3
		mix = map[PersonalityType]int{
			PersonalityHighPerformer:        each + remainder,
			PersonalityStruggling:           each,
			PersonalityIndustryProfessional: each,
		}
	}

	var students []AIStudent
	for personality, count := range mix {
		profile, ok := profiles[personality]
		if !ok {
			return nil, fmt.Errorf("unknown personality type: %s", personality)
		}

		for i := 0; i < count; i++ {
			name := generateStudentName()

			traitsJSON, _ := json.Marshal(profile.Traits)
			skillsJSON, _ := json.Marshal(profile.TechSkills)
			behaviorJSON, _ := json.Marshal(profile.BehavioralConfig)

			student := &AIStudent{
				SimulationID:     simulationID,
				Name:             name,
				Personality:      personality,
				Traits:           traitsJSON,
				TechSkills:       skillsJSON,
				BehavioralConfig: behaviorJSON,
				State:            json.RawMessage(`{}`),
			}

			if err := s.repo.CreateStudent(ctx, student); err != nil {
				return nil, fmt.Errorf("creating student %s: %w", name, err)
			}
			students = append(students, *student)
		}
	}

	return students, nil
}

// Student name generation pools.
var (
	firstNames = []string{
		"Alex", "Jordan", "Taylor", "Casey", "Morgan", "Riley", "Quinn", "Avery",
		"Parker", "Dakota", "Skyler", "Cameron", "Drew", "Reese", "Sage", "Blake",
		"Hayden", "Rowan", "Emery", "Finley", "Charlie", "Harper", "Kendall", "Logan",
		"Peyton", "River", "Spencer", "Devon", "Jamie", "Kai",
	}
	lastNames = []string{
		"Chen", "Patel", "Williams", "Garcia", "Kim", "Nguyen", "Martinez", "Johnson",
		"Lee", "Brown", "Davis", "Wilson", "Anderson", "Thomas", "Moore", "Jackson",
		"White", "Harris", "Clark", "Lewis", "Robinson", "Walker", "Young", "Allen",
		"King", "Scott", "Adams", "Hill", "Green", "Baker",
	}
)

// generateStudentName creates a random student name.
// #nosec G404 -- math/rand is acceptable for generating non-security-sensitive simulated names.
func generateStudentName() string {
	first := firstNames[rand.Intn(len(firstNames))]
	last := lastNames[rand.Intn(len(lastNames))]
	return fmt.Sprintf("%s %s", first, last)
}
