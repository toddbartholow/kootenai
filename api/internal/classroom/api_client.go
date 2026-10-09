package classroom

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// LabAPIClient calls the Kootenai API for pod/session operations.
type LabAPIClient struct {
	baseURL    string
	authToken  string
	httpClient *http.Client
	logger     *slog.Logger
}

// NewLabAPIClient creates a new lab API client.
func NewLabAPIClient(baseURL string, logger *slog.Logger) *LabAPIClient {
	return &LabAPIClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		logger: logger,
	}
}

// SetAuthToken sets the bearer token for API requests.
func (c *LabAPIClient) SetAuthToken(token string) {
	c.authToken = token
}

// PodInfo holds pod status information returned by the API.
type PodInfo struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Template string `json:"template"`
}

// SessionProgress holds session progress from the API.
type SessionProgress struct {
	SessionID    string               `json:"id"`
	Status       string               `json:"status"`
	EarnedPoints int                  `json:"earnedPoints"`
	TotalPoints  int                  `json:"totalPoints"`
	Percentage   int                  `json:"percentage"`
	Passed       bool                 `json:"passed"`
	Checkpoints  []CheckpointProgress `json:"checkpoints,omitempty"`
}

// CheckpointProgress holds individual checkpoint status.
type CheckpointProgress struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	Points       int    `json:"points"`
	EarnedPoints int    `json:"earnedPoints"`
}

// CreatePod creates a new pod via the API and returns the pod ID.
func (c *LabAPIClient) CreatePod(ctx context.Context, labTemplate, ownerID string) (string, error) {
	body := fmt.Sprintf(`{"template":%q,"owner":%q}`, labTemplate, ownerID)
	resp, err := c.doRequest(ctx, http.MethodPost, "/pods", body)
	if err != nil {
		return "", fmt.Errorf("creating pod: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("create pod failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ID        string `json:"id"`
		PodID     string `json:"podId"`
		RequestID string `json:"requestId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decoding create pod response: %w", err)
	}

	podID := result.ID
	if podID == "" {
		podID = result.PodID
	}
	if podID == "" {
		podID = result.RequestID
	}

	c.logger.Info("Pod creation initiated", "podId", podID, "template", labTemplate)
	return podID, nil
}

// WaitForPodReady polls the pod status until it's ready or the timeout expires.
func (c *LabAPIClient) WaitForPodReady(ctx context.Context, podID string, timeout time.Duration) (*PodInfo, error) {
	deadline := time.Now().Add(timeout)
	pollInterval := 5 * time.Second

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		info, err := c.GetPod(ctx, podID)
		switch {
		case err != nil:
			c.logger.Debug("Pod not yet available", "podId", podID, "error", err)
		case info.Status == "running" || info.Status == "ready":
			return info, nil
		case info.Status == "failed" || info.Status == "error":
			return nil, fmt.Errorf("pod %s entered %s state", podID, info.Status)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}
	}

	return nil, fmt.Errorf("pod %s did not become ready within %v", podID, timeout)
}

// GetPod retrieves pod info.
func (c *LabAPIClient) GetPod(ctx context.Context, podID string) (*PodInfo, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/pods/"+podID, "")
	if err != nil {
		return nil, fmt.Errorf("getting pod: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get pod failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var info PodInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decoding pod response: %w", err)
	}
	return &info, nil
}

// CreateSession creates a lab session for a pod.
func (c *LabAPIClient) CreateSession(ctx context.Context, podID, userID, labTemplate string) (string, error) {
	body := fmt.Sprintf(`{"podId":%q,"userId":%q,"labTemplate":%q}`, podID, userID, labTemplate)
	resp, err := c.doRequest(ctx, http.MethodPost, "/sessions", body)
	if err != nil {
		return "", fmt.Errorf("creating session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("create session failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ID        string `json:"id"`
		SessionID string `json:"sessionId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decoding session response: %w", err)
	}

	sessionID := result.ID
	if sessionID == "" {
		sessionID = result.SessionID
	}

	return sessionID, nil
}

// GetSessionProgress retrieves session progress including checkpoint statuses.
func (c *LabAPIClient) GetSessionProgress(ctx context.Context, sessionID string) (*SessionProgress, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/sessions/"+sessionID, "")
	if err != nil {
		return nil, fmt.Errorf("getting session progress: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get session failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var progress SessionProgress
	if err := json.NewDecoder(resp.Body).Decode(&progress); err != nil {
		return nil, fmt.Errorf("decoding session progress: %w", err)
	}
	return &progress, nil
}

// DeletePod destroys a pod.
func (c *LabAPIClient) DeletePod(ctx context.Context, podID string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, "/pods/"+podID, "")
	if err != nil {
		return fmt.Errorf("deleting pod: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete pod failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	c.logger.Info("Pod deleted", "podId", podID)
	return nil
}

func (c *LabAPIClient) doRequest(ctx context.Context, method, path, body string) (*http.Response, error) {
	url := c.baseURL + path
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	return c.httpClient.Do(req)
}
