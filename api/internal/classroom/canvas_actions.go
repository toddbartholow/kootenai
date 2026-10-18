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

// CanvasClient wraps the Canvas REST API for classroom simulation actions.
type CanvasClient struct {
	baseURL    string
	apiToken   string
	httpClient *http.Client
	logger     *slog.Logger
}

// NewCanvasClient creates a new Canvas REST API client.
func NewCanvasClient(baseURL, apiToken string, logger *slog.Logger) *CanvasClient {
	return &CanvasClient{
		baseURL:  baseURL,
		apiToken: apiToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// IsConfigured returns true if Canvas credentials are set.
func (c *CanvasClient) IsConfigured() bool {
	return c.baseURL != "" && c.apiToken != ""
}

// canvasRequest sends an authenticated request to the Canvas API.
func (c *CanvasClient) canvasRequest(ctx context.Context, method, path string, body any) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	url := fmt.Sprintf("%s/api/v1%s", c.baseURL, path)
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("canvas API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// CanvasUser represents a Canvas user.
type CanvasUser struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"login_id"`
}

// CreateUser creates a user in Canvas.
func (c *CanvasClient) CreateUser(ctx context.Context, name, email, password string) (*CanvasUser, error) {
	payload := map[string]any{
		"user": map[string]any{
			"name":       name,
			"short_name": name,
		},
		"pseudonym": map[string]any{
			"unique_id":               email,
			"password":                password,
			"send_confirmation":       false,
			"force_self_registration": false,
		},
	}

	data, err := c.canvasRequest(ctx, http.MethodPost, "/accounts/1/users", payload)
	if err != nil {
		return nil, fmt.Errorf("creating canvas user: %w", err)
	}

	var user CanvasUser
	if err := json.Unmarshal(data, &user); err != nil {
		return nil, fmt.Errorf("parsing canvas user: %w", err)
	}

	c.logger.Info("Created Canvas user", "canvasId", user.ID, "name", name)
	return &user, nil
}

// EnrollUser enrolls a user in a Canvas course.
func (c *CanvasClient) EnrollUser(ctx context.Context, courseID string, userID int, role string) error {
	payload := map[string]any{
		"enrollment": map[string]any{
			"user_id":          userID,
			"type":             role + "Enrollment",
			"enrollment_state": "active",
			"notify":           false,
		},
	}

	_, err := c.canvasRequest(ctx, http.MethodPost, fmt.Sprintf("/courses/%s/enrollments", courseID), payload)
	if err != nil {
		return fmt.Errorf("enrolling user in canvas course: %w", err)
	}

	c.logger.Info("Enrolled user in Canvas course", "courseId", courseID, "userId", userID, "role", role)
	return nil
}

// SubmitAssignment submits an assignment in Canvas.
func (c *CanvasClient) SubmitAssignment(ctx context.Context, courseID string, assignmentID, userID int, body string) error {
	payload := map[string]any{
		"submission": map[string]any{
			"submission_type": "online_text_entry",
			"body":            body,
		},
	}

	path := fmt.Sprintf("/courses/%s/assignments/%d/submissions", courseID, assignmentID)
	// Canvas requires acting as the user for submission
	path += fmt.Sprintf("?as_user_id=%d", userID)
	_, err := c.canvasRequest(ctx, http.MethodPost, path, payload)
	if err != nil {
		return fmt.Errorf("submitting assignment: %w", err)
	}

	c.logger.Info("Submitted Canvas assignment", "courseId", courseID, "assignmentId", assignmentID, "userId", userID)
	return nil
}

// PostDiscussion posts a discussion entry in Canvas.
func (c *CanvasClient) PostDiscussion(ctx context.Context, courseID string, topicID, userID int, message string) error {
	payload := map[string]any{
		"message": message,
	}

	path := fmt.Sprintf("/courses/%s/discussion_topics/%d/entries?as_user_id=%d", courseID, topicID, userID)
	_, err := c.canvasRequest(ctx, http.MethodPost, path, payload)
	if err != nil {
		return fmt.Errorf("posting discussion: %w", err)
	}

	c.logger.Info("Posted Canvas discussion", "courseId", courseID, "topicId", topicID, "userId", userID)
	return nil
}

// SubmitQuiz submits quiz answers in Canvas.
func (c *CanvasClient) SubmitQuiz(ctx context.Context, courseID string, quizID, submissionID int, answers map[string]string) error {
	// Canvas quiz submission is a two-step process:
	// 1. Start the submission (already done, submissionID provided)
	// 2. Submit answers and complete

	payload := map[string]any{
		"attempt":          1,
		"validation_token": "",
		"quiz_questions":   answers,
	}

	path := fmt.Sprintf("/courses/%s/quizzes/%d/submissions/%d/complete", courseID, quizID, submissionID)
	_, err := c.canvasRequest(ctx, http.MethodPost, path, payload)
	if err != nil {
		return fmt.Errorf("submitting quiz: %w", err)
	}

	c.logger.Info("Submitted Canvas quiz", "courseId", courseID, "quizId", quizID)
	return nil
}
