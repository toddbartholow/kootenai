package canvas

import (
	"bytes"
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// GradeService handles grade passback to Canvas
type GradeService struct {
	config     Config
	privateKey *rsa.PrivateKey
	httpClient *http.Client
	tokenCache *tokenCache
	logger     *slog.Logger
}

// tokenCache stores OAuth2 access tokens
type tokenCache struct {
	token     string
	expiresAt time.Time
}

// NewGradeService creates a new grade service
func NewGradeService(cfg Config, logger *slog.Logger) *GradeService {
	svc := &GradeService{
		config: cfg,
		httpClient: &http.Client{
			Timeout: cfg.RequestTimeout,
		},
		logger: logger,
	}

	// Parse private key if provided for OAuth2 JWT assertions
	if cfg.ToolPrivateKey != "" {
		key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(cfg.ToolPrivateKey))
		if err != nil {
			logger.Warn("Failed to parse tool private key for grade service", "error", err)
		} else {
			svc.privateKey = key
			logger.Debug("Grade service initialized with private key for OAuth2")
		}
	}

	return svc
}

// SubmitScore submits a score to Canvas via AGS
func (s *GradeService) SubmitScore(ctx context.Context, lineItemURL string, score Score) error {
	token, err := s.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("getting access token: %w", err)
	}

	// Build score submission URL
	scoreURL := lineItemURL + "/scores"

	// Prepare score payload
	payload := map[string]any{
		"userId":           score.UserID,
		"activityProgress": score.ActivityProgress,
		"gradingProgress":  score.GradingProgress,
		"timestamp":        score.Timestamp.UTC().Format(time.RFC3339),
	}

	if score.ScoreGiven > 0 {
		payload["scoreGiven"] = score.ScoreGiven
		payload["scoreMaximum"] = score.ScoreMaximum
	}

	if score.Comment != "" {
		payload["comment"] = score.Comment
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling score: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", scoreURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/vnd.ims.lis.v1.score+json")
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("score submission failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	s.logger.Info("Score submitted successfully",
		"lineItem", lineItemURL,
		"userId", score.UserID,
		"score", score.ScoreGiven,
	)

	return nil
}

// GetLineItem retrieves a line item (assignment) from Canvas
func (s *GradeService) GetLineItem(ctx context.Context, lineItemURL string) (*LineItem, error) {
	token, err := s.getAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting access token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", lineItemURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.ims.lis.v2.lineitem+json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get line item failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var lineItem LineItem
	if err := json.NewDecoder(resp.Body).Decode(&lineItem); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &lineItem, nil
}

// ListLineItems lists all line items in a context
func (s *GradeService) ListLineItems(ctx context.Context, lineItemsURL string) ([]LineItem, error) {
	token, err := s.getAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting access token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", lineItemsURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.ims.lis.v2.lineitemcontainer+json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list line items failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var lineItems []LineItem
	if err := json.NewDecoder(resp.Body).Decode(&lineItems); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return lineItems, nil
}

// CreateLineItem creates a new line item (gradebook column)
func (s *GradeService) CreateLineItem(ctx context.Context, lineItemsURL string, lineItem LineItem) (*LineItem, error) {
	token, err := s.getAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting access token: %w", err)
	}

	body, err := json.Marshal(lineItem)
	if err != nil {
		return nil, fmt.Errorf("marshaling line item: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", lineItemsURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/vnd.ims.lis.v2.lineitem+json")
	req.Header.Set("Accept", "application/vnd.ims.lis.v2.lineitem+json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create line item failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var created LineItem
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &created, nil
}

// SubmitGrade is a convenience method that submits a percentage grade
func (s *GradeService) SubmitGrade(ctx context.Context, lineItemURL, userID string, percentage, maxPoints float64, comment string) error {
	score := Score{
		UserID:           userID,
		ScoreGiven:       percentage / 100.0 * maxPoints,
		ScoreMaximum:     maxPoints,
		Comment:          comment,
		Timestamp:        time.Now().UTC(),
		ActivityProgress: ActivityProgressCompleted,
		GradingProgress:  GradingProgressFullyGraded,
	}

	return s.SubmitScore(ctx, lineItemURL, score)
}

// getAccessToken retrieves or refreshes the OAuth2 access token
func (s *GradeService) getAccessToken(ctx context.Context) (string, error) {
	// If we have a direct API token, use it
	if s.config.APIAccessToken != "" {
		return s.config.APIAccessToken, nil
	}

	// Check cache
	if s.tokenCache != nil && time.Now().Before(s.tokenCache.expiresAt) {
		return s.tokenCache.token, nil
	}

	// Request new token using client credentials
	token, expiresIn, err := s.requestAccessToken(ctx)
	if err != nil {
		return "", err
	}

	// Cache token
	s.tokenCache = &tokenCache{
		token:     token,
		expiresAt: time.Now().Add(time.Duration(expiresIn-60) * time.Second), // Refresh 60s early
	}

	return token, nil
}

// requestAccessToken requests a new access token from Canvas using OAuth2 JWT client assertion
func (s *GradeService) requestAccessToken(ctx context.Context) (token string, expiresIn int, err error) {
	// Check if we have the necessary configuration for OAuth2
	if s.privateKey == nil {
		return "", 0, fmt.Errorf("private key not configured for OAuth2 token flow")
	}
	if s.config.TokenURL == "" {
		return "", 0, fmt.Errorf("token URL not configured")
	}

	// Create JWT assertion for client credentials grant
	// Per LTI 1.3 spec and OAuth2 client assertion (RFC 7523)
	now := time.Now()
	jti := uuid.New().String()

	claims := jwt.MapClaims{
		"iss": s.config.ClientID,               // Client ID (tool)
		"sub": s.config.ClientID,               // Client ID (tool)
		"aud": s.config.TokenURL,               // Token endpoint URL
		"iat": now.Unix(),                      // Issued at
		"exp": now.Add(5 * time.Minute).Unix(), // Expires in 5 minutes
		"jti": jti,                             // Unique token ID
	}

	// Create and sign the JWT
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signedJWT, err := jwtToken.SignedString(s.privateKey)
	if err != nil {
		return "", 0, fmt.Errorf("signing JWT assertion: %w", err)
	}

	// Prepare form data for token request
	formData := url.Values{}
	formData.Set("grant_type", "client_credentials")
	formData.Set("client_assertion_type", "urn:ietf:params:oauth:client-assertion-type:jwt-bearer")
	formData.Set("client_assertion", signedJWT)
	// Canvas AGS scopes
	formData.Set("scope", "https://purl.imsglobal.org/spec/lti-ags/scope/lineitem https://purl.imsglobal.org/spec/lti-ags/scope/lineitem.readonly https://purl.imsglobal.org/spec/lti-ags/scope/result.readonly https://purl.imsglobal.org/spec/lti-ags/scope/score")

	// Make the token request
	req, err := http.NewRequestWithContext(ctx, "POST", s.config.TokenURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return "", 0, fmt.Errorf("creating token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("sending token request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse the token response
	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
		Scope       string `json:"scope"`
	}

	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", 0, fmt.Errorf("parsing token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return "", 0, fmt.Errorf("empty access token in response")
	}

	s.logger.Info("OAuth2 access token obtained",
		"tokenType", tokenResp.TokenType,
		"expiresIn", tokenResp.ExpiresIn,
		"scope", tokenResp.Scope,
	)

	return tokenResp.AccessToken, tokenResp.ExpiresIn, nil
}

// -----------------------------------------------------------------------------
// Grade Sync Worker
// -----------------------------------------------------------------------------

// GradeSyncWorker processes grade sync queue entries
type GradeSyncWorker struct {
	gradeService *GradeService
	logger       *slog.Logger
}

// NewGradeSyncWorker creates a new grade sync worker
func NewGradeSyncWorker(gs *GradeService, logger *slog.Logger) *GradeSyncWorker {
	return &GradeSyncWorker{
		gradeService: gs,
		logger:       logger,
	}
}

// GradeSyncRequest represents a grade sync request
type GradeSyncRequest struct {
	SessionID    string
	UserID       string
	LineItemURL  string
	EarnedPoints int
	MaxPoints    int
	Percentage   float64
	Comment      string
}

// ProcessGradeSync processes a single grade sync request
func (w *GradeSyncWorker) ProcessGradeSync(ctx context.Context, req GradeSyncRequest) error {
	w.logger.Info("Processing grade sync",
		"sessionId", req.SessionID,
		"userId", req.UserID,
		"percentage", req.Percentage,
	)

	err := w.gradeService.SubmitGrade(
		ctx,
		req.LineItemURL,
		req.UserID,
		req.Percentage,
		float64(req.MaxPoints),
		req.Comment,
	)

	if err != nil {
		w.logger.Error("Grade sync failed",
			"sessionId", req.SessionID,
			"error", err,
		)
		return err
	}

	w.logger.Info("Grade sync completed",
		"sessionId", req.SessionID,
		"userId", req.UserID,
	)

	return nil
}
