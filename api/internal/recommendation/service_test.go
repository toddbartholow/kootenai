package recommendation

import (
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestNewService(t *testing.T) {
	logger := testLogger()
	svc := NewService(nil, nil, nil, nil, logger)

	assert.NotNil(t, svc)
	assert.Equal(t, logger, svc.logger)
}

func TestService_GetRecommendations_NoRepos(t *testing.T) {
	logger := testLogger()
	svc := &Service{
		enrollmentRepo:  nil,
		pathwayRepo:     nil,
		labTemplateRepo: nil,
		sessionRepo:     nil,
		logger:          logger,
	}

	// Should not error, just return empty
	result, err := svc.GetRecommendations(t.Context(), "user-123", 5)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.Labs)
	assert.Empty(t, result.Pathways)
}

func TestService_GetRecommendations_DefaultLimit(t *testing.T) {
	logger := testLogger()
	svc := &Service{logger: logger}

	// Test with 0 limit - should use default of 5
	result, err := svc.GetRecommendations(t.Context(), "user-123", 0)

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestService_GetNextLabInPathway_NoRepos(t *testing.T) {
	svc := &Service{
		enrollmentRepo: nil,
		pathwayRepo:    nil,
	}

	result, err := svc.GetNextLabInPathway(t.Context(), "user-123", "pathway-456")

	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestRecommendationType_Values(t *testing.T) {
	assert.Equal(t, RecommendationType("next_in_pathway"), TypeNextInPathway)
	assert.Equal(t, RecommendationType("continue_progress"), TypeContinueProgress)
	assert.Equal(t, RecommendationType("new_pathway"), TypeNewPathway)
	assert.Equal(t, RecommendationType("similar_difficulty"), TypeSimilarDifficulty)
	assert.Equal(t, RecommendationType("popular"), TypePopular)
}

func TestLabRecommendation_Structure(t *testing.T) {
	rec := LabRecommendation{
		LabTemplateID:   "lab-123",
		LabName:         "Introduction to Linux",
		LabSlug:         "intro-to-linux",
		LabDescription:  "Learn Linux basics",
		Difficulty:      "beginner",
		DurationMinutes: 30,
		MaxPoints:       100,
		Type:            TypeNextInPathway,
		Reason:          "Continue your progress",
		Priority:        1,
		PathwayID:       "pathway-456",
		PathwayName:     "Linux Fundamentals",
		ModuleID:        "module-789",
		ModuleName:      "Getting Started",
	}

	assert.Equal(t, "lab-123", rec.LabTemplateID)
	assert.Equal(t, "Introduction to Linux", rec.LabName)
	assert.Equal(t, TypeNextInPathway, rec.Type)
	assert.Equal(t, 1, rec.Priority)
}

func TestPathwayRecommendation_Structure(t *testing.T) {
	rec := PathwayRecommendation{
		PathwayID:      "pathway-123",
		PathwayName:    "Cybersecurity Basics",
		PathwaySlug:    "cybersecurity-basics",
		Description:    "Learn the fundamentals of cybersecurity",
		Difficulty:     "beginner",
		EstimatedHours: 10,
		ModuleCount:    4,
		LabCount:       12,
		Type:           TypeNewPathway,
		Reason:         "Expand your skills",
		Priority:       5,
		CoverImageURL:  "https://example.com/cover.jpg",
		Icon:           "shield",
	}

	assert.Equal(t, "pathway-123", rec.PathwayID)
	assert.Equal(t, "Cybersecurity Basics", rec.PathwayName)
	assert.Equal(t, TypeNewPathway, rec.Type)
}

func TestRecommendationsResponse_Structure(t *testing.T) {
	response := RecommendationsResponse{
		Labs: []LabRecommendation{
			{LabTemplateID: "lab-1", Priority: 1},
			{LabTemplateID: "lab-2", Priority: 2},
		},
		Pathways: []PathwayRecommendation{
			{PathwayID: "pathway-1", Priority: 3},
		},
	}

	assert.Len(t, response.Labs, 2)
	assert.Len(t, response.Pathways, 1)
}
