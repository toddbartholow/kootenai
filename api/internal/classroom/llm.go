package classroom

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

const anthropicAPIURL = "https://api.anthropic.com/v1/messages"

// AnthropicClient calls the Anthropic API to generate student content.
type AnthropicClient struct {
	apiKey     string
	httpClient *http.Client
	model      string
	logger     *slog.Logger
}

// NewAnthropicClient creates a new Anthropic API client.
func NewAnthropicClient(apiKey string, logger *slog.Logger) *AnthropicClient {
	return &AnthropicClient{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		model:  "claude-sonnet-4-20250514",
		logger: logger,
	}
}

// anthropicRequest is the API request payload.
type anthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	System      string             `json:"system,omitempty"`
	Messages    []anthropicMessage `json:"messages"`
	Temperature float64            `json:"temperature,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// anthropicResponse is the API response payload.
type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

// generate sends a request to the Anthropic API and returns the response text.
func (c *AnthropicClient) generate(ctx context.Context, system, prompt string, temperature float64, maxTokens int) (string, error) {
	if c.apiKey == "" {
		return c.fallbackGenerate(prompt), nil
	}

	reqBody := anthropicRequest{
		Model:     c.model,
		MaxTokens: maxTokens,
		System:    system,
		Messages: []anthropicMessage{
			{Role: "user", Content: prompt},
		},
		Temperature: temperature,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicAPIURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("anthropic API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var apiResp anthropicResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return "", fmt.Errorf("parsing response: %w", err)
	}

	if len(apiResp.Content) == 0 {
		return "", fmt.Errorf("empty response from anthropic API")
	}

	return apiResp.Content[0].Text, nil
}

// fallbackGenerate returns a canned response when no API key is configured.
func (c *AnthropicClient) fallbackGenerate(prompt string) string {
	return fmt.Sprintf("[Simulated response - no API key configured] Based on the prompt: %.100s...", prompt)
}

// GenerateAssignmentResponse generates an assignment submission for a student persona.
func (c *AnthropicClient) GenerateAssignmentResponse(ctx context.Context, profile PersonalityProfile, assignmentTitle, assignmentDescription, courseContext string) (string, error) {
	system := fmt.Sprintf(
		"You are a %s student. Personality traits: motivation=%d, conscientiousness=%d, confidence=%d, anxiety=%d. "+
			"Write in a %s style. Quality level: %s. Keep your response under 500 words.",
		profile.Personality,
		profile.Traits.Motivation, profile.Traits.Conscientiousness,
		profile.Traits.Confidence, profile.Traits.Anxiety,
		profile.BehavioralConfig.DiscussionStyle,
		profile.BehavioralConfig.AssignmentQuality,
	)

	prompt := fmt.Sprintf("Course context: %s\n\nAssignment: %s\n%s\n\nWrite your submission.",
		courseContext, assignmentTitle, assignmentDescription)

	return c.generate(ctx, system, prompt, profile.BehavioralConfig.LLMTemperature, 1024)
}

// GenerateDiscussionPost generates a discussion forum post for a student persona.
func (c *AnthropicClient) GenerateDiscussionPost(ctx context.Context, profile PersonalityProfile, topic string, existingPosts []string) (string, error) {
	system := fmt.Sprintf(
		"You are a %s student. Write a discussion post in a %s style. "+
			"Your confidence level is %d/10 and motivation is %d/10. Keep it under 200 words.",
		profile.Personality,
		profile.BehavioralConfig.DiscussionStyle,
		profile.Traits.Confidence, profile.Traits.Motivation,
	)

	prompt := fmt.Sprintf("Discussion topic: %s\n", topic)
	if len(existingPosts) > 0 {
		prompt += "\nExisting posts from classmates:\n"
		for i, post := range existingPosts {
			if i >= 3 {
				break
			}
			prompt += fmt.Sprintf("- %s\n", post)
		}
		prompt += "\nWrite your response, optionally referencing classmates' posts."
	} else {
		prompt += "\nWrite the first post on this topic."
	}

	return c.generate(ctx, system, prompt, profile.BehavioralConfig.LLMTemperature, 512)
}

// QuizAnswer represents an answer to a quiz question.
type QuizAnswer struct {
	QuestionID string `json:"questionId"`
	Answer     string `json:"answer"`
	Correct    bool   `json:"correct"`
}

// LabFeedback holds structured feedback about a lab from an AI student's perspective.
type LabFeedback struct {
	Summary                 string   `json:"summary"`
	Rating                  int      `json:"rating"`
	InstructionClarity      int      `json:"instructionClarity"`
	CheckpointAchievability int      `json:"checkpointAchievability"`
	HintHelpfulness         int      `json:"hintHelpfulness"`
	DifficultyCalibration   string   `json:"difficultyCalibration"`
	Issues                  []string `json:"issues,omitempty"`
}

// PathwayFeedback holds structured feedback about a learning pathway.
type PathwayFeedback struct {
	Summary                 string   `json:"summary"`
	Rating                  int      `json:"rating"`
	ModuleOrdering          int      `json:"moduleOrdering"`
	PrerequisiteSufficiency int      `json:"prerequisiteSufficiency"`
	PacingScore             int      `json:"pacingScore"`
	Suggestions             []string `json:"suggestions,omitempty"`
}

// LabSummary provides a brief summary of lab execution for pathway-level feedback.
type LabSummary struct {
	LabName           string `json:"labName"`
	Success           bool   `json:"success"`
	DurationMs        int64  `json:"durationMs"`
	CheckpointsPassed int    `json:"checkpointsPassed"`
	CheckpointsTotal  int    `json:"checkpointsTotal"`
	Rating            int    `json:"rating,omitempty"`
}

// GenerateLabFeedback generates structured feedback about a lab from a student persona's perspective.
func (c *AnthropicClient) GenerateLabFeedback(
	ctx context.Context,
	profile PersonalityProfile,
	labName, labDesc string,
	execResult *ExecuteLabResult,
	checkpointDescs, hints []string,
) (*LabFeedback, error) {
	system := fmt.Sprintf(
		"You are a QA reviewer evaluating a cybersecurity lab from the perspective of a %s student "+
			"with skill level: motivation=%d, conscientiousness=%d, confidence=%d. "+
			"Respond ONLY with valid JSON matching this structure: "+
			`{"summary":"...","rating":N,"instructionClarity":N,"checkpointAchievability":N,`+
			`"hintHelpfulness":N,"difficultyCalibration":"too_easy|appropriate|too_hard","issues":["..."]}. `+
			"All ratings are 1-5. Be specific and actionable in issues.",
		profile.Personality,
		profile.Traits.Motivation, profile.Traits.Conscientiousness, profile.Traits.Confidence,
	)

	// Truncate output to last 2000 chars
	output := execResult.Output
	if len(output) > 2000 {
		output = output[len(output)-2000:]
	}

	prompt := fmt.Sprintf(
		"Lab: %s\nDescription: %s\n\nCheckpoints (%d/%d passed):\n",
		labName, labDesc, execResult.CheckpointsPassed, execResult.CheckpointsTotal,
	)
	for i, cp := range checkpointDescs {
		status := "FAILED"
		if i < execResult.CheckpointsPassed {
			status = "PASSED"
		}
		prompt += fmt.Sprintf("- [%s] %s\n", status, cp)
	}
	if len(hints) > 0 {
		prompt += "\nHints provided:\n"
		for _, h := range hints {
			prompt += fmt.Sprintf("- %s\n", h)
		}
	}
	prompt += fmt.Sprintf("\nExecution time: %dms\nSuccess: %v\nOutput (last 2000 chars):\n%s",
		execResult.DurationMs, execResult.Success, output)

	text, err := c.generate(ctx, system, prompt, 0.3, 1024)
	if err != nil {
		return nil, fmt.Errorf("generating lab feedback: %w", err)
	}

	var feedback LabFeedback
	if err := json.Unmarshal([]byte(text), &feedback); err != nil {
		c.logger.Warn("Failed to parse lab feedback JSON, using raw text", "error", err)
		feedback = LabFeedback{
			Summary: text,
			Rating:  3,
		}
	}
	return &feedback, nil
}

// GeneratePathwayFeedback generates feedback about a learning pathway after completing all labs.
func (c *AnthropicClient) GeneratePathwayFeedback(
	ctx context.Context,
	profile PersonalityProfile,
	pathwayName string,
	labResults []LabSummary,
) (*PathwayFeedback, error) {
	system := fmt.Sprintf(
		"You are a QA reviewer evaluating a cybersecurity learning pathway from the perspective of a %s student. "+
			"Respond ONLY with valid JSON matching this structure: "+
			`{"summary":"...","rating":N,"moduleOrdering":N,"prerequisiteSufficiency":N,`+
			`"pacingScore":N,"suggestions":["..."]}. `+
			"All ratings are 1-5. Be specific and actionable.",
		profile.Personality,
	)

	prompt := fmt.Sprintf("Pathway: %s\n\nLabs completed in order:\n", pathwayName)
	for _, lab := range labResults {
		prompt += fmt.Sprintf("- %s: %d/%d checkpoints, %dms, success=%v\n",
			lab.LabName, lab.CheckpointsPassed, lab.CheckpointsTotal, lab.DurationMs, lab.Success)
	}

	text, err := c.generate(ctx, system, prompt, 0.3, 1024)
	if err != nil {
		return nil, fmt.Errorf("generating pathway feedback: %w", err)
	}

	var feedback PathwayFeedback
	if err := json.Unmarshal([]byte(text), &feedback); err != nil {
		c.logger.Warn("Failed to parse pathway feedback JSON, using raw text", "error", err)
		feedback = PathwayFeedback{
			Summary: text,
			Rating:  3,
		}
	}
	return &feedback, nil
}

// GenerateQuizAnswers generates quiz answers at the student's skill level.
func (c *AnthropicClient) GenerateQuizAnswers(ctx context.Context, profile PersonalityProfile, questions []string) ([]QuizAnswer, error) {
	system := fmt.Sprintf(
		"You are a %s student taking a quiz. Your skill level means you score between %d%% and %d%%. "+
			"For each question, provide an answer. Some answers should be wrong based on your skill level. "+
			"Respond in JSON format: [{\"questionId\": \"q1\", \"answer\": \"your answer\"}]",
		profile.Personality,
		profile.BehavioralConfig.QuizScoreMin, profile.BehavioralConfig.QuizScoreMax,
	)

	prompt := "Answer these quiz questions:\n"
	for i, q := range questions {
		prompt += fmt.Sprintf("q%d: %s\n", i+1, q)
	}

	text, err := c.generate(ctx, system, prompt, profile.BehavioralConfig.LLMTemperature, 1024)
	if err != nil {
		return nil, err
	}

	var answers []QuizAnswer
	if err := json.Unmarshal([]byte(text), &answers); err != nil {
		// If LLM didn't return valid JSON, create placeholder answers
		c.logger.Warn("Failed to parse quiz answers from LLM, using placeholders", "error", err)
		for i := range questions {
			answers = append(answers, QuizAnswer{
				QuestionID: fmt.Sprintf("q%d", i+1),
				Answer:     "placeholder answer",
			})
		}
	}

	return answers, nil
}
