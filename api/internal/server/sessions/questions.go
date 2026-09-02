package sessions

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// -----------------------------------------------------------------------------
// Question Handlers
// -----------------------------------------------------------------------------

// handleGetSessionQuestions returns questions for a session with current progress
// GET /sessions/{sessionID}/questions
func (m *Manager) handleGetSessionQuestions() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := chi.URLParam(r, "sessionID")
		ctx := r.Context()
		user, _ := auth.UserFromContext(ctx)

		// Verify session access
		sess, err := m.sessionRepo.GetByID(ctx, sessionID)
		if err != nil || sess == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "question.errors.sessionNotFound", nil)
			return
		}
		if !canAccessSession(user, sess) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "question.errors.accessDenied", nil)
			return
		}

		// Get questions from lab template
		template, err := m.labTemplateRepo.GetByID(ctx, sess.LabTemplateID)
		if err != nil || template == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "question.errors.labTemplateNotFound", nil)
			return
		}

		// Parse questions from template spec
		var spec models.LabSpec
		if err := json.Unmarshal(template.Spec, &spec); err != nil {
			m.logger.Error("Failed to parse template spec", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "question.errors.parseTemplateFailed", nil)
			return
		}

		// If no questions defined, return empty list
		if len(spec.Questions) == 0 {
			m.responder.JSONResponse(w, http.StatusOK, models.QuestionsListResponse{
				SessionID:    sessionID,
				Questions:    []models.QuestionProgressResponse{},
				TotalPoints:  0,
				EarnedPoints: 0,
			})
			return
		}

		// Get existing responses
		responses, err := m.questionResponseRepo.GetBySessionID(ctx, sessionID)
		if err != nil {
			m.logger.Error("Failed to get question responses", "error", err)
			// Don't fail - just proceed with empty responses
			responses = nil
		}

		responseMap := make(map[string]*models.QuestionResponse)
		for _, resp := range responses {
			responseMap[resp.QuestionID] = resp
		}

		// Get checkpoint progress for dependency checking
		checkpointProgress, _ := m.checkpointRepo.GetBySessionID(ctx, sessionID)
		passedCheckpoints := make(map[string]bool)
		for _, cp := range checkpointProgress {
			if cp.Status == models.CheckpointStatusPassed {
				passedCheckpoints[cp.CheckpointID] = true
			}
		}

		// Build response with progress
		result := make([]models.QuestionProgressResponse, 0, len(spec.Questions))
		totalPoints := 0
		earnedPoints := 0

		for _, q := range spec.Questions {
			totalPoints += q.Points

			progress := models.QuestionProgressResponse{
				QuestionPublic:     q.ToPublic(),
				Status:             models.QuestionStatusPending,
				EarnedPoints:       0,
				AttemptCount:       0,
				IsLocked:           !areDependenciesMet(q.DependsOn, responseMap, passedCheckpoints),
				HintAvailable:      q.HasHints(),
				HintLevelShown:     0,
				HintPenaltyApplied: 0,
			}

			if resp, ok := responseMap[q.ID]; ok {
				progress.Status = resp.Status
				progress.EarnedPoints = resp.EarnedPoints
				progress.AttemptCount = resp.AttemptCount
				progress.HintLevelShown = resp.HintLevelShown
				progress.HintPenaltyApplied = resp.HintPenaltyApplied
				earnedPoints += resp.EarnedPoints
			}

			result = append(result, progress)
		}

		m.responder.JSONResponse(w, http.StatusOK, models.QuestionsListResponse{
			SessionID:    sessionID,
			Questions:    result,
			TotalPoints:  totalPoints,
			EarnedPoints: earnedPoints,
		})
	}
}

// handleSubmitQuestionAnswer validates and records an answer
// POST /sessions/{sessionID}/questions/{questionID}/answer
func (m *Manager) handleSubmitQuestionAnswer() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := chi.URLParam(r, "sessionID")
		questionID := chi.URLParam(r, "questionID")
		ctx := r.Context()
		user, _ := auth.UserFromContext(ctx)

		// Verify session access and ownership
		sess, err := m.sessionRepo.GetByID(ctx, sessionID)
		if err != nil || sess == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "question.errors.sessionNotFound", nil)
			return
		}
		if !canModifySession(user, sess) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "question.errors.accessDenied", nil)
			return
		}

		// Check if session has ended
		if sess.EndedAt != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "question.errors.sessionEnded", nil)
			return
		}

		// Parse request
		var req models.SubmitAnswerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
			return
		}

		// Get question definition from template
		template, err := m.labTemplateRepo.GetByID(ctx, sess.LabTemplateID)
		if err != nil || template == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "question.errors.labTemplateNotFound", nil)
			return
		}

		var spec models.LabSpec
		if err := json.Unmarshal(template.Spec, &spec); err != nil {
			m.logger.Error("Failed to parse template spec", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "question.errors.parseTemplateFailed", nil)
			return
		}

		// Find the question
		var question *models.Question
		for i := range spec.Questions {
			if spec.Questions[i].ID == questionID {
				question = &spec.Questions[i]
				break
			}
		}
		if question == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "question.errors.notFound", nil)
			return
		}

		// Check dependencies
		responses, _ := m.questionResponseRepo.GetBySessionID(ctx, sessionID)
		responseMap := make(map[string]*models.QuestionResponse)
		for _, resp := range responses {
			responseMap[resp.QuestionID] = resp
		}

		checkpointProgress, _ := m.checkpointRepo.GetBySessionID(ctx, sessionID)
		passedCheckpoints := make(map[string]bool)
		for _, cp := range checkpointProgress {
			if cp.Status == models.CheckpointStatusPassed {
				passedCheckpoints[cp.CheckpointID] = true
			}
		}

		if !areDependenciesMet(question.DependsOn, responseMap, passedCheckpoints) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "question.errors.prerequisitesNotMet", nil)
			return
		}

		// Validate answer
		isCorrect, feedback := validateAnswer(question, &req)

		// Get or create response record
		existing, _ := m.questionResponseRepo.GetBySessionAndQuestion(ctx, sessionID, questionID)
		now := time.Now()

		response := &models.QuestionResponse{
			SessionID:     sessionID,
			QuestionID:    questionID,
			QuestionType:  string(question.Type),
			Points:        question.Points,
			AttemptCount:  1,
			LastAttemptAt: &now,
		}

		if existing != nil {
			response = existing
			response.AttemptCount++
			response.LastAttemptAt = &now
		} else {
			response.ID = uuid.New().String()
			response.FirstAttemptAt = &now
		}

		// Update response data
		if question.Type == models.QuestionTypeText {
			response.ResponseText = req.ResponseText
		} else {
			response.SelectedOptions = req.SelectedOptions
		}

		if isCorrect {
			response.Status = models.QuestionStatusCorrect
			response.IsCorrect = true
			response.EarnedPoints = question.Points
			response.CorrectAt = &now
			response.Feedback = "Correct!"
		} else {
			response.Status = models.QuestionStatusIncorrect
			response.IsCorrect = false
			response.EarnedPoints = 0
			response.Feedback = feedback
		}

		// Save response
		if existing != nil {
			if err := m.questionResponseRepo.Update(ctx, response); err != nil {
				m.logger.Error("Failed to update question response", "error", err)
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "question.errors.saveResponseFailed", nil)
				return
			}
		} else {
			if err := m.questionResponseRepo.Create(ctx, response); err != nil {
				m.logger.Error("Failed to create question response", "error", err)
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "question.errors.saveResponseFailed", nil)
				return
			}
		}

		m.responder.JSONResponse(w, http.StatusOK, models.SubmitAnswerResponse{
			QuestionID:   questionID,
			Status:       response.Status,
			IsCorrect:    response.IsCorrect,
			EarnedPoints: response.EarnedPoints,
			MaxPoints:    question.Points,
			Feedback:     response.Feedback,
			AttemptCount: response.AttemptCount,
		})
	}
}

// handleGetQuestionHint returns a progressive hint for a question
// POST /sessions/{sessionID}/questions/{questionID}/hint?level=N
func (m *Manager) handleGetQuestionHint() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := chi.URLParam(r, "sessionID")
		questionID := chi.URLParam(r, "questionID")
		ctx := r.Context()
		user, _ := auth.UserFromContext(ctx)

		// Verify session access
		sess, err := m.sessionRepo.GetByID(ctx, sessionID)
		if err != nil || sess == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "question.errors.sessionNotFound", nil)
			return
		}
		if !canAccessSession(user, sess) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "question.errors.accessDenied", nil)
			return
		}

		// Get question definition from template
		template, err := m.labTemplateRepo.GetByID(ctx, sess.LabTemplateID)
		if err != nil || template == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "question.errors.labTemplateNotFound", nil)
			return
		}

		var spec models.LabSpec
		if err := json.Unmarshal(template.Spec, &spec); err != nil {
			m.logger.Error("Failed to parse template spec", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "question.errors.parseTemplateFailed", nil)
			return
		}

		// Find the question
		var question *models.Question
		for i := range spec.Questions {
			if spec.Questions[i].ID == questionID {
				question = &spec.Questions[i]
				break
			}
		}
		if question == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "question.errors.notFound", nil)
			return
		}

		maxLevel := question.GetHintCount()
		if maxLevel == 0 {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "question.errors.noHintsAvailable", nil)
			return
		}

		// Get current hint level and penalty
		currentLevel, totalPenalty, err := m.questionResponseRepo.GetHintLevel(ctx, sessionID, questionID)
		if err != nil {
			m.logger.Error("Failed to get hint level", "error", err)
			// Continue with level 0
			currentLevel = 0
			totalPenalty = 0
		}

		// Determine which level to show
		// Default: next level after current
		requestedLevel := currentLevel + 1

		// Parse optional level parameter
		if levelStr := r.URL.Query().Get("level"); levelStr != "" {
			if parsed, err := strconv.Atoi(levelStr); err == nil && parsed > 0 {
				requestedLevel = parsed
			}
		}

		// Validate the requested level
		if requestedLevel > maxLevel {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "question.errors.noMoreHints", nil)
			return
		}

		// If already shown this level or higher, return the current hint without penalty
		if requestedLevel <= currentLevel {
			hint := question.GetHintForLevel(requestedLevel)
			if hint == nil {
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "question.errors.hintNotFound", nil)
				return
			}
			m.responder.JSONResponse(w, http.StatusOK, models.GetHintResponse{
				QuestionID:   questionID,
				Level:        requestedLevel,
				MaxLevel:     maxLevel,
				Hint:         hint.Text,
				Penalty:      0, // No penalty for already-shown hints
				TotalPenalty: totalPenalty,
			})
			return
		}

		// Get the hint for the requested level
		hint := question.GetHintForLevel(requestedLevel)
		if hint == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "question.errors.hintNotFound", nil)
			return
		}

		// Ensure response record exists before updating hint level
		existing, _ := m.questionResponseRepo.GetBySessionAndQuestion(ctx, sessionID, questionID)
		if existing == nil {
			// Create a pending response record first
			now := time.Now()
			response := &models.QuestionResponse{
				ID:           uuid.New().String(),
				SessionID:    sessionID,
				QuestionID:   questionID,
				QuestionType: string(question.Type),
				Points:       question.Points,
				Status:       models.QuestionStatusPending,
				CreatedAt:    now,
				UpdatedAt:    now,
			}
			if err := m.questionResponseRepo.Create(ctx, response); err != nil {
				m.logger.Warn("Failed to create response for hint", "error", err)
			}
		}

		// Update hint level and apply penalty
		if err := m.questionResponseRepo.ShowHintLevel(ctx, sessionID, questionID, requestedLevel, hint.PointPenalty); err != nil {
			m.logger.Warn("Failed to update hint level", "error", err)
		}

		m.responder.JSONResponse(w, http.StatusOK, models.GetHintResponse{
			QuestionID:   questionID,
			Level:        requestedLevel,
			MaxLevel:     maxLevel,
			Hint:         hint.Text,
			Penalty:      hint.PointPenalty,
			TotalPenalty: totalPenalty + hint.PointPenalty,
		})
	}
}

// -----------------------------------------------------------------------------
// Checkpoint Hint Handler
// -----------------------------------------------------------------------------

// handleGetCheckpointHint returns a progressive hint for a checkpoint
func (m *Manager) handleGetCheckpointHint() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := chi.URLParam(r, "sessionID")
		checkpointID := chi.URLParam(r, "checkpointID")
		ctx := r.Context()
		user, _ := auth.UserFromContext(ctx)

		// Verify session access
		sess, err := m.sessionRepo.GetByID(ctx, sessionID)
		if err != nil || sess == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "question.errors.sessionNotFound", nil)
			return
		}
		if !canAccessSession(user, sess) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "question.errors.accessDenied", nil)
			return
		}

		// Get checkpoint definition from template
		template, err := m.labTemplateRepo.GetByID(ctx, sess.LabTemplateID)
		if err != nil || template == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "question.errors.labTemplateNotFound", nil)
			return
		}

		var spec models.LabSpec
		if err := json.Unmarshal(template.Spec, &spec); err != nil {
			m.logger.Error("Failed to parse template spec", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "question.errors.parseTemplateFailed", nil)
			return
		}

		// Find the checkpoint in Objectives
		var checkpoint *models.Checkpoint
		for i := range spec.Objectives {
			if spec.Objectives[i].ID == checkpointID {
				checkpoint = &spec.Objectives[i]
				break
			}
		}
		if checkpoint == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "question.errors.checkpointNotFound", nil)
			return
		}

		maxLevel := checkpoint.GetHintCount()
		if maxLevel == 0 {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "question.errors.noHintsAvailable", nil)
			return
		}

		// Check if checkpoint hint repo is available
		if m.checkpointHintRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "question.errors.checkpointHintNotAvailable", nil)
			return
		}

		// Get current hint level and penalty
		currentLevel, totalPenalty, err := m.checkpointHintRepo.GetHintLevel(ctx, sessionID, checkpointID)
		if err != nil {
			m.logger.Error("Failed to get checkpoint hint level", "error", err)
			currentLevel = 0
			totalPenalty = 0
		}

		// Determine which level to show
		requestedLevel := currentLevel + 1

		// Parse optional level parameter
		if levelStr := r.URL.Query().Get("level"); levelStr != "" {
			if parsed, err := strconv.Atoi(levelStr); err == nil && parsed > 0 {
				requestedLevel = parsed
			}
		}

		// Validate the requested level
		if requestedLevel > maxLevel {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "question.errors.noMoreHints", nil)
			return
		}

		// If already shown this level or higher, return the current hint without penalty
		if requestedLevel <= currentLevel {
			hint := checkpoint.GetHintForLevel(requestedLevel)
			if hint == nil {
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "question.errors.hintNotFound", nil)
				return
			}
			m.responder.JSONResponse(w, http.StatusOK, models.GetHintResponse{
				CheckpointID: checkpointID,
				Level:        requestedLevel,
				MaxLevel:     maxLevel,
				Hint:         hint.Text,
				Penalty:      0,
				TotalPenalty: totalPenalty,
			})
			return
		}

		// Get the hint for the requested level
		hint := checkpoint.GetHintForLevel(requestedLevel)
		if hint == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "question.errors.hintNotFound", nil)
			return
		}

		// Update hint level and apply penalty
		if err := m.checkpointHintRepo.ShowHintLevel(ctx, sessionID, checkpointID, requestedLevel, hint.PointPenalty); err != nil {
			m.logger.Warn("Failed to update checkpoint hint level", "error", err)
		}

		m.responder.JSONResponse(w, http.StatusOK, models.GetHintResponse{
			CheckpointID: checkpointID,
			Level:        requestedLevel,
			MaxLevel:     maxLevel,
			Hint:         hint.Text,
			Penalty:      hint.PointPenalty,
			TotalPenalty: totalPenalty + hint.PointPenalty,
		})
	}
}

// -----------------------------------------------------------------------------
// Helper Functions
// -----------------------------------------------------------------------------

// validateAnswer checks if the submitted answer is correct
func validateAnswer(q *models.Question, req *models.SubmitAnswerRequest) (bool, string) {
	switch q.Type {
	case models.QuestionTypeText:
		return validateTextAnswer(q, req.ResponseText)
	case models.QuestionTypeMultipleChoice:
		return validateMultipleChoiceAnswer(q, req.SelectedOptions)
	}
	return false, "Unknown question type"
}

// validateTextAnswer validates a text answer using exact or regex matching
func validateTextAnswer(q *models.Question, answer string) (bool, string) {
	if q.Validation == nil {
		return false, "Question not properly configured"
	}

	switch q.Validation.Type {
	case models.ValidationTypeExact:
		expected := q.Validation.Answer
		actual := answer
		if !q.Validation.CaseSensitive {
			expected = strings.ToLower(expected)
			actual = strings.ToLower(actual)
		}
		if strings.TrimSpace(actual) == strings.TrimSpace(expected) {
			return true, ""
		}
		return false, "Incorrect answer. Please try again."

	case models.ValidationTypeRegex:
		re, err := regexp.Compile(q.Validation.Pattern)
		if err != nil {
			return false, "Invalid question configuration"
		}
		if re.MatchString(answer) {
			return true, ""
		}
		return false, "Answer format is incorrect. Please try again."
	}

	return false, "Unknown validation type"
}

// validateMultipleChoiceAnswer validates a multiple choice answer
func validateMultipleChoiceAnswer(q *models.Question, selected []string) (bool, string) {
	if len(q.Options) == 0 {
		return false, "Question not properly configured"
	}

	// Get correct options
	correctSet := make(map[string]bool)
	for _, opt := range q.Options {
		if opt.Correct {
			correctSet[opt.ID] = true
		}
	}

	// For single-select questions
	if !q.MultiSelect {
		if len(selected) != 1 {
			return false, "Please select exactly one answer."
		}
		if correctSet[selected[0]] {
			return true, ""
		}
		return false, "Incorrect answer. Please try again."
	}

	// For multi-select questions, check if selected matches correct exactly
	if len(selected) != len(correctSet) {
		return false, "Incorrect selection. Please try again."
	}

	for _, sel := range selected {
		if !correctSet[sel] {
			return false, "Incorrect selection. Please try again."
		}
	}

	return true, ""
}

// areDependenciesMet checks if all dependencies (questions and checkpoints) are met
func areDependenciesMet(deps []string, questionResponses map[string]*models.QuestionResponse, checkpoints map[string]bool) bool {
	for _, dep := range deps {
		// Check if it's a question dependency
		if resp, ok := questionResponses[dep]; ok {
			if resp.Status != models.QuestionStatusCorrect {
				return false
			}
			continue
		}
		// Check if it's a checkpoint dependency
		if !checkpoints[dep] {
			return false
		}
	}
	return true
}
