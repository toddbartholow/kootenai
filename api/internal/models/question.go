// Package models contains shared data types for the lab platform
package models

import "time"

// -----------------------------------------------------------------------------
// Question Types (for question-based lab assessments)
// -----------------------------------------------------------------------------

// QuestionType defines the type of question
type QuestionType string

const (
	QuestionTypeText           QuestionType = "text"
	QuestionTypeMultipleChoice QuestionType = "multiple_choice"
)

// QuestionValidationType defines how to validate the answer
type QuestionValidationType string

const (
	ValidationTypeExact QuestionValidationType = "exact"
	ValidationTypeRegex QuestionValidationType = "regex"
)

// QuestionResponseStatus defines the status of a response
type QuestionResponseStatus string

const (
	QuestionStatusPending   QuestionResponseStatus = "pending"
	QuestionStatusCorrect   QuestionResponseStatus = "correct"
	QuestionStatusIncorrect QuestionResponseStatus = "incorrect"
	QuestionStatusPartial   QuestionResponseStatus = "partial" // For multi-select with partial correct
)

// -----------------------------------------------------------------------------
// Progressive Hints Types
// -----------------------------------------------------------------------------

// HintLevel represents one level of a progressive hint
type HintLevel struct {
	Level        int    `yaml:"level" json:"level"`                         // 1, 2, 3...
	Text         string `yaml:"text" json:"text"`                           // The hint text
	PointPenalty int    `yaml:"penalty,omitempty" json:"penalty,omitempty"` // Points deducted when revealed
}

// -----------------------------------------------------------------------------
// Question Template Types (defined in lab YAML/JSON spec)
// -----------------------------------------------------------------------------

// Question defines a question in a lab template
type Question struct {
	ID          string              `yaml:"id" json:"id"`
	Type        QuestionType        `yaml:"type" json:"type"`
	Description string              `yaml:"description" json:"description"`
	Points      int                 `yaml:"points" json:"points"`
	Hint        string              `yaml:"hint,omitempty" json:"hint,omitempty"`   // Legacy single hint (backward compatible)
	Hints       []HintLevel         `yaml:"hints,omitempty" json:"hints,omitempty"` // Progressive hints
	DependsOn   []string            `yaml:"dependsOn,omitempty" json:"dependsOn,omitempty"`
	Order       int                 `yaml:"order,omitempty" json:"order,omitempty"`
	Required    bool                `yaml:"required,omitempty" json:"required,omitempty"`
	Validation  *QuestionValidation `yaml:"validation,omitempty" json:"validation,omitempty"`
	Options     []QuestionOption    `yaml:"options,omitempty" json:"options,omitempty"`
	MultiSelect bool                `yaml:"multiSelect,omitempty" json:"multiSelect,omitempty"`
}

// GetHintCount returns the total number of available hints
func (q *Question) GetHintCount() int {
	if len(q.Hints) > 0 {
		return len(q.Hints)
	}
	if q.Hint != "" {
		return 1
	}
	return 0
}

// GetHintForLevel returns the hint for a specific level (1-indexed)
// Returns empty string if level is invalid
func (q *Question) GetHintForLevel(level int) *HintLevel {
	if level < 1 {
		return nil
	}

	// Progressive hints take precedence
	if len(q.Hints) > 0 {
		for i := range q.Hints {
			if q.Hints[i].Level == level {
				return &q.Hints[i]
			}
		}
		// If no matching level found, try by index
		if level <= len(q.Hints) {
			return &q.Hints[level-1]
		}
		return nil
	}

	// Fallback to legacy single hint (level 1 only)
	if level == 1 && q.Hint != "" {
		return &HintLevel{
			Level:        1,
			Text:         q.Hint,
			PointPenalty: 0,
		}
	}

	return nil
}

// HasHints returns true if any hints are available
func (q *Question) HasHints() bool {
	return q.GetHintCount() > 0
}

// QuestionValidation defines how to validate a text answer.
//
// Answer/Pattern are fields the server compares submitted responses against.
// They MUST round-trip through JSON because the authoring pipeline
// (parseAndValidateSpec) marshals a parsed LabTemplate to JSON before
// writing the spec blob to the `lab_templates.spec` column; the handler
// later unmarshals that blob to validate answers.
//
// Client-safety is not enforced at the struct-tag level — it's enforced by
// the response-shaping types (QuestionPublic, QuestionOptionPublic) which
// omit these fields entirely. Every HTTP response path uses the Public
// variants; the raw Question type is never serialized to a client.
// Regression coverage: TestQuestionPublic_StripsSensitiveFields.
type QuestionValidation struct {
	Type          QuestionValidationType `yaml:"type" json:"type"`
	Answer        string                 `yaml:"answer,omitempty" json:"answer,omitempty"`
	Pattern       string                 `yaml:"pattern,omitempty" json:"pattern,omitempty"`
	CaseSensitive bool                   `yaml:"caseSensitive,omitempty" json:"caseSensitive,omitempty"`
}

// QuestionOption defines an option for multiple choice questions.
// `Correct` round-trips through JSON for the same spec-storage reason as
// QuestionValidation.Answer above; client-safety is enforced by
// QuestionOptionPublic.
type QuestionOption struct {
	ID      string `yaml:"id" json:"id"`
	Text    string `yaml:"text" json:"text"`
	Correct bool   `yaml:"correct" json:"correct,omitempty"`
}

// QuestionOptionPublic is the client-safe version (no correct flag)
type QuestionOptionPublic struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// QuestionPublic is the client-safe version of Question (no answers)
type QuestionPublic struct {
	ID          string                 `json:"id"`
	Type        QuestionType           `json:"type"`
	Description string                 `json:"description"`
	Points      int                    `json:"points"`
	Hint        string                 `json:"hint,omitempty"`      // Legacy single hint (for backward compatibility)
	HintCount   int                    `json:"hintCount,omitempty"` // Number of progressive hints available
	DependsOn   []string               `json:"dependsOn,omitempty"`
	Order       int                    `json:"order,omitempty"`
	Options     []QuestionOptionPublic `json:"options,omitempty"`
	MultiSelect bool                   `json:"multiSelect,omitempty"`
}

// ToPublic converts a Question to its client-safe version
func (q *Question) ToPublic() QuestionPublic {
	pub := QuestionPublic{
		ID:          q.ID,
		Type:        q.Type,
		Description: q.Description,
		Points:      q.Points,
		Hint:        q.Hint,
		HintCount:   q.GetHintCount(),
		DependsOn:   q.DependsOn,
		Order:       q.Order,
		MultiSelect: q.MultiSelect,
	}

	if len(q.Options) > 0 {
		pub.Options = make([]QuestionOptionPublic, len(q.Options))
		for i, opt := range q.Options {
			pub.Options[i] = QuestionOptionPublic{
				ID:   opt.ID,
				Text: opt.Text,
			}
		}
	}

	return pub
}

// -----------------------------------------------------------------------------
// Question Response Types (stored in database)
// -----------------------------------------------------------------------------

// QuestionResponse represents a student's answer to a question
type QuestionResponse struct {
	ID                 string                 `json:"id"`
	SessionID          string                 `json:"sessionId"`
	QuestionID         string                 `json:"questionId"`
	QuestionType       string                 `json:"questionType"`
	Points             int                    `json:"points"`
	ResponseText       string                 `json:"responseText,omitempty"`
	SelectedOptions    []string               `json:"selectedOptions,omitempty"`
	Status             QuestionResponseStatus `json:"status"`
	EarnedPoints       int                    `json:"earnedPoints"`
	IsCorrect          bool                   `json:"isCorrect"`
	AttemptCount       int                    `json:"attemptCount"`
	FirstAttemptAt     *time.Time             `json:"firstAttemptAt,omitempty"`
	LastAttemptAt      *time.Time             `json:"lastAttemptAt,omitempty"`
	CorrectAt          *time.Time             `json:"correctAt,omitempty"`
	Feedback           string                 `json:"feedback,omitempty"`
	HintLevelShown     int                    `json:"hintLevelShown"`     // Highest hint level revealed (0 = none)
	HintPenaltyApplied int                    `json:"hintPenaltyApplied"` // Total penalty from revealed hints
	CreatedAt          time.Time              `json:"createdAt"`
	UpdatedAt          time.Time              `json:"updatedAt"`
}

// -----------------------------------------------------------------------------
// API Request/Response Types
// -----------------------------------------------------------------------------

// SubmitAnswerRequest represents a request to submit an answer
type SubmitAnswerRequest struct {
	ResponseText    string   `json:"responseText,omitempty"`
	SelectedOptions []string `json:"selectedOptions,omitempty"`
}

// SubmitAnswerResponse represents the result of submitting an answer
type SubmitAnswerResponse struct {
	QuestionID   string                 `json:"questionId"`
	Status       QuestionResponseStatus `json:"status"`
	IsCorrect    bool                   `json:"isCorrect"`
	EarnedPoints int                    `json:"earnedPoints"`
	MaxPoints    int                    `json:"maxPoints"`
	Feedback     string                 `json:"feedback,omitempty"`
	AttemptCount int                    `json:"attemptCount"`
}

// QuestionProgressResponse represents a question with its current response status
type QuestionProgressResponse struct {
	QuestionPublic
	Status             QuestionResponseStatus `json:"status"`
	EarnedPoints       int                    `json:"earnedPoints"`
	AttemptCount       int                    `json:"attemptCount"`
	IsLocked           bool                   `json:"isLocked"`           // True if dependencies not met
	HintAvailable      bool                   `json:"hintAvailable"`      // True if any hints exist
	HintLevelShown     int                    `json:"hintLevelShown"`     // Highest hint level revealed (0 = none)
	HintPenaltyApplied int                    `json:"hintPenaltyApplied"` // Total penalty from hints
}

// QuestionsListResponse is the response for listing session questions
type QuestionsListResponse struct {
	SessionID    string                     `json:"sessionId"`
	Questions    []QuestionProgressResponse `json:"questions"`
	TotalPoints  int                        `json:"totalPoints"`
	EarnedPoints int                        `json:"earnedPoints"`
}

// GetHintResponse is the response for getting a question hint (progressive)
type GetHintResponse struct {
	QuestionID   string `json:"questionId,omitempty"`
	CheckpointID string `json:"checkpointId,omitempty"`
	Level        int    `json:"level"`        // Current hint level revealed
	MaxLevel     int    `json:"maxLevel"`     // Total hints available
	Hint         string `json:"hint"`         // The hint text for this level
	Penalty      int    `json:"penalty"`      // Points deducted for this hint
	TotalPenalty int    `json:"totalPenalty"` // Total penalty applied so far
}
