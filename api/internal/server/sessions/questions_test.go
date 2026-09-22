package sessions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/server/httputil"
)

// -----------------------------------------------------------------------------
// validateAnswer Tests
// -----------------------------------------------------------------------------

func TestValidateTextAnswer(t *testing.T) {
	tests := []struct {
		name        string
		question    *models.Question
		answer      string
		wantCorrect bool
		wantMsg     string
	}{
		{
			name: "exact match - correct",
			question: &models.Question{
				Type: models.QuestionTypeText,
				Validation: &models.QuestionValidation{
					Type:          models.ValidationTypeExact,
					Answer:        "192.168.1.1",
					CaseSensitive: true,
				},
			},
			answer:      "192.168.1.1",
			wantCorrect: true,
			wantMsg:     "",
		},
		{
			name: "exact match - incorrect",
			question: &models.Question{
				Type: models.QuestionTypeText,
				Validation: &models.QuestionValidation{
					Type:          models.ValidationTypeExact,
					Answer:        "192.168.1.1",
					CaseSensitive: true,
				},
			},
			answer:      "192.168.1.2",
			wantCorrect: false,
			wantMsg:     "Incorrect answer. Please try again.",
		},
		{
			name: "exact match - case insensitive correct",
			question: &models.Question{
				Type: models.QuestionTypeText,
				Validation: &models.QuestionValidation{
					Type:          models.ValidationTypeExact,
					Answer:        "ADMIN",
					CaseSensitive: false,
				},
			},
			answer:      "admin",
			wantCorrect: true,
			wantMsg:     "",
		},
		{
			name: "exact match - case sensitive incorrect",
			question: &models.Question{
				Type: models.QuestionTypeText,
				Validation: &models.QuestionValidation{
					Type:          models.ValidationTypeExact,
					Answer:        "ADMIN",
					CaseSensitive: true,
				},
			},
			answer:      "admin",
			wantCorrect: false,
			wantMsg:     "Incorrect answer. Please try again.",
		},
		{
			name: "exact match - whitespace trimmed",
			question: &models.Question{
				Type: models.QuestionTypeText,
				Validation: &models.QuestionValidation{
					Type:          models.ValidationTypeExact,
					Answer:        "hello",
					CaseSensitive: true,
				},
			},
			answer:      "  hello  ",
			wantCorrect: true,
			wantMsg:     "",
		},
		{
			name: "regex match - correct IP pattern",
			question: &models.Question{
				Type: models.QuestionTypeText,
				Validation: &models.QuestionValidation{
					Type:    models.ValidationTypeRegex,
					Pattern: `^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$`,
				},
			},
			answer:      "192.168.1.100",
			wantCorrect: true,
			wantMsg:     "",
		},
		{
			name: "regex match - incorrect format",
			question: &models.Question{
				Type: models.QuestionTypeText,
				Validation: &models.QuestionValidation{
					Type:    models.ValidationTypeRegex,
					Pattern: `^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$`,
				},
			},
			answer:      "not-an-ip",
			wantCorrect: false,
			wantMsg:     "Answer format is incorrect. Please try again.",
		},
		{
			name: "nil validation",
			question: &models.Question{
				Type:       models.QuestionTypeText,
				Validation: nil,
			},
			answer:      "any",
			wantCorrect: false,
			wantMsg:     "Question not properly configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			correct, msg := validateTextAnswer(tt.question, tt.answer)
			if correct != tt.wantCorrect {
				t.Errorf("validateTextAnswer() correct = %v, want %v", correct, tt.wantCorrect)
			}
			if msg != tt.wantMsg {
				t.Errorf("validateTextAnswer() msg = %q, want %q", msg, tt.wantMsg)
			}
		})
	}
}

func TestValidateMultipleChoiceAnswer(t *testing.T) {
	tests := []struct {
		name        string
		question    *models.Question
		selected    []string
		wantCorrect bool
		wantMsg     string
	}{
		{
			name: "single select - correct",
			question: &models.Question{
				Type:        models.QuestionTypeMultipleChoice,
				MultiSelect: false,
				Options: []models.QuestionOption{
					{ID: "a", Text: "Option A", Correct: true},
					{ID: "b", Text: "Option B", Correct: false},
					{ID: "c", Text: "Option C", Correct: false},
				},
			},
			selected:    []string{"a"},
			wantCorrect: true,
			wantMsg:     "",
		},
		{
			name: "single select - incorrect",
			question: &models.Question{
				Type:        models.QuestionTypeMultipleChoice,
				MultiSelect: false,
				Options: []models.QuestionOption{
					{ID: "a", Text: "Option A", Correct: true},
					{ID: "b", Text: "Option B", Correct: false},
				},
			},
			selected:    []string{"b"},
			wantCorrect: false,
			wantMsg:     "Incorrect answer. Please try again.",
		},
		{
			name: "single select - too many selected",
			question: &models.Question{
				Type:        models.QuestionTypeMultipleChoice,
				MultiSelect: false,
				Options: []models.QuestionOption{
					{ID: "a", Text: "Option A", Correct: true},
					{ID: "b", Text: "Option B", Correct: false},
				},
			},
			selected:    []string{"a", "b"},
			wantCorrect: false,
			wantMsg:     "Please select exactly one answer.",
		},
		{
			name: "single select - none selected",
			question: &models.Question{
				Type:        models.QuestionTypeMultipleChoice,
				MultiSelect: false,
				Options: []models.QuestionOption{
					{ID: "a", Text: "Option A", Correct: true},
				},
			},
			selected:    []string{},
			wantCorrect: false,
			wantMsg:     "Please select exactly one answer.",
		},
		{
			name: "multi select - all correct",
			question: &models.Question{
				Type:        models.QuestionTypeMultipleChoice,
				MultiSelect: true,
				Options: []models.QuestionOption{
					{ID: "a", Text: "Option A", Correct: true},
					{ID: "b", Text: "Option B", Correct: true},
					{ID: "c", Text: "Option C", Correct: false},
				},
			},
			selected:    []string{"a", "b"},
			wantCorrect: true,
			wantMsg:     "",
		},
		{
			name: "multi select - missing one",
			question: &models.Question{
				Type:        models.QuestionTypeMultipleChoice,
				MultiSelect: true,
				Options: []models.QuestionOption{
					{ID: "a", Text: "Option A", Correct: true},
					{ID: "b", Text: "Option B", Correct: true},
					{ID: "c", Text: "Option C", Correct: false},
				},
			},
			selected:    []string{"a"},
			wantCorrect: false,
			wantMsg:     "Incorrect selection. Please try again.",
		},
		{
			name: "multi select - wrong option included",
			question: &models.Question{
				Type:        models.QuestionTypeMultipleChoice,
				MultiSelect: true,
				Options: []models.QuestionOption{
					{ID: "a", Text: "Option A", Correct: true},
					{ID: "b", Text: "Option B", Correct: true},
					{ID: "c", Text: "Option C", Correct: false},
				},
			},
			selected:    []string{"a", "c"},
			wantCorrect: false,
			wantMsg:     "Incorrect selection. Please try again.",
		},
		{
			name: "no options configured",
			question: &models.Question{
				Type:        models.QuestionTypeMultipleChoice,
				MultiSelect: false,
				Options:     []models.QuestionOption{},
			},
			selected:    []string{"a"},
			wantCorrect: false,
			wantMsg:     "Question not properly configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			correct, msg := validateMultipleChoiceAnswer(tt.question, tt.selected)
			if correct != tt.wantCorrect {
				t.Errorf("validateMultipleChoiceAnswer() correct = %v, want %v", correct, tt.wantCorrect)
			}
			if msg != tt.wantMsg {
				t.Errorf("validateMultipleChoiceAnswer() msg = %q, want %q", msg, tt.wantMsg)
			}
		})
	}
}

func TestValidateAnswer(t *testing.T) {
	tests := []struct {
		name        string
		question    *models.Question
		request     *models.SubmitAnswerRequest
		wantCorrect bool
	}{
		{
			name: "text question - correct",
			question: &models.Question{
				Type: models.QuestionTypeText,
				Validation: &models.QuestionValidation{
					Type:   models.ValidationTypeExact,
					Answer: "correct",
				},
			},
			request: &models.SubmitAnswerRequest{
				ResponseText: "correct",
			},
			wantCorrect: true,
		},
		{
			name: "multiple choice question - correct",
			question: &models.Question{
				Type:        models.QuestionTypeMultipleChoice,
				MultiSelect: false,
				Options: []models.QuestionOption{
					{ID: "a", Correct: true},
					{ID: "b", Correct: false},
				},
			},
			request: &models.SubmitAnswerRequest{
				SelectedOptions: []string{"a"},
			},
			wantCorrect: true,
		},
		{
			name: "unknown question type",
			question: &models.Question{
				Type: "unknown",
			},
			request:     &models.SubmitAnswerRequest{},
			wantCorrect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			correct, _ := validateAnswer(tt.question, tt.request)
			if correct != tt.wantCorrect {
				t.Errorf("validateAnswer() correct = %v, want %v", correct, tt.wantCorrect)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// areDependenciesMet Tests
// -----------------------------------------------------------------------------

func TestAreDependenciesMet(t *testing.T) {
	tests := []struct {
		name              string
		deps              []string
		questionResponses map[string]*models.QuestionResponse
		checkpoints       map[string]bool
		want              bool
	}{
		{
			name:              "no dependencies",
			deps:              []string{},
			questionResponses: map[string]*models.QuestionResponse{},
			checkpoints:       map[string]bool{},
			want:              true,
		},
		{
			name: "question dependency met",
			deps: []string{"q1"},
			questionResponses: map[string]*models.QuestionResponse{
				"q1": {QuestionID: "q1", Status: models.QuestionStatusCorrect},
			},
			checkpoints: map[string]bool{},
			want:        true,
		},
		{
			name: "question dependency not met - incorrect",
			deps: []string{"q1"},
			questionResponses: map[string]*models.QuestionResponse{
				"q1": {QuestionID: "q1", Status: models.QuestionStatusIncorrect},
			},
			checkpoints: map[string]bool{},
			want:        false,
		},
		{
			name: "question dependency not met - pending",
			deps: []string{"q1"},
			questionResponses: map[string]*models.QuestionResponse{
				"q1": {QuestionID: "q1", Status: models.QuestionStatusPending},
			},
			checkpoints: map[string]bool{},
			want:        false,
		},
		{
			name:              "checkpoint dependency met",
			deps:              []string{"cp1"},
			questionResponses: map[string]*models.QuestionResponse{},
			checkpoints: map[string]bool{
				"cp1": true,
			},
			want: true,
		},
		{
			name:              "checkpoint dependency not met",
			deps:              []string{"cp1"},
			questionResponses: map[string]*models.QuestionResponse{},
			checkpoints: map[string]bool{
				"cp1": false,
			},
			want: false,
		},
		{
			name:              "checkpoint dependency missing",
			deps:              []string{"cp1"},
			questionResponses: map[string]*models.QuestionResponse{},
			checkpoints:       map[string]bool{},
			want:              false,
		},
		{
			name: "mixed dependencies - all met",
			deps: []string{"q1", "cp1"},
			questionResponses: map[string]*models.QuestionResponse{
				"q1": {QuestionID: "q1", Status: models.QuestionStatusCorrect},
			},
			checkpoints: map[string]bool{
				"cp1": true,
			},
			want: true,
		},
		{
			name: "mixed dependencies - question not met",
			deps: []string{"q1", "cp1"},
			questionResponses: map[string]*models.QuestionResponse{
				"q1": {QuestionID: "q1", Status: models.QuestionStatusIncorrect},
			},
			checkpoints: map[string]bool{
				"cp1": true,
			},
			want: false,
		},
		{
			name: "mixed dependencies - checkpoint not met",
			deps: []string{"q1", "cp1"},
			questionResponses: map[string]*models.QuestionResponse{
				"q1": {QuestionID: "q1", Status: models.QuestionStatusCorrect},
			},
			checkpoints: map[string]bool{
				"cp1": false,
			},
			want: false,
		},
		{
			name: "multiple questions - all met",
			deps: []string{"q1", "q2", "q3"},
			questionResponses: map[string]*models.QuestionResponse{
				"q1": {QuestionID: "q1", Status: models.QuestionStatusCorrect},
				"q2": {QuestionID: "q2", Status: models.QuestionStatusCorrect},
				"q3": {QuestionID: "q3", Status: models.QuestionStatusCorrect},
			},
			checkpoints: map[string]bool{},
			want:        true,
		},
		{
			name: "multiple questions - one not met",
			deps: []string{"q1", "q2", "q3"},
			questionResponses: map[string]*models.QuestionResponse{
				"q1": {QuestionID: "q1", Status: models.QuestionStatusCorrect},
				"q2": {QuestionID: "q2", Status: models.QuestionStatusIncorrect},
				"q3": {QuestionID: "q3", Status: models.QuestionStatusCorrect},
			},
			checkpoints: map[string]bool{},
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := areDependenciesMet(tt.deps, tt.questionResponses, tt.checkpoints)
			if got != tt.want {
				t.Errorf("areDependenciesMet() = %v, want %v", got, tt.want)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Edge Cases and Validation Tests
// -----------------------------------------------------------------------------

func TestValidateTextAnswerRegexEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		answer      string
		wantCorrect bool
	}{
		{
			name:        "email pattern - valid",
			pattern:     `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
			answer:      "test@example.com",
			wantCorrect: true,
		},
		{
			name:        "email pattern - invalid",
			pattern:     `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
			answer:      "not-an-email",
			wantCorrect: false,
		},
		{
			name:        "MAC address pattern - valid",
			pattern:     `^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$`,
			answer:      "00:1A:2B:3C:4D:5E",
			wantCorrect: true,
		},
		{
			name:        "port number pattern - valid",
			pattern:     `^([1-9][0-9]{0,3}|[1-5][0-9]{4}|6[0-4][0-9]{3}|65[0-4][0-9]{2}|655[0-2][0-9]|6553[0-5])$`,
			answer:      "443",
			wantCorrect: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			question := &models.Question{
				Type: models.QuestionTypeText,
				Validation: &models.QuestionValidation{
					Type:    models.ValidationTypeRegex,
					Pattern: tt.pattern,
				},
			}
			correct, _ := validateTextAnswer(question, tt.answer)
			if correct != tt.wantCorrect {
				t.Errorf("validateTextAnswer() with pattern %q and answer %q = %v, want %v",
					tt.pattern, tt.answer, correct, tt.wantCorrect)
			}
		})
	}
}

func TestValidateTextAnswerInvalidRegex(t *testing.T) {
	question := &models.Question{
		Type: models.QuestionTypeText,
		Validation: &models.QuestionValidation{
			Type:    models.ValidationTypeRegex,
			Pattern: "[invalid(regex", // Invalid regex pattern
		},
	}
	correct, msg := validateTextAnswer(question, "test")
	if correct {
		t.Error("validateTextAnswer() should fail with invalid regex")
	}
	if msg != "Invalid question configuration" {
		t.Errorf("validateTextAnswer() msg = %q, want %q", msg, "Invalid question configuration")
	}
}

func TestValidateTextAnswerUnknownValidationType(t *testing.T) {
	question := &models.Question{
		Type: models.QuestionTypeText,
		Validation: &models.QuestionValidation{
			Type: "unknown_type",
		},
	}
	correct, msg := validateTextAnswer(question, "test")
	if correct {
		t.Error("validateTextAnswer() should fail with unknown validation type")
	}
	if msg != "Unknown validation type" {
		t.Errorf("validateTextAnswer() msg = %q, want %q", msg, "Unknown validation type")
	}
}

// -----------------------------------------------------------------------------
// Mock Repositories for Question Handler Tests
// -----------------------------------------------------------------------------

type mockQuestionResponseRepo struct {
	responses []*models.QuestionResponse
	createErr error
	updateErr error
}

func (r *mockQuestionResponseRepo) Create(ctx context.Context, response *models.QuestionResponse) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.responses = append(r.responses, response)
	return nil
}

func (r *mockQuestionResponseRepo) GetBySessionID(ctx context.Context, sessionID string) ([]*models.QuestionResponse, error) {
	var result []*models.QuestionResponse
	for _, resp := range r.responses {
		if resp.SessionID == sessionID {
			result = append(result, resp)
		}
	}
	return result, nil
}

func (r *mockQuestionResponseRepo) GetBySessionAndQuestion(ctx context.Context, sessionID, questionID string) (*models.QuestionResponse, error) {
	for _, resp := range r.responses {
		if resp.SessionID == sessionID && resp.QuestionID == questionID {
			return resp, nil
		}
	}
	return nil, nil
}

func (r *mockQuestionResponseRepo) Update(ctx context.Context, response *models.QuestionResponse) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	return nil
}

func (r *mockQuestionResponseRepo) MarkCorrect(ctx context.Context, sessionID, questionID string, earnedPoints int, feedback string) error {
	return nil
}

func (r *mockQuestionResponseRepo) MarkIncorrect(ctx context.Context, sessionID, questionID, feedback string) error {
	return nil
}

func (r *mockQuestionResponseRepo) ShowHintLevel(ctx context.Context, sessionID, questionID string, level, penalty int) error {
	return nil
}

func (r *mockQuestionResponseRepo) GetHintLevel(ctx context.Context, sessionID, questionID string) (level int, penalty int, err error) {
	for _, resp := range r.responses {
		if resp.SessionID == sessionID && resp.QuestionID == questionID {
			return resp.HintLevelShown, resp.HintPenaltyApplied, nil
		}
	}
	return 0, 0, nil
}

func (r *mockQuestionResponseRepo) GetSessionQuestionStats(ctx context.Context, sessionID string) (earned int, total int, err error) {
	return 0, 0, nil
}

func (r *mockQuestionResponseRepo) InitializeForSession(ctx context.Context, sessionID string, questions []models.Question) error {
	return nil
}

// mockSessionRepoForQuestions implements session repository for question handler tests
type mockSessionRepoForQuestions struct {
	sessions []*models.Session
	getErr   error
}

func (r *mockSessionRepoForQuestions) Create(ctx context.Context, session *models.Session) error {
	r.sessions = append(r.sessions, session)
	return nil
}

func (r *mockSessionRepoForQuestions) GetByID(ctx context.Context, id string) (*models.Session, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	for _, s := range r.sessions {
		if s.ID == id {
			return s, nil
		}
	}
	return nil, nil
}

func (r *mockSessionRepoForQuestions) GetByPodID(ctx context.Context, podID string) ([]*models.Session, error) {
	return nil, nil
}

func (r *mockSessionRepoForQuestions) GetActiveByUserID(ctx context.Context, userID string) ([]*models.Session, error) {
	return nil, nil
}

func (r *mockSessionRepoForQuestions) List(ctx context.Context, filter repositories.SessionFilter) ([]*models.Session, error) {
	return r.sessions, nil
}

func (r *mockSessionRepoForQuestions) Update(ctx context.Context, session *models.Session) error {
	return nil
}

func (r *mockSessionRepoForQuestions) End(ctx context.Context, id string) error {
	return nil
}

func (r *mockSessionRepoForQuestions) Delete(ctx context.Context, id string) error {
	return nil
}

func (r *mockSessionRepoForQuestions) UpdateGrade(ctx context.Context, id string, earnedPoints int, passed bool) error {
	return nil
}

func (r *mockSessionRepoForQuestions) MarkGradeSynced(ctx context.Context, id string, syncedAt time.Time) error {
	return nil
}

func (r *mockSessionRepoForQuestions) MarkGradeSyncFailed(ctx context.Context, id string, errMsg string) error {
	return nil
}

func (r *mockSessionRepoForQuestions) GetUserID(ctx context.Context, id string) (string, error) {
	for _, s := range r.sessions {
		if s.ID == id {
			return s.UserID, nil
		}
	}
	return "", nil
}

func (r *mockSessionRepoForQuestions) IsOwner(ctx context.Context, id, userID string) (bool, error) {
	for _, s := range r.sessions {
		if s.ID == id {
			return s.UserID == userID, nil
		}
	}
	return false, nil
}

func (r *mockSessionRepoForQuestions) GetOrganizationID(ctx context.Context, id string) (*string, error) {
	return nil, nil
}

func (r *mockSessionRepoForQuestions) ListAll(ctx context.Context) ([]*models.Session, error) {
	return r.sessions, nil
}

func (r *mockSessionRepoForQuestions) EndStaleSessions(ctx context.Context, maxAge time.Duration) (int64, error) {
	return 0, nil
}

func (r *mockSessionRepoForQuestions) DeleteEndedBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return 0, nil
}

func (r *mockSessionRepoForQuestions) GetUserStats(ctx context.Context, userID string) (*repositories.UserSessionStats, error) {
	return nil, nil
}

func (r *mockSessionRepoForQuestions) ListWithLabNames(ctx context.Context, filter repositories.SessionFilter) ([]*repositories.SessionWithLabName, error) {
	return nil, nil
}

func (r *mockSessionRepoForQuestions) CountCompletedLabsByUser(ctx context.Context) (map[string]int, error) {
	return nil, nil
}

// mockLabTemplateRepoForQuestions implements lab template repository for question tests
type mockLabTemplateRepoForQuestions struct {
	templates map[string]*models.LabTemplateRecord
	getErr    error
}

func (r *mockLabTemplateRepoForQuestions) Create(ctx context.Context, record *models.LabTemplateRecord) error {
	return nil
}

func (r *mockLabTemplateRepoForQuestions) GetByID(ctx context.Context, id string) (*models.LabTemplateRecord, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	if r.templates != nil {
		return r.templates[id], nil
	}
	return nil, nil
}

func (r *mockLabTemplateRepoForQuestions) GetByIDs(ctx context.Context, ids []string) ([]*models.LabTemplateRecord, error) {
	return nil, nil
}

func (r *mockLabTemplateRepoForQuestions) GetByName(ctx context.Context, name string) (*models.LabTemplateRecord, error) {
	return nil, nil
}

func (r *mockLabTemplateRepoForQuestions) List(ctx context.Context, filter repositories.LabTemplateFilter) ([]*models.LabTemplateRecord, error) {
	return nil, nil
}

func (r *mockLabTemplateRepoForQuestions) Update(ctx context.Context, record *models.LabTemplateRecord) error {
	return nil
}

func (r *mockLabTemplateRepoForQuestions) Delete(ctx context.Context, id string) error {
	return nil
}

func (r *mockLabTemplateRepoForQuestions) SetActive(ctx context.Context, id string, active bool) error {
	return nil
}
func (r *mockLabTemplateRepoForQuestions) CreateVersion(_ context.Context, _ *models.LabTemplateVersion) error {
	return nil
}
func (r *mockLabTemplateRepoForQuestions) ListVersions(_ context.Context, _ string, _, _ int) ([]*models.LabTemplateVersion, error) {
	return nil, nil
}
func (r *mockLabTemplateRepoForQuestions) GetVersionByNumber(_ context.Context, _ string, _ int) (*models.LabTemplateVersion, error) {
	return nil, nil
}
func (r *mockLabTemplateRepoForQuestions) CountVersions(_ context.Context, _ string) (int, error) {
	return 0, nil
}

// mockCheckpointRepoForQuestions implements checkpoint repository for question tests
type mockCheckpointRepoForQuestions struct {
	progress []*models.CheckpointProgress
}

func (r *mockCheckpointRepoForQuestions) Create(ctx context.Context, progress *models.CheckpointProgress) error {
	r.progress = append(r.progress, progress)
	return nil
}

func (r *mockCheckpointRepoForQuestions) GetBySessionID(ctx context.Context, sessionID string) ([]*models.CheckpointProgress, error) {
	var result []*models.CheckpointProgress
	for _, p := range r.progress {
		if p.SessionID == sessionID {
			result = append(result, p)
		}
	}
	return result, nil
}

func (r *mockCheckpointRepoForQuestions) GetBySessionAndCheckpoint(ctx context.Context, sessionID, checkpointID string) (*models.CheckpointProgress, error) {
	return nil, nil
}

func (r *mockCheckpointRepoForQuestions) Update(ctx context.Context, progress *models.CheckpointProgress) error {
	return nil
}

func (r *mockCheckpointRepoForQuestions) MarkPassed(ctx context.Context, sessionID, checkpointID string, triggerEventID *string) error {
	return nil
}

func (r *mockCheckpointRepoForQuestions) MarkFailed(ctx context.Context, sessionID, checkpointID string) error {
	return nil
}

func (r *mockCheckpointRepoForQuestions) ResetForSession(ctx context.Context, sessionID string) error {
	return nil
}

// createTestManager creates a Manager for testing question handlers
func createTestManager(
	sessionRepo repositories.SessionRepository,
	labTemplateRepo repositories.LabTemplateRepository,
	checkpointRepo repositories.CheckpointProgressRepository,
	questionResponseRepo repositories.QuestionResponseRepository,
) *Manager {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewManager(Config{
		SessionRepo:          sessionRepo,
		LabTemplateRepo:      labTemplateRepo,
		CheckpointRepo:       checkpointRepo,
		QuestionResponseRepo: questionResponseRepo,
		Logger:               logger,
		Responder:            httputil.NewResponder(logger),
	})
}

// -----------------------------------------------------------------------------
// HTTP Handler Tests
// -----------------------------------------------------------------------------

func TestHandleGetSessionQuestions(t *testing.T) {
	// Create template spec with questions
	spec := models.LabSpec{
		Questions: []models.Question{
			{
				ID:          "q1",
				Description: "What is the IP address?",
				Type:        models.QuestionTypeText,
				Points:      10,
				Validation: &models.QuestionValidation{
					Type:   models.ValidationTypeExact,
					Answer: "192.168.1.1",
				},
			},
			{
				ID:          "q2",
				Description: "Select the correct option",
				Type:        models.QuestionTypeMultipleChoice,
				Points:      10,
				Options: []models.QuestionOption{
					{ID: "a", Text: "Option A", Correct: true},
					{ID: "b", Text: "Option B", Correct: false},
				},
			},
		},
	}
	specJSON, _ := json.Marshal(spec)

	t.Run("session not found", func(t *testing.T) {
		sessionRepo := &mockSessionRepoForQuestions{}
		labTemplateRepo := &mockLabTemplateRepoForQuestions{}
		checkpointRepo := &mockCheckpointRepoForQuestions{}
		questionResponseRepo := &mockQuestionResponseRepo{}

		mgr := createTestManager(sessionRepo, labTemplateRepo, checkpointRepo, questionResponseRepo)

		req := httptest.NewRequest(http.MethodGet, "/sessions/nonexistent/questions", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionID", "nonexistent")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		// Add user to context
		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user1", Email: "user@test.com"})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler := mgr.handleGetSessionQuestions()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("access denied for other user", func(t *testing.T) {
		sessionRepo := &mockSessionRepoForQuestions{
			sessions: []*models.Session{
				{ID: "session1", UserID: "owner1", LabTemplateID: "tmpl1"},
			},
		}
		labTemplateRepo := &mockLabTemplateRepoForQuestions{}
		checkpointRepo := &mockCheckpointRepoForQuestions{}
		questionResponseRepo := &mockQuestionResponseRepo{}

		mgr := createTestManager(sessionRepo, labTemplateRepo, checkpointRepo, questionResponseRepo)

		req := httptest.NewRequest(http.MethodGet, "/sessions/session1/questions", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionID", "session1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		// Add different user to context (not the owner)
		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "other-user", Email: "other@test.com"})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler := mgr.handleGetSessionQuestions()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
		}
	})

	t.Run("template not found", func(t *testing.T) {
		sessionRepo := &mockSessionRepoForQuestions{
			sessions: []*models.Session{
				{ID: "session1", UserID: "user1", LabTemplateID: "tmpl1"},
			},
		}
		labTemplateRepo := &mockLabTemplateRepoForQuestions{} // No templates
		checkpointRepo := &mockCheckpointRepoForQuestions{}
		questionResponseRepo := &mockQuestionResponseRepo{}

		mgr := createTestManager(sessionRepo, labTemplateRepo, checkpointRepo, questionResponseRepo)

		req := httptest.NewRequest(http.MethodGet, "/sessions/session1/questions", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionID", "session1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user1", Email: "user@test.com"})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler := mgr.handleGetSessionQuestions()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("no questions returns empty list", func(t *testing.T) {
		emptySpec := models.LabSpec{Questions: []models.Question{}}
		emptySpecJSON, _ := json.Marshal(emptySpec)

		sessionRepo := &mockSessionRepoForQuestions{
			sessions: []*models.Session{
				{ID: "session1", UserID: "user1", LabTemplateID: "tmpl1"},
			},
		}
		labTemplateRepo := &mockLabTemplateRepoForQuestions{
			templates: map[string]*models.LabTemplateRecord{
				"tmpl1": {ID: "tmpl1", Name: "Test Lab", Spec: emptySpecJSON},
			},
		}
		checkpointRepo := &mockCheckpointRepoForQuestions{}
		questionResponseRepo := &mockQuestionResponseRepo{}

		mgr := createTestManager(sessionRepo, labTemplateRepo, checkpointRepo, questionResponseRepo)

		req := httptest.NewRequest(http.MethodGet, "/sessions/session1/questions", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionID", "session1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user1", Email: "user@test.com"})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler := mgr.handleGetSessionQuestions()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var resp models.QuestionsListResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if len(resp.Questions) != 0 {
			t.Errorf("expected 0 questions, got %d", len(resp.Questions))
		}
	})

	t.Run("returns questions with progress", func(t *testing.T) {
		sessionRepo := &mockSessionRepoForQuestions{
			sessions: []*models.Session{
				{ID: "session1", UserID: "user1", LabTemplateID: "tmpl1"},
			},
		}
		labTemplateRepo := &mockLabTemplateRepoForQuestions{
			templates: map[string]*models.LabTemplateRecord{
				"tmpl1": {ID: "tmpl1", Name: "Test Lab", Spec: specJSON},
			},
		}
		checkpointRepo := &mockCheckpointRepoForQuestions{}
		questionResponseRepo := &mockQuestionResponseRepo{
			responses: []*models.QuestionResponse{
				{
					SessionID:    "session1",
					QuestionID:   "q1",
					Status:       models.QuestionStatusCorrect,
					EarnedPoints: 10,
					AttemptCount: 1,
				},
			},
		}

		mgr := createTestManager(sessionRepo, labTemplateRepo, checkpointRepo, questionResponseRepo)

		req := httptest.NewRequest(http.MethodGet, "/sessions/session1/questions", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionID", "session1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user1", Email: "user@test.com"})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler := mgr.handleGetSessionQuestions()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var resp models.QuestionsListResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if len(resp.Questions) != 2 {
			t.Errorf("expected 2 questions, got %d", len(resp.Questions))
		}

		if resp.TotalPoints != 20 {
			t.Errorf("expected total points 20, got %d", resp.TotalPoints)
		}

		if resp.EarnedPoints != 10 {
			t.Errorf("expected earned points 10, got %d", resp.EarnedPoints)
		}
	})
}

func TestHandleSubmitQuestionAnswer(t *testing.T) {
	// QuestionValidation.Answer now round-trips through JSON (fixed alongside
	// issue #121), so we can build the spec via json.Marshal of a LabSpec.
	spec := models.LabSpec{
		Questions: []models.Question{
			{
				ID:          "q1",
				Description: "What is the answer?",
				Type:        models.QuestionTypeText,
				Points:      10,
				Validation: &models.QuestionValidation{
					Type:   models.ValidationTypeExact,
					Answer: "correct",
				},
			},
		},
	}
	specJSON, _ := json.Marshal(spec)

	t.Run("session not found", func(t *testing.T) {
		sessionRepo := &mockSessionRepoForQuestions{}
		labTemplateRepo := &mockLabTemplateRepoForQuestions{}
		checkpointRepo := &mockCheckpointRepoForQuestions{}
		questionResponseRepo := &mockQuestionResponseRepo{}

		mgr := createTestManager(sessionRepo, labTemplateRepo, checkpointRepo, questionResponseRepo)

		body := `{"responseText": "answer"}`
		req := httptest.NewRequest(http.MethodPost, "/sessions/nonexistent/questions/q1/answer", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionID", "nonexistent")
		rctx.URLParams.Add("questionID", "q1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user1", Email: "user@test.com"})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler := mgr.handleSubmitQuestionAnswer()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("access denied for other user", func(t *testing.T) {
		sessionRepo := &mockSessionRepoForQuestions{
			sessions: []*models.Session{
				{ID: "session1", UserID: "owner1", LabTemplateID: "tmpl1"},
			},
		}
		labTemplateRepo := &mockLabTemplateRepoForQuestions{}
		checkpointRepo := &mockCheckpointRepoForQuestions{}
		questionResponseRepo := &mockQuestionResponseRepo{}

		mgr := createTestManager(sessionRepo, labTemplateRepo, checkpointRepo, questionResponseRepo)

		body := `{"responseText": "answer"}`
		req := httptest.NewRequest(http.MethodPost, "/sessions/session1/questions/q1/answer", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionID", "session1")
		rctx.URLParams.Add("questionID", "q1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		// Different user
		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "other-user", Email: "other@test.com"})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler := mgr.handleSubmitQuestionAnswer()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
		}
	})

	t.Run("invalid request body", func(t *testing.T) {
		sessionRepo := &mockSessionRepoForQuestions{
			sessions: []*models.Session{
				{ID: "session1", UserID: "user1", LabTemplateID: "tmpl1"},
			},
		}
		labTemplateRepo := &mockLabTemplateRepoForQuestions{}
		checkpointRepo := &mockCheckpointRepoForQuestions{}
		questionResponseRepo := &mockQuestionResponseRepo{}

		mgr := createTestManager(sessionRepo, labTemplateRepo, checkpointRepo, questionResponseRepo)

		body := `{invalid json}`
		req := httptest.NewRequest(http.MethodPost, "/sessions/session1/questions/q1/answer", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionID", "session1")
		rctx.URLParams.Add("questionID", "q1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user1", Email: "user@test.com"})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler := mgr.handleSubmitQuestionAnswer()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("question not found", func(t *testing.T) {
		sessionRepo := &mockSessionRepoForQuestions{
			sessions: []*models.Session{
				{ID: "session1", UserID: "user1", LabTemplateID: "tmpl1"},
			},
		}
		labTemplateRepo := &mockLabTemplateRepoForQuestions{
			templates: map[string]*models.LabTemplateRecord{
				"tmpl1": {ID: "tmpl1", Name: "Test Lab", Spec: specJSON},
			},
		}
		checkpointRepo := &mockCheckpointRepoForQuestions{}
		questionResponseRepo := &mockQuestionResponseRepo{}

		mgr := createTestManager(sessionRepo, labTemplateRepo, checkpointRepo, questionResponseRepo)

		body := `{"responseText": "answer"}`
		req := httptest.NewRequest(http.MethodPost, "/sessions/session1/questions/nonexistent/answer", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionID", "session1")
		rctx.URLParams.Add("questionID", "nonexistent")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user1", Email: "user@test.com"})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler := mgr.handleSubmitQuestionAnswer()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("session has ended", func(t *testing.T) {
		endedAt := time.Now()
		sessionRepo := &mockSessionRepoForQuestions{
			sessions: []*models.Session{
				{ID: "session1", UserID: "user1", LabTemplateID: "tmpl1", EndedAt: &endedAt},
			},
		}
		labTemplateRepo := &mockLabTemplateRepoForQuestions{}
		checkpointRepo := &mockCheckpointRepoForQuestions{}
		questionResponseRepo := &mockQuestionResponseRepo{}

		mgr := createTestManager(sessionRepo, labTemplateRepo, checkpointRepo, questionResponseRepo)

		body := `{"responseText": "answer"}`
		req := httptest.NewRequest(http.MethodPost, "/sessions/session1/questions/q1/answer", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionID", "session1")
		rctx.URLParams.Add("questionID", "q1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user1", Email: "user@test.com"})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler := mgr.handleSubmitQuestionAnswer()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("lab template not found", func(t *testing.T) {
		sessionRepo := &mockSessionRepoForQuestions{
			sessions: []*models.Session{
				{ID: "session1", UserID: "user1", LabTemplateID: "tmpl1"},
			},
		}
		labTemplateRepo := &mockLabTemplateRepoForQuestions{} // No templates
		checkpointRepo := &mockCheckpointRepoForQuestions{}
		questionResponseRepo := &mockQuestionResponseRepo{}

		mgr := createTestManager(sessionRepo, labTemplateRepo, checkpointRepo, questionResponseRepo)

		body := `{"responseText": "answer"}`
		req := httptest.NewRequest(http.MethodPost, "/sessions/session1/questions/q1/answer", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionID", "session1")
		rctx.URLParams.Add("questionID", "q1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user1", Email: "user@test.com"})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler := mgr.handleSubmitQuestionAnswer()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("failed to parse template spec", func(t *testing.T) {
		sessionRepo := &mockSessionRepoForQuestions{
			sessions: []*models.Session{
				{ID: "session1", UserID: "user1", LabTemplateID: "tmpl1"},
			},
		}
		labTemplateRepo := &mockLabTemplateRepoForQuestions{
			templates: map[string]*models.LabTemplateRecord{
				"tmpl1": {ID: "tmpl1", Name: "Test Lab", Spec: []byte(`{invalid json}`)},
			},
		}
		checkpointRepo := &mockCheckpointRepoForQuestions{}
		questionResponseRepo := &mockQuestionResponseRepo{}

		mgr := createTestManager(sessionRepo, labTemplateRepo, checkpointRepo, questionResponseRepo)

		body := `{"responseText": "answer"}`
		req := httptest.NewRequest(http.MethodPost, "/sessions/session1/questions/q1/answer", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionID", "session1")
		rctx.URLParams.Add("questionID", "q1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user1", Email: "user@test.com"})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler := mgr.handleSubmitQuestionAnswer()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})

	t.Run("prerequisites not met", func(t *testing.T) {
		// Template with question that has dependencies
		specWithDeps := []byte(`{
			"questions": [
				{
					"id": "q1",
					"description": "First question",
					"type": "text",
					"points": 10,
					"validation": {"type": "exact", "answer": "test"}
				},
				{
					"id": "q2",
					"description": "Second question - depends on q1",
					"type": "text",
					"points": 10,
					"dependsOn": ["q1"],
					"validation": {"type": "exact", "answer": "test"}
				}
			]
		}`)

		sessionRepo := &mockSessionRepoForQuestions{
			sessions: []*models.Session{
				{ID: "session1", UserID: "user1", LabTemplateID: "tmpl1"},
			},
		}
		labTemplateRepo := &mockLabTemplateRepoForQuestions{
			templates: map[string]*models.LabTemplateRecord{
				"tmpl1": {ID: "tmpl1", Name: "Test Lab", Spec: specWithDeps},
			},
		}
		checkpointRepo := &mockCheckpointRepoForQuestions{}
		questionResponseRepo := &mockQuestionResponseRepo{} // No prior responses, so q1 not answered

		mgr := createTestManager(sessionRepo, labTemplateRepo, checkpointRepo, questionResponseRepo)

		// Try to answer q2 without completing q1
		body := `{"responseText": "answer"}`
		req := httptest.NewRequest(http.MethodPost, "/sessions/session1/questions/q2/answer", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionID", "session1")
		rctx.URLParams.Add("questionID", "q2")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user1", Email: "user@test.com"})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler := mgr.handleSubmitQuestionAnswer()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
		}
	})

	t.Run("failed to create response", func(t *testing.T) {
		sessionRepo := &mockSessionRepoForQuestions{
			sessions: []*models.Session{
				{ID: "session1", UserID: "user1", LabTemplateID: "tmpl1"},
			},
		}
		labTemplateRepo := &mockLabTemplateRepoForQuestions{
			templates: map[string]*models.LabTemplateRecord{
				"tmpl1": {ID: "tmpl1", Name: "Test Lab", Spec: specJSON},
			},
		}
		checkpointRepo := &mockCheckpointRepoForQuestions{}
		questionResponseRepo := &mockQuestionResponseRepo{
			createErr: errors.New("database error"),
		}

		mgr := createTestManager(sessionRepo, labTemplateRepo, checkpointRepo, questionResponseRepo)

		body := `{"responseText": "answer"}`
		req := httptest.NewRequest(http.MethodPost, "/sessions/session1/questions/q1/answer", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionID", "session1")
		rctx.URLParams.Add("questionID", "q1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user1", Email: "user@test.com"})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler := mgr.handleSubmitQuestionAnswer()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, rr.Code, rr.Body.String())
		}
	})

	t.Run("failed to update response", func(t *testing.T) {
		sessionRepo := &mockSessionRepoForQuestions{
			sessions: []*models.Session{
				{ID: "session1", UserID: "user1", LabTemplateID: "tmpl1"},
			},
		}
		labTemplateRepo := &mockLabTemplateRepoForQuestions{
			templates: map[string]*models.LabTemplateRecord{
				"tmpl1": {ID: "tmpl1", Name: "Test Lab", Spec: specJSON},
			},
		}
		checkpointRepo := &mockCheckpointRepoForQuestions{}
		questionResponseRepo := &mockQuestionResponseRepo{
			responses: []*models.QuestionResponse{
				{
					SessionID:  "session1",
					QuestionID: "q1",
					Status:     models.QuestionStatusPending,
				},
			},
			updateErr: errors.New("database error"),
		}

		mgr := createTestManager(sessionRepo, labTemplateRepo, checkpointRepo, questionResponseRepo)

		body := `{"responseText": "answer"}`
		req := httptest.NewRequest(http.MethodPost, "/sessions/session1/questions/q1/answer", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionID", "session1")
		rctx.URLParams.Add("questionID", "q1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user1", Email: "user@test.com"})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler := mgr.handleSubmitQuestionAnswer()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, rr.Code, rr.Body.String())
		}
	})

	// submitAnswer is a small helper that wires up a request against the
	// shared spec fixture (q1 text question, expected answer "correct") and
	// returns the recorded response. Keeps the per-case blocks focused on
	// the input/assertion rather than httptest boilerplate.
	submitAnswer := func(t *testing.T, specBytes []byte, body string, questionID string) *httptest.ResponseRecorder {
		t.Helper()
		sessionRepo := &mockSessionRepoForQuestions{
			sessions: []*models.Session{
				{ID: "session1", UserID: "user1", LabTemplateID: "tmpl1"},
			},
		}
		labTemplateRepo := &mockLabTemplateRepoForQuestions{
			templates: map[string]*models.LabTemplateRecord{
				"tmpl1": {ID: "tmpl1", Name: "Test Lab", Spec: specBytes},
			},
		}
		checkpointRepo := &mockCheckpointRepoForQuestions{}
		questionResponseRepo := &mockQuestionResponseRepo{}

		mgr := createTestManager(sessionRepo, labTemplateRepo, checkpointRepo, questionResponseRepo)

		req := httptest.NewRequest(http.MethodPost, "/sessions/session1/questions/"+questionID+"/answer", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionID", "session1")
		rctx.URLParams.Add("questionID", questionID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		req = req.WithContext(auth.ContextWithUser(req.Context(), &auth.User{ID: "user1", Email: "user@test.com"}))

		rr := httptest.NewRecorder()
		mgr.handleSubmitQuestionAnswer().ServeHTTP(rr, req)
		return rr
	}

	parseResp := func(t *testing.T, rr *httptest.ResponseRecorder) models.SubmitAnswerResponse {
		t.Helper()
		var resp models.SubmitAnswerResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v, body: %s", err, rr.Body.String())
		}
		return resp
	}

	t.Run("text: incorrect answer is rejected", func(t *testing.T) {
		rr := submitAnswer(t, specJSON, `{"responseText": "wrong"}`, "q1")
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
		}
		resp := parseResp(t, rr)
		if resp.IsCorrect {
			t.Error("expected isCorrect=false for wrong answer")
		}
		if resp.EarnedPoints != 0 {
			t.Errorf("expected 0 points for wrong answer, got %d", resp.EarnedPoints)
		}
		if resp.QuestionID != "q1" {
			t.Errorf("expected questionID 'q1', got '%s'", resp.QuestionID)
		}
		if resp.MaxPoints != 10 {
			t.Errorf("expected maxPoints 10, got %d", resp.MaxPoints)
		}
		if resp.AttemptCount != 1 {
			t.Errorf("expected attemptCount 1, got %d", resp.AttemptCount)
		}
		if resp.Feedback == "" {
			t.Error("expected non-empty feedback on incorrect answer")
		}
	})

	t.Run("text: correct answer awards full points", func(t *testing.T) {
		rr := submitAnswer(t, specJSON, `{"responseText": "correct"}`, "q1")
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
		}
		resp := parseResp(t, rr)
		if !resp.IsCorrect {
			t.Error("expected isCorrect=true for exact match")
		}
		if resp.EarnedPoints != 10 {
			t.Errorf("expected 10 points on correct text answer, got %d", resp.EarnedPoints)
		}
	})

	t.Run("text: case-insensitive match by default", func(t *testing.T) {
		// Default CaseSensitive=false — "CORRECT" should match "correct".
		rr := submitAnswer(t, specJSON, `{"responseText": "CORRECT"}`, "q1")
		resp := parseResp(t, rr)
		if !resp.IsCorrect {
			t.Error("expected case-insensitive match to accept 'CORRECT'")
		}
	})

	t.Run("text: case-sensitive rejects mismatched case", func(t *testing.T) {
		csSpec := models.LabSpec{
			Questions: []models.Question{{
				ID: "q1", Description: "case-sensitive Q", Type: models.QuestionTypeText, Points: 10,
				Validation: &models.QuestionValidation{
					Type: models.ValidationTypeExact, Answer: "correct", CaseSensitive: true,
				},
			}},
		}
		csJSON, _ := json.Marshal(csSpec)
		rr := submitAnswer(t, csJSON, `{"responseText": "CORRECT"}`, "q1")
		resp := parseResp(t, rr)
		if resp.IsCorrect {
			t.Error("expected case-sensitive mismatch to reject 'CORRECT'")
		}
	})

	t.Run("text: regex pattern matches", func(t *testing.T) {
		reSpec := models.LabSpec{
			Questions: []models.Question{{
				ID: "q1", Description: "regex Q", Type: models.QuestionTypeText, Points: 15,
				Validation: &models.QuestionValidation{
					Type: models.ValidationTypeRegex, Pattern: `^\d{3}-\d{4}$`,
				},
			}},
		}
		reJSON, _ := json.Marshal(reSpec)

		t.Run("matching input", func(t *testing.T) {
			rr := submitAnswer(t, reJSON, `{"responseText": "555-1234"}`, "q1")
			resp := parseResp(t, rr)
			if !resp.IsCorrect {
				t.Error("expected regex match to succeed")
			}
			if resp.EarnedPoints != 15 {
				t.Errorf("expected 15 points, got %d", resp.EarnedPoints)
			}
		})

		t.Run("non-matching input", func(t *testing.T) {
			rr := submitAnswer(t, reJSON, `{"responseText": "not a phone"}`, "q1")
			resp := parseResp(t, rr)
			if resp.IsCorrect {
				t.Error("expected regex non-match to fail")
			}
		})
	})

	t.Run("multiple-choice single-select", func(t *testing.T) {
		mcSpec := models.LabSpec{
			Questions: []models.Question{{
				ID: "q1", Description: "pick one", Type: models.QuestionTypeMultipleChoice, Points: 20,
				Options: []models.QuestionOption{
					{ID: "a", Text: "Option A", Correct: false},
					{ID: "b", Text: "Option B", Correct: true},
					{ID: "c", Text: "Option C", Correct: false},
				},
			}},
		}
		mcJSON, _ := json.Marshal(mcSpec)

		t.Run("correct selection", func(t *testing.T) {
			rr := submitAnswer(t, mcJSON, `{"selectedOptions": ["b"]}`, "q1")
			resp := parseResp(t, rr)
			if !resp.IsCorrect {
				t.Error("expected option b to be correct")
			}
			if resp.EarnedPoints != 20 {
				t.Errorf("expected 20 points, got %d", resp.EarnedPoints)
			}
		})

		t.Run("incorrect selection", func(t *testing.T) {
			rr := submitAnswer(t, mcJSON, `{"selectedOptions": ["a"]}`, "q1")
			resp := parseResp(t, rr)
			if resp.IsCorrect {
				t.Error("expected option a to be incorrect")
			}
		})

		t.Run("multiple options rejected for single-select", func(t *testing.T) {
			rr := submitAnswer(t, mcJSON, `{"selectedOptions": ["a", "b"]}`, "q1")
			resp := parseResp(t, rr)
			if resp.IsCorrect {
				t.Error("single-select should reject multi-option submission")
			}
		})
	})

	t.Run("multiple-choice multi-select", func(t *testing.T) {
		msSpec := models.LabSpec{
			Questions: []models.Question{{
				ID: "q1", Description: "pick all that apply", Type: models.QuestionTypeMultipleChoice, Points: 30,
				MultiSelect: true,
				Options: []models.QuestionOption{
					{ID: "a", Text: "Option A", Correct: true},
					{ID: "b", Text: "Option B", Correct: false},
					{ID: "c", Text: "Option C", Correct: true},
				},
			}},
		}
		msJSON, _ := json.Marshal(msSpec)

		t.Run("all correct options selected", func(t *testing.T) {
			rr := submitAnswer(t, msJSON, `{"selectedOptions": ["a", "c"]}`, "q1")
			resp := parseResp(t, rr)
			if !resp.IsCorrect {
				t.Error("expected [a,c] to match both-correct set")
			}
			if resp.EarnedPoints != 30 {
				t.Errorf("expected 30 points, got %d", resp.EarnedPoints)
			}
		})

		t.Run("partial correct fails multi-select", func(t *testing.T) {
			rr := submitAnswer(t, msJSON, `{"selectedOptions": ["a"]}`, "q1")
			resp := parseResp(t, rr)
			if resp.IsCorrect {
				t.Error("expected partial correct [a] to fail multi-select")
			}
		})

		t.Run("includes wrong option fails", func(t *testing.T) {
			rr := submitAnswer(t, msJSON, `{"selectedOptions": ["a", "b", "c"]}`, "q1")
			resp := parseResp(t, rr)
			if resp.IsCorrect {
				t.Error("expected submission including wrong option to fail")
			}
		})
	})
}

func TestHandleGetQuestionHint(t *testing.T) {
	spec := models.LabSpec{
		Questions: []models.Question{
			{
				ID:          "q1",
				Description: "What is the answer?",
				Type:        models.QuestionTypeText,
				Points:      10,
				Hint:        "The answer is a word.",
				Validation: &models.QuestionValidation{
					Type:   models.ValidationTypeExact,
					Answer: "correct",
				},
			},
			{
				ID:          "q2",
				Description: "No hint available",
				Type:        models.QuestionTypeText,
				Points:      10,
				Hint:        "", // No hint
				Validation: &models.QuestionValidation{
					Type:   models.ValidationTypeExact,
					Answer: "answer",
				},
			},
		},
	}
	hintSpecJSON, _ := json.Marshal(spec)

	t.Run("session not found", func(t *testing.T) {
		sessionRepo := &mockSessionRepoForQuestions{}
		labTemplateRepo := &mockLabTemplateRepoForQuestions{}
		checkpointRepo := &mockCheckpointRepoForQuestions{}
		questionResponseRepo := &mockQuestionResponseRepo{}

		mgr := createTestManager(sessionRepo, labTemplateRepo, checkpointRepo, questionResponseRepo)

		req := httptest.NewRequest(http.MethodPost, "/sessions/nonexistent/questions/q1/hint", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionID", "nonexistent")
		rctx.URLParams.Add("questionID", "q1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user1", Email: "user@test.com"})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler := mgr.handleGetQuestionHint()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("access denied for other user", func(t *testing.T) {
		sessionRepo := &mockSessionRepoForQuestions{
			sessions: []*models.Session{
				{ID: "session1", UserID: "owner1", LabTemplateID: "tmpl1"},
			},
		}
		labTemplateRepo := &mockLabTemplateRepoForQuestions{}
		checkpointRepo := &mockCheckpointRepoForQuestions{}
		questionResponseRepo := &mockQuestionResponseRepo{}

		mgr := createTestManager(sessionRepo, labTemplateRepo, checkpointRepo, questionResponseRepo)

		req := httptest.NewRequest(http.MethodPost, "/sessions/session1/questions/q1/hint", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionID", "session1")
		rctx.URLParams.Add("questionID", "q1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		// Different user
		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "other-user", Email: "other@test.com"})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler := mgr.handleGetQuestionHint()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
		}
	})

	t.Run("lab template not found", func(t *testing.T) {
		sessionRepo := &mockSessionRepoForQuestions{
			sessions: []*models.Session{
				{ID: "session1", UserID: "user1", LabTemplateID: "tmpl1"},
			},
		}
		labTemplateRepo := &mockLabTemplateRepoForQuestions{} // No templates
		checkpointRepo := &mockCheckpointRepoForQuestions{}
		questionResponseRepo := &mockQuestionResponseRepo{}

		mgr := createTestManager(sessionRepo, labTemplateRepo, checkpointRepo, questionResponseRepo)

		req := httptest.NewRequest(http.MethodPost, "/sessions/session1/questions/q1/hint", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionID", "session1")
		rctx.URLParams.Add("questionID", "q1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user1", Email: "user@test.com"})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler := mgr.handleGetQuestionHint()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("failed to parse template spec", func(t *testing.T) {
		sessionRepo := &mockSessionRepoForQuestions{
			sessions: []*models.Session{
				{ID: "session1", UserID: "user1", LabTemplateID: "tmpl1"},
			},
		}
		labTemplateRepo := &mockLabTemplateRepoForQuestions{
			templates: map[string]*models.LabTemplateRecord{
				"tmpl1": {ID: "tmpl1", Name: "Test Lab", Spec: []byte(`{invalid json}`)},
			},
		}
		checkpointRepo := &mockCheckpointRepoForQuestions{}
		questionResponseRepo := &mockQuestionResponseRepo{}

		mgr := createTestManager(sessionRepo, labTemplateRepo, checkpointRepo, questionResponseRepo)

		req := httptest.NewRequest(http.MethodPost, "/sessions/session1/questions/q1/hint", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionID", "session1")
		rctx.URLParams.Add("questionID", "q1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user1", Email: "user@test.com"})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler := mgr.handleGetQuestionHint()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})

	t.Run("question not found", func(t *testing.T) {
		sessionRepo := &mockSessionRepoForQuestions{
			sessions: []*models.Session{
				{ID: "session1", UserID: "user1", LabTemplateID: "tmpl1"},
			},
		}
		labTemplateRepo := &mockLabTemplateRepoForQuestions{
			templates: map[string]*models.LabTemplateRecord{
				"tmpl1": {ID: "tmpl1", Name: "Test Lab", Spec: hintSpecJSON},
			},
		}
		checkpointRepo := &mockCheckpointRepoForQuestions{}
		questionResponseRepo := &mockQuestionResponseRepo{}

		mgr := createTestManager(sessionRepo, labTemplateRepo, checkpointRepo, questionResponseRepo)

		req := httptest.NewRequest(http.MethodPost, "/sessions/session1/questions/nonexistent/hint", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionID", "session1")
		rctx.URLParams.Add("questionID", "nonexistent")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user1", Email: "user@test.com"})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler := mgr.handleGetQuestionHint()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("no hint available", func(t *testing.T) {
		sessionRepo := &mockSessionRepoForQuestions{
			sessions: []*models.Session{
				{ID: "session1", UserID: "user1", LabTemplateID: "tmpl1"},
			},
		}
		labTemplateRepo := &mockLabTemplateRepoForQuestions{
			templates: map[string]*models.LabTemplateRecord{
				"tmpl1": {ID: "tmpl1", Name: "Test Lab", Spec: hintSpecJSON},
			},
		}
		checkpointRepo := &mockCheckpointRepoForQuestions{}
		questionResponseRepo := &mockQuestionResponseRepo{}

		mgr := createTestManager(sessionRepo, labTemplateRepo, checkpointRepo, questionResponseRepo)

		req := httptest.NewRequest(http.MethodPost, "/sessions/session1/questions/q2/hint", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionID", "session1")
		rctx.URLParams.Add("questionID", "q2")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user1", Email: "user@test.com"})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler := mgr.handleGetQuestionHint()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("returns hint successfully", func(t *testing.T) {
		sessionRepo := &mockSessionRepoForQuestions{
			sessions: []*models.Session{
				{ID: "session1", UserID: "user1", LabTemplateID: "tmpl1"},
			},
		}
		labTemplateRepo := &mockLabTemplateRepoForQuestions{
			templates: map[string]*models.LabTemplateRecord{
				"tmpl1": {ID: "tmpl1", Name: "Test Lab", Spec: hintSpecJSON},
			},
		}
		checkpointRepo := &mockCheckpointRepoForQuestions{}
		questionResponseRepo := &mockQuestionResponseRepo{}

		mgr := createTestManager(sessionRepo, labTemplateRepo, checkpointRepo, questionResponseRepo)

		req := httptest.NewRequest(http.MethodPost, "/sessions/session1/questions/q1/hint", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionID", "session1")
		rctx.URLParams.Add("questionID", "q1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user1", Email: "user@test.com"})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler := mgr.handleGetQuestionHint()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var resp models.GetHintResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if resp.Hint != "The answer is a word." {
			t.Errorf("expected hint 'The answer is a word.', got '%s'", resp.Hint)
		}

		if resp.QuestionID != "q1" {
			t.Errorf("expected questionID 'q1', got '%s'", resp.QuestionID)
		}
	})

	t.Run("marks hint as shown for existing response", func(t *testing.T) {
		sessionRepo := &mockSessionRepoForQuestions{
			sessions: []*models.Session{
				{ID: "session1", UserID: "user1", LabTemplateID: "tmpl1"},
			},
		}
		labTemplateRepo := &mockLabTemplateRepoForQuestions{
			templates: map[string]*models.LabTemplateRecord{
				"tmpl1": {ID: "tmpl1", Name: "Test Lab", Spec: hintSpecJSON},
			},
		}
		checkpointRepo := &mockCheckpointRepoForQuestions{}
		questionResponseRepo := &mockQuestionResponseRepo{
			responses: []*models.QuestionResponse{
				{
					SessionID:      "session1",
					QuestionID:     "q1",
					Status:         models.QuestionStatusPending,
					HintLevelShown: 0,
				},
			},
		}

		mgr := createTestManager(sessionRepo, labTemplateRepo, checkpointRepo, questionResponseRepo)

		req := httptest.NewRequest(http.MethodPost, "/sessions/session1/questions/q1/hint", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionID", "session1")
		rctx.URLParams.Add("questionID", "q1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user1", Email: "user@test.com"})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler := mgr.handleGetQuestionHint()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// Checkpoint Hint Handler Tests
// -----------------------------------------------------------------------------

// mockCheckpointHintRepo implements CheckpointHintRepository for testing
type mockCheckpointHintRepo struct {
	hintProgress map[string]*models.CheckpointHintProgress
}

func (r *mockCheckpointHintRepo) GetBySessionID(ctx context.Context, sessionID string) ([]*models.CheckpointHintProgress, error) {
	var results []*models.CheckpointHintProgress
	for _, p := range r.hintProgress {
		if p.SessionID == sessionID {
			results = append(results, p)
		}
	}
	return results, nil
}

func (r *mockCheckpointHintRepo) GetHintLevel(ctx context.Context, sessionID, checkpointID string) (level int, penalty int, err error) {
	key := sessionID + "-" + checkpointID
	if p, ok := r.hintProgress[key]; ok {
		return p.HintLevelShown, p.HintPenaltyApplied, nil
	}
	return 0, 0, nil
}

func (r *mockCheckpointHintRepo) ShowHintLevel(ctx context.Context, sessionID, checkpointID string, level, penalty int) error {
	if r.hintProgress == nil {
		r.hintProgress = make(map[string]*models.CheckpointHintProgress)
	}
	key := sessionID + "-" + checkpointID
	if p, ok := r.hintProgress[key]; ok {
		if level > p.HintLevelShown {
			p.HintLevelShown = level
			p.HintPenaltyApplied += penalty
		}
	} else {
		r.hintProgress[key] = &models.CheckpointHintProgress{
			SessionID:          sessionID,
			CheckpointID:       checkpointID,
			HintLevelShown:     level,
			HintPenaltyApplied: penalty,
		}
	}
	return nil
}

func (r *mockCheckpointHintRepo) Create(ctx context.Context, progress *models.CheckpointHintProgress) error {
	if r.hintProgress == nil {
		r.hintProgress = make(map[string]*models.CheckpointHintProgress)
	}
	key := progress.SessionID + "-" + progress.CheckpointID
	r.hintProgress[key] = progress
	return nil
}

// createTestManagerWithCheckpointHints creates a Manager with checkpoint hint repo
func createTestManagerWithCheckpointHints(
	sessionRepo repositories.SessionRepository,
	labTemplateRepo repositories.LabTemplateRepository,
	checkpointRepo repositories.CheckpointProgressRepository,
	checkpointHintRepo repositories.CheckpointHintRepository,
	questionResponseRepo repositories.QuestionResponseRepository,
) *Manager {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewManager(Config{
		SessionRepo:          sessionRepo,
		LabTemplateRepo:      labTemplateRepo,
		CheckpointRepo:       checkpointRepo,
		CheckpointHintRepo:   checkpointHintRepo,
		QuestionResponseRepo: questionResponseRepo,
		Logger:               logger,
		Responder:            httputil.NewResponder(logger),
	})
}

func TestHandleGetCheckpointHint(t *testing.T) {
	// Create template spec with checkpoints that have progressive hints
	spec := models.LabSpec{
		Objectives: []models.Checkpoint{
			{
				ID:          "cp1",
				Description: "Create a configuration file",
				Points:      20,
				Hints: []models.HintLevel{
					{Level: 1, Text: "Look in the home directory", PointPenalty: 0},
					{Level: 2, Text: "Use touch ~/.config", PointPenalty: 2},
					{Level: 3, Text: "Run: touch ~/.config", PointPenalty: 5},
				},
			},
			{
				ID:          "cp2",
				Description: "A checkpoint without hints",
				Points:      10,
			},
		},
	}
	checkpointSpecJSON, _ := json.Marshal(spec)

	t.Run("session not found", func(t *testing.T) {
		sessionRepo := &mockSessionRepoForQuestions{}
		labTemplateRepo := &mockLabTemplateRepoForQuestions{}
		checkpointRepo := &mockCheckpointRepoForQuestions{}
		checkpointHintRepo := &mockCheckpointHintRepo{}
		questionResponseRepo := &mockQuestionResponseRepo{}

		mgr := createTestManagerWithCheckpointHints(sessionRepo, labTemplateRepo, checkpointRepo, checkpointHintRepo, questionResponseRepo)

		req := httptest.NewRequest(http.MethodPost, "/sessions/nonexistent/checkpoints/cp1/hint", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionID", "nonexistent")
		rctx.URLParams.Add("checkpointID", "cp1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user1", Email: "user@test.com"})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler := mgr.handleGetCheckpointHint()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("access denied for other user", func(t *testing.T) {
		sessionRepo := &mockSessionRepoForQuestions{
			sessions: []*models.Session{
				{ID: "session1", UserID: "owner1", LabTemplateID: "tmpl1"},
			},
		}
		labTemplateRepo := &mockLabTemplateRepoForQuestions{}
		checkpointRepo := &mockCheckpointRepoForQuestions{}
		checkpointHintRepo := &mockCheckpointHintRepo{}
		questionResponseRepo := &mockQuestionResponseRepo{}

		mgr := createTestManagerWithCheckpointHints(sessionRepo, labTemplateRepo, checkpointRepo, checkpointHintRepo, questionResponseRepo)

		req := httptest.NewRequest(http.MethodPost, "/sessions/session1/checkpoints/cp1/hint", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionID", "session1")
		rctx.URLParams.Add("checkpointID", "cp1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		// Different user
		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "other-user", Email: "other@test.com"})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler := mgr.handleGetCheckpointHint()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
		}
	})

	t.Run("lab template not found", func(t *testing.T) {
		sessionRepo := &mockSessionRepoForQuestions{
			sessions: []*models.Session{
				{ID: "session1", UserID: "user1", LabTemplateID: "tmpl1"},
			},
		}
		labTemplateRepo := &mockLabTemplateRepoForQuestions{} // No templates
		checkpointRepo := &mockCheckpointRepoForQuestions{}
		checkpointHintRepo := &mockCheckpointHintRepo{}
		questionResponseRepo := &mockQuestionResponseRepo{}

		mgr := createTestManagerWithCheckpointHints(sessionRepo, labTemplateRepo, checkpointRepo, checkpointHintRepo, questionResponseRepo)

		req := httptest.NewRequest(http.MethodPost, "/sessions/session1/checkpoints/cp1/hint", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionID", "session1")
		rctx.URLParams.Add("checkpointID", "cp1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user1", Email: "user@test.com"})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler := mgr.handleGetCheckpointHint()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("checkpoint not found", func(t *testing.T) {
		sessionRepo := &mockSessionRepoForQuestions{
			sessions: []*models.Session{
				{ID: "session1", UserID: "user1", LabTemplateID: "tmpl1"},
			},
		}
		labTemplateRepo := &mockLabTemplateRepoForQuestions{
			templates: map[string]*models.LabTemplateRecord{
				"tmpl1": {ID: "tmpl1", Name: "Test Lab", Spec: checkpointSpecJSON},
			},
		}
		checkpointRepo := &mockCheckpointRepoForQuestions{}
		checkpointHintRepo := &mockCheckpointHintRepo{}
		questionResponseRepo := &mockQuestionResponseRepo{}

		mgr := createTestManagerWithCheckpointHints(sessionRepo, labTemplateRepo, checkpointRepo, checkpointHintRepo, questionResponseRepo)

		req := httptest.NewRequest(http.MethodPost, "/sessions/session1/checkpoints/nonexistent/hint", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionID", "session1")
		rctx.URLParams.Add("checkpointID", "nonexistent")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user1", Email: "user@test.com"})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler := mgr.handleGetCheckpointHint()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("no hints available", func(t *testing.T) {
		sessionRepo := &mockSessionRepoForQuestions{
			sessions: []*models.Session{
				{ID: "session1", UserID: "user1", LabTemplateID: "tmpl1"},
			},
		}
		labTemplateRepo := &mockLabTemplateRepoForQuestions{
			templates: map[string]*models.LabTemplateRecord{
				"tmpl1": {ID: "tmpl1", Name: "Test Lab", Spec: checkpointSpecJSON},
			},
		}
		checkpointRepo := &mockCheckpointRepoForQuestions{}
		checkpointHintRepo := &mockCheckpointHintRepo{}
		questionResponseRepo := &mockQuestionResponseRepo{}

		mgr := createTestManagerWithCheckpointHints(sessionRepo, labTemplateRepo, checkpointRepo, checkpointHintRepo, questionResponseRepo)

		req := httptest.NewRequest(http.MethodPost, "/sessions/session1/checkpoints/cp2/hint", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionID", "session1")
		rctx.URLParams.Add("checkpointID", "cp2")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user1", Email: "user@test.com"})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler := mgr.handleGetCheckpointHint()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("returns first hint successfully", func(t *testing.T) {
		sessionRepo := &mockSessionRepoForQuestions{
			sessions: []*models.Session{
				{ID: "session1", UserID: "user1", LabTemplateID: "tmpl1"},
			},
		}
		labTemplateRepo := &mockLabTemplateRepoForQuestions{
			templates: map[string]*models.LabTemplateRecord{
				"tmpl1": {ID: "tmpl1", Name: "Test Lab", Spec: checkpointSpecJSON},
			},
		}
		checkpointRepo := &mockCheckpointRepoForQuestions{}
		checkpointHintRepo := &mockCheckpointHintRepo{}
		questionResponseRepo := &mockQuestionResponseRepo{}

		mgr := createTestManagerWithCheckpointHints(sessionRepo, labTemplateRepo, checkpointRepo, checkpointHintRepo, questionResponseRepo)

		req := httptest.NewRequest(http.MethodPost, "/sessions/session1/checkpoints/cp1/hint", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionID", "session1")
		rctx.URLParams.Add("checkpointID", "cp1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user1", Email: "user@test.com"})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler := mgr.handleGetCheckpointHint()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var resp models.GetHintResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if resp.CheckpointID != "cp1" {
			t.Errorf("expected checkpointId 'cp1', got '%s'", resp.CheckpointID)
		}
		if resp.Level != 1 {
			t.Errorf("expected level 1, got %d", resp.Level)
		}
		if resp.MaxLevel != 3 {
			t.Errorf("expected maxLevel 3, got %d", resp.MaxLevel)
		}
		if resp.Hint != "Look in the home directory" {
			t.Errorf("expected hint 'Look in the home directory', got '%s'", resp.Hint)
		}
		if resp.Penalty != 0 {
			t.Errorf("expected penalty 0, got %d", resp.Penalty)
		}
	})

	t.Run("returns progressive hints with penalty", func(t *testing.T) {
		sessionRepo := &mockSessionRepoForQuestions{
			sessions: []*models.Session{
				{ID: "session1", UserID: "user1", LabTemplateID: "tmpl1"},
			},
		}
		labTemplateRepo := &mockLabTemplateRepoForQuestions{
			templates: map[string]*models.LabTemplateRecord{
				"tmpl1": {ID: "tmpl1", Name: "Test Lab", Spec: checkpointSpecJSON},
			},
		}
		checkpointRepo := &mockCheckpointRepoForQuestions{}
		// Pre-populate with level 1 already shown
		checkpointHintRepo := &mockCheckpointHintRepo{
			hintProgress: map[string]*models.CheckpointHintProgress{
				"session1-cp1": {
					SessionID:          "session1",
					CheckpointID:       "cp1",
					HintLevelShown:     1,
					HintPenaltyApplied: 0,
				},
			},
		}
		questionResponseRepo := &mockQuestionResponseRepo{}

		mgr := createTestManagerWithCheckpointHints(sessionRepo, labTemplateRepo, checkpointRepo, checkpointHintRepo, questionResponseRepo)

		req := httptest.NewRequest(http.MethodPost, "/sessions/session1/checkpoints/cp1/hint", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionID", "session1")
		rctx.URLParams.Add("checkpointID", "cp1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user1", Email: "user@test.com"})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler := mgr.handleGetCheckpointHint()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var resp models.GetHintResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		// Should return level 2 (next after level 1)
		if resp.Level != 2 {
			t.Errorf("expected level 2, got %d", resp.Level)
		}
		if resp.Hint != "Use touch ~/.config" {
			t.Errorf("expected hint 'Use touch ~/.config', got '%s'", resp.Hint)
		}
		if resp.Penalty != 2 {
			t.Errorf("expected penalty 2, got %d", resp.Penalty)
		}
		if resp.TotalPenalty != 2 {
			t.Errorf("expected totalPenalty 2, got %d", resp.TotalPenalty)
		}
	})

	t.Run("re-viewing shown hint has no penalty", func(t *testing.T) {
		sessionRepo := &mockSessionRepoForQuestions{
			sessions: []*models.Session{
				{ID: "session1", UserID: "user1", LabTemplateID: "tmpl1"},
			},
		}
		labTemplateRepo := &mockLabTemplateRepoForQuestions{
			templates: map[string]*models.LabTemplateRecord{
				"tmpl1": {ID: "tmpl1", Name: "Test Lab", Spec: checkpointSpecJSON},
			},
		}
		checkpointRepo := &mockCheckpointRepoForQuestions{}
		// Level 2 already shown with penalty
		checkpointHintRepo := &mockCheckpointHintRepo{
			hintProgress: map[string]*models.CheckpointHintProgress{
				"session1-cp1": {
					SessionID:          "session1",
					CheckpointID:       "cp1",
					HintLevelShown:     2,
					HintPenaltyApplied: 2,
				},
			},
		}
		questionResponseRepo := &mockQuestionResponseRepo{}

		mgr := createTestManagerWithCheckpointHints(sessionRepo, labTemplateRepo, checkpointRepo, checkpointHintRepo, questionResponseRepo)

		// Request level 1 (already shown)
		req := httptest.NewRequest(http.MethodPost, "/sessions/session1/checkpoints/cp1/hint?level=1", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionID", "session1")
		rctx.URLParams.Add("checkpointID", "cp1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user1", Email: "user@test.com"})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler := mgr.handleGetCheckpointHint()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var resp models.GetHintResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		// Should return level 1 with no new penalty
		if resp.Level != 1 {
			t.Errorf("expected level 1, got %d", resp.Level)
		}
		if resp.Hint != "Look in the home directory" {
			t.Errorf("expected hint 'Look in the home directory', got '%s'", resp.Hint)
		}
		if resp.Penalty != 0 {
			t.Errorf("expected penalty 0 for re-viewing, got %d", resp.Penalty)
		}
		if resp.TotalPenalty != 2 {
			t.Errorf("expected totalPenalty 2 (unchanged), got %d", resp.TotalPenalty)
		}
	})

	t.Run("max level exceeded returns error", func(t *testing.T) {
		sessionRepo := &mockSessionRepoForQuestions{
			sessions: []*models.Session{
				{ID: "session1", UserID: "user1", LabTemplateID: "tmpl1"},
			},
		}
		labTemplateRepo := &mockLabTemplateRepoForQuestions{
			templates: map[string]*models.LabTemplateRecord{
				"tmpl1": {ID: "tmpl1", Name: "Test Lab", Spec: checkpointSpecJSON},
			},
		}
		checkpointRepo := &mockCheckpointRepoForQuestions{}
		// All hints already shown
		checkpointHintRepo := &mockCheckpointHintRepo{
			hintProgress: map[string]*models.CheckpointHintProgress{
				"session1-cp1": {
					SessionID:          "session1",
					CheckpointID:       "cp1",
					HintLevelShown:     3,
					HintPenaltyApplied: 7,
				},
			},
		}
		questionResponseRepo := &mockQuestionResponseRepo{}

		mgr := createTestManagerWithCheckpointHints(sessionRepo, labTemplateRepo, checkpointRepo, checkpointHintRepo, questionResponseRepo)

		req := httptest.NewRequest(http.MethodPost, "/sessions/session1/checkpoints/cp1/hint", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("sessionID", "session1")
		rctx.URLParams.Add("checkpointID", "cp1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user1", Email: "user@test.com"})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler := mgr.handleGetCheckpointHint()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})
}

func (r *mockSessionRepoForQuestions) CountActive(ctx context.Context) (int64, error) {
	return 0, nil
}
