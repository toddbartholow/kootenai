// Package cli provides CLI command implementations
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is an HTTP client for the labctl API
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// Config holds CLI configuration
type Config struct {
	APIBaseURL string        `yaml:"api_base_url"`
	Timeout    time.Duration `yaml:"timeout"`
}

// DefaultConfig returns default CLI configuration
func DefaultConfig() Config {
	return Config{
		APIBaseURL: "http://localhost:8080",
		Timeout:    30 * time.Second,
	}
}

// NewClient creates a new API client
func NewClient(cfg Config) *Client {
	return &Client{
		baseURL: cfg.APIBaseURL,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// request performs an HTTP request and decodes the response
func (c *Client) request(ctx context.Context, method, path string, body, result any) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error != "" {
			return fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Error)
		}
		return fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}

	return nil
}

// Get performs a GET request
func (c *Client) Get(ctx context.Context, path string, result any) error {
	return c.request(ctx, "GET", path, nil, result)
}

// Post performs a POST request
func (c *Client) Post(ctx context.Context, path string, body, result any) error {
	return c.request(ctx, "POST", path, body, result)
}

// Delete performs a DELETE request
func (c *Client) Delete(ctx context.Context, path string, result any) error {
	return c.request(ctx, "DELETE", path, nil, result)
}

// Health checks API health
func (c *Client) Health(ctx context.Context) error {
	var result map[string]string
	if err := c.Get(ctx, "/health", &result); err != nil {
		return err
	}
	if result["status"] != "ok" {
		return fmt.Errorf("unhealthy: %s", result["status"])
	}
	return nil
}
