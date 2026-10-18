package canvas

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// -----------------------------------------------------------------------------
// NewGradeService Tests
// -----------------------------------------------------------------------------

func TestNewGradeService_Success(t *testing.T) {
	cfg := Config{
		CanvasURL:      "https://canvas.example.com",
		RequestTimeout: 30 * time.Second,
	}

	svc := NewGradeService(cfg, testLogger())
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestNewGradeService_WithPrivateKey(t *testing.T) {
	privateKey := generateTestRSAKey(t)
	keyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privatePEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: keyBytes,
	}))

	cfg := Config{
		CanvasURL:      "https://canvas.example.com",
		RequestTimeout: 30 * time.Second,
		ToolPrivateKey: privatePEM,
	}

	svc := NewGradeService(cfg, testLogger())
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if svc.privateKey == nil {
		t.Fatal("expected private key to be set")
	}
}

func TestNewGradeService_WithInvalidPrivateKey(t *testing.T) {
	cfg := Config{
		CanvasURL:      "https://canvas.example.com",
		RequestTimeout: 30 * time.Second,
		ToolPrivateKey: "invalid-pem-data",
	}

	// Should not panic, just log a warning
	svc := NewGradeService(cfg, testLogger())
	if svc == nil {
		t.Fatal("expected non-nil service even with invalid key")
	}
	if svc.privateKey != nil {
		t.Error("expected private key to be nil with invalid PEM")
	}
}

// -----------------------------------------------------------------------------
// SubmitScore Tests
// -----------------------------------------------------------------------------

func TestSubmitScore_WithAPIToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/scores") {
			t.Errorf("expected path to end with /scores, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-api-token" {
			t.Errorf("expected Bearer token, got %s", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/vnd.ims.lis.v1.score+json" {
			t.Errorf("unexpected content type: %s", r.Header.Get("Content-Type"))
		}

		// Verify body
		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		json.Unmarshal(body, &payload)

		if payload["userId"] != "user-123" {
			t.Errorf("expected userId user-123, got %v", payload["userId"])
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := Config{
		CanvasURL:      "https://canvas.example.com",
		APIAccessToken: "test-api-token",
		RequestTimeout: 30 * time.Second,
	}

	svc := NewGradeService(cfg, testLogger())

	score := Score{
		UserID:           "user-123",
		ScoreGiven:       85.0,
		ScoreMaximum:     100.0,
		Timestamp:        time.Now(),
		ActivityProgress: ActivityProgressCompleted,
		GradingProgress:  GradingProgressFullyGraded,
	}

	err := svc.SubmitScore(context.Background(), server.URL+"/line_items/1", score)
	if err != nil {
		t.Fatalf("SubmitScore failed: %v", err)
	}
}

func TestSubmitScore_WithComment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		json.Unmarshal(body, &payload)

		if payload["comment"] != "Great work!" {
			t.Errorf("expected comment 'Great work!', got %v", payload["comment"])
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := Config{
		APIAccessToken: "test-api-token",
		RequestTimeout: 30 * time.Second,
	}

	svc := NewGradeService(cfg, testLogger())

	score := Score{
		UserID:           "user-123",
		ScoreGiven:       90.0,
		ScoreMaximum:     100.0,
		Comment:          "Great work!",
		Timestamp:        time.Now(),
		ActivityProgress: ActivityProgressCompleted,
		GradingProgress:  GradingProgressFullyGraded,
	}

	err := svc.SubmitScore(context.Background(), server.URL+"/line_items/1", score)
	if err != nil {
		t.Fatalf("SubmitScore failed: %v", err)
	}
}

func TestSubmitScore_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("Access denied"))
	}))
	defer server.Close()

	cfg := Config{
		APIAccessToken: "test-api-token",
		RequestTimeout: 30 * time.Second,
	}

	svc := NewGradeService(cfg, testLogger())

	score := Score{
		UserID:           "user-123",
		Timestamp:        time.Now(),
		ActivityProgress: ActivityProgressCompleted,
		GradingProgress:  GradingProgressFullyGraded,
	}

	err := svc.SubmitScore(context.Background(), server.URL+"/line_items/1", score)
	if err == nil {
		t.Fatal("expected error for HTTP 403")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("expected error to contain 403, got: %v", err)
	}
}

func TestSubmitScore_NoToken(t *testing.T) {
	cfg := Config{
		RequestTimeout: 30 * time.Second,
		// No APIAccessToken and no private key
	}

	svc := NewGradeService(cfg, testLogger())

	score := Score{
		UserID:           "user-123",
		Timestamp:        time.Now(),
		ActivityProgress: ActivityProgressCompleted,
		GradingProgress:  GradingProgressFullyGraded,
	}

	err := svc.SubmitScore(context.Background(), "https://canvas.example.com/line_items/1", score)
	if err == nil {
		t.Fatal("expected error when no token available")
	}
}

// -----------------------------------------------------------------------------
// GetLineItem Tests
// -----------------------------------------------------------------------------

func TestGetLineItem_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.Header.Get("Accept") != "application/vnd.ims.lis.v2.lineitem+json" {
			t.Errorf("unexpected Accept header: %s", r.Header.Get("Accept"))
		}

		lineItem := LineItem{
			ID:           "https://canvas.example.com/line_items/1",
			ScoreMaximum: 100,
			Label:        "Lab 1",
			Tag:          "lab",
		}
		json.NewEncoder(w).Encode(lineItem)
	}))
	defer server.Close()

	cfg := Config{
		APIAccessToken: "test-api-token",
		RequestTimeout: 30 * time.Second,
	}

	svc := NewGradeService(cfg, testLogger())

	lineItem, err := svc.GetLineItem(context.Background(), server.URL+"/line_items/1")
	if err != nil {
		t.Fatalf("GetLineItem failed: %v", err)
	}
	if lineItem.Label != "Lab 1" {
		t.Errorf("expected label 'Lab 1', got %s", lineItem.Label)
	}
	if lineItem.ScoreMaximum != 100 {
		t.Errorf("expected score max 100, got %f", lineItem.ScoreMaximum)
	}
}

func TestGetLineItem_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Line item not found"))
	}))
	defer server.Close()

	cfg := Config{
		APIAccessToken: "test-api-token",
		RequestTimeout: 30 * time.Second,
	}

	svc := NewGradeService(cfg, testLogger())

	_, err := svc.GetLineItem(context.Background(), server.URL+"/line_items/999")
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

// -----------------------------------------------------------------------------
// ListLineItems Tests
// -----------------------------------------------------------------------------

func TestListLineItems_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/vnd.ims.lis.v2.lineitemcontainer+json" {
			t.Errorf("unexpected Accept header: %s", r.Header.Get("Accept"))
		}

		lineItems := []LineItem{
			{ID: "1", Label: "Lab 1", ScoreMaximum: 100},
			{ID: "2", Label: "Lab 2", ScoreMaximum: 50},
		}
		json.NewEncoder(w).Encode(lineItems)
	}))
	defer server.Close()

	cfg := Config{
		APIAccessToken: "test-api-token",
		RequestTimeout: 30 * time.Second,
	}

	svc := NewGradeService(cfg, testLogger())

	items, err := svc.ListLineItems(context.Background(), server.URL+"/line_items")
	if err != nil {
		t.Fatalf("ListLineItems failed: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestListLineItems_Empty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]LineItem{})
	}))
	defer server.Close()

	cfg := Config{
		APIAccessToken: "test-api-token",
		RequestTimeout: 30 * time.Second,
	}

	svc := NewGradeService(cfg, testLogger())

	items, err := svc.ListLineItems(context.Background(), server.URL+"/line_items")
	if err != nil {
		t.Fatalf("ListLineItems failed: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

// -----------------------------------------------------------------------------
// CreateLineItem Tests
// -----------------------------------------------------------------------------

func TestCreateLineItem_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/vnd.ims.lis.v2.lineitem+json" {
			t.Errorf("unexpected Content-Type: %s", r.Header.Get("Content-Type"))
		}

		body, _ := io.ReadAll(r.Body)
		var input LineItem
		json.Unmarshal(body, &input)

		// Return the created item with an ID
		created := input
		created.ID = "https://canvas.example.com/line_items/123"

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(created)
	}))
	defer server.Close()

	cfg := Config{
		APIAccessToken: "test-api-token",
		RequestTimeout: 30 * time.Second,
	}

	svc := NewGradeService(cfg, testLogger())

	lineItem := LineItem{
		Label:        "New Lab",
		ScoreMaximum: 100,
		Tag:          "lab",
	}

	created, err := svc.CreateLineItem(context.Background(), server.URL+"/line_items", lineItem)
	if err != nil {
		t.Fatalf("CreateLineItem failed: %v", err)
	}
	if created.ID == "" {
		t.Error("expected created line item to have ID")
	}
	if created.Label != "New Lab" {
		t.Errorf("expected label 'New Lab', got %s", created.Label)
	}
}

func TestCreateLineItem_Conflict(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte("Line item already exists"))
	}))
	defer server.Close()

	cfg := Config{
		APIAccessToken: "test-api-token",
		RequestTimeout: 30 * time.Second,
	}

	svc := NewGradeService(cfg, testLogger())

	lineItem := LineItem{
		Label:        "Existing Lab",
		ScoreMaximum: 100,
	}

	_, err := svc.CreateLineItem(context.Background(), server.URL+"/line_items", lineItem)
	if err == nil {
		t.Fatal("expected error for conflict")
	}
}

// -----------------------------------------------------------------------------
// SubmitGrade Tests
// -----------------------------------------------------------------------------

func TestSubmitGrade_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		json.Unmarshal(body, &payload)

		// Verify the score is calculated correctly (85% of 100 = 85)
		scoreGiven := payload["scoreGiven"].(float64)
		if scoreGiven != 85.0 {
			t.Errorf("expected scoreGiven 85.0, got %f", scoreGiven)
		}

		if payload["activityProgress"] != ActivityProgressCompleted {
			t.Errorf("expected activityProgress Completed, got %v", payload["activityProgress"])
		}
		if payload["gradingProgress"] != GradingProgressFullyGraded {
			t.Errorf("expected gradingProgress FullyGraded, got %v", payload["gradingProgress"])
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := Config{
		APIAccessToken: "test-api-token",
		RequestTimeout: 30 * time.Second,
	}

	svc := NewGradeService(cfg, testLogger())

	err := svc.SubmitGrade(context.Background(), server.URL+"/line_items/1", "user-123", 85.0, 100.0, "")
	if err != nil {
		t.Fatalf("SubmitGrade failed: %v", err)
	}
}

func TestSubmitGrade_WithComment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		json.Unmarshal(body, &payload)

		if payload["comment"] != "Excellent work!" {
			t.Errorf("expected comment 'Excellent work!', got %v", payload["comment"])
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := Config{
		APIAccessToken: "test-api-token",
		RequestTimeout: 30 * time.Second,
	}

	svc := NewGradeService(cfg, testLogger())

	err := svc.SubmitGrade(context.Background(), server.URL+"/line_items/1", "user-123", 100.0, 100.0, "Excellent work!")
	if err != nil {
		t.Fatalf("SubmitGrade failed: %v", err)
	}
}

// -----------------------------------------------------------------------------
// getAccessToken Tests
// -----------------------------------------------------------------------------

func TestGetAccessToken_WithAPIToken(t *testing.T) {
	cfg := Config{
		APIAccessToken: "direct-api-token",
		RequestTimeout: 30 * time.Second,
	}

	svc := NewGradeService(cfg, testLogger())

	token, err := svc.getAccessToken(context.Background())
	if err != nil {
		t.Fatalf("getAccessToken failed: %v", err)
	}
	if token != "direct-api-token" {
		t.Errorf("expected direct-api-token, got %s", token)
	}
}

func TestGetAccessToken_CachedToken(t *testing.T) {
	cfg := Config{
		RequestTimeout: 30 * time.Second,
	}

	svc := NewGradeService(cfg, testLogger())

	// Pre-populate cache
	svc.tokenCache = &tokenCache{
		token:     "cached-token",
		expiresAt: time.Now().Add(time.Hour),
	}

	token, err := svc.getAccessToken(context.Background())
	if err != nil {
		t.Fatalf("getAccessToken failed: %v", err)
	}
	if token != "cached-token" {
		t.Errorf("expected cached-token, got %s", token)
	}
}

func TestGetAccessToken_ExpiredCache(t *testing.T) {
	cfg := Config{
		RequestTimeout: 30 * time.Second,
		// No APIAccessToken and no private key
	}

	svc := NewGradeService(cfg, testLogger())

	// Pre-populate expired cache
	svc.tokenCache = &tokenCache{
		token:     "expired-token",
		expiresAt: time.Now().Add(-time.Hour), // Expired
	}

	// Should try to refresh and fail (no private key)
	_, err := svc.getAccessToken(context.Background())
	if err == nil {
		t.Fatal("expected error when token expired and no refresh mechanism")
	}
}

// -----------------------------------------------------------------------------
// requestAccessToken Tests
// -----------------------------------------------------------------------------

func TestRequestAccessToken_NoPrivateKey(t *testing.T) {
	cfg := Config{
		TokenURL:       "https://canvas.example.com/token",
		RequestTimeout: 30 * time.Second,
	}

	svc := NewGradeService(cfg, testLogger())

	_, _, err := svc.requestAccessToken(context.Background())
	if err == nil {
		t.Fatal("expected error when no private key")
	}
	if !strings.Contains(err.Error(), "private key not configured") {
		t.Errorf("expected 'private key not configured' error, got: %v", err)
	}
}

func TestRequestAccessToken_NoTokenURL(t *testing.T) {
	privateKey := generateTestRSAKey(t)
	keyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privatePEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: keyBytes,
	}))

	cfg := Config{
		ToolPrivateKey: privatePEM,
		RequestTimeout: 30 * time.Second,
		// No TokenURL
	}

	svc := NewGradeService(cfg, testLogger())

	_, _, err := svc.requestAccessToken(context.Background())
	if err == nil {
		t.Fatal("expected error when no token URL")
	}
	if !strings.Contains(err.Error(), "token URL not configured") {
		t.Errorf("expected 'token URL not configured' error, got: %v", err)
	}
}

func TestRequestAccessToken_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("unexpected Content-Type: %s", r.Header.Get("Content-Type"))
		}

		r.ParseForm()
		if r.Form.Get("grant_type") != "client_credentials" {
			t.Errorf("expected grant_type=client_credentials, got %s", r.Form.Get("grant_type"))
		}
		if r.Form.Get("client_assertion") == "" {
			t.Error("expected client_assertion to be set")
		}

		response := map[string]interface{}{
			"access_token": "new-access-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
			"scope":        "https://purl.imsglobal.org/spec/lti-ags/scope/score",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	privateKey := generateTestRSAKey(t)
	keyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privatePEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: keyBytes,
	}))

	cfg := Config{
		ClientID:       "test-client-id",
		TokenURL:       server.URL,
		ToolPrivateKey: privatePEM,
		RequestTimeout: 30 * time.Second,
	}

	svc := NewGradeService(cfg, testLogger())

	token, expiresIn, err := svc.requestAccessToken(context.Background())
	if err != nil {
		t.Fatalf("requestAccessToken failed: %v", err)
	}
	if token != "new-access-token" {
		t.Errorf("expected new-access-token, got %s", token)
	}
	if expiresIn != 3600 {
		t.Errorf("expected expiresIn 3600, got %d", expiresIn)
	}
}

func TestRequestAccessToken_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal server error"))
	}))
	defer server.Close()

	privateKey := generateTestRSAKey(t)
	keyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privatePEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: keyBytes,
	}))

	cfg := Config{
		ClientID:       "test-client-id",
		TokenURL:       server.URL,
		ToolPrivateKey: privatePEM,
		RequestTimeout: 30 * time.Second,
	}

	svc := NewGradeService(cfg, testLogger())

	_, _, err := svc.requestAccessToken(context.Background())
	if err == nil {
		t.Fatal("expected error for server error")
	}
}

func TestRequestAccessToken_EmptyAccessToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"access_token": "", // Empty token
			"token_type":   "Bearer",
			"expires_in":   3600,
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	privateKey := generateTestRSAKey(t)
	keyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privatePEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: keyBytes,
	}))

	cfg := Config{
		ClientID:       "test-client-id",
		TokenURL:       server.URL,
		ToolPrivateKey: privatePEM,
		RequestTimeout: 30 * time.Second,
	}

	svc := NewGradeService(cfg, testLogger())

	_, _, err := svc.requestAccessToken(context.Background())
	if err == nil {
		t.Fatal("expected error for empty access token")
	}
}

// -----------------------------------------------------------------------------
// GradeSyncWorker Tests
// -----------------------------------------------------------------------------

func TestNewGradeSyncWorker(t *testing.T) {
	cfg := Config{
		APIAccessToken: "test-token",
		RequestTimeout: 30 * time.Second,
	}

	svc := NewGradeService(cfg, testLogger())
	worker := NewGradeSyncWorker(svc, testLogger())

	if worker == nil {
		t.Fatal("expected non-nil worker")
	}
}

func TestProcessGradeSync_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := Config{
		APIAccessToken: "test-token",
		RequestTimeout: 30 * time.Second,
	}

	svc := NewGradeService(cfg, testLogger())
	worker := NewGradeSyncWorker(svc, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))

	req := GradeSyncRequest{
		SessionID:    "session-123",
		UserID:       "user-123",
		LineItemURL:  server.URL + "/line_items/1",
		EarnedPoints: 85,
		MaxPoints:    100,
		Percentage:   85.0,
		Comment:      "Good job!",
	}

	err := worker.ProcessGradeSync(context.Background(), req)
	if err != nil {
		t.Fatalf("ProcessGradeSync failed: %v", err)
	}
}

func TestProcessGradeSync_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("Access denied"))
	}))
	defer server.Close()

	cfg := Config{
		APIAccessToken: "test-token",
		RequestTimeout: 30 * time.Second,
	}

	svc := NewGradeService(cfg, testLogger())
	worker := NewGradeSyncWorker(svc, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))

	req := GradeSyncRequest{
		SessionID:   "session-123",
		UserID:      "user-123",
		LineItemURL: server.URL + "/line_items/1",
		Percentage:  85.0,
		MaxPoints:   100,
	}

	err := worker.ProcessGradeSync(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for failed grade sync")
	}
}
