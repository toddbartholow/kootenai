package canvas

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

// generateTestPrivateKey creates a test RSA private key for testing
func generateTestPrivateKey() string {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	keyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	return string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: keyBytes,
	}))
}

func TestNewDeepLinkingService(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	testKey := generateTestPrivateKey()

	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name:    "empty config",
			cfg:     Config{},
			wantErr: false,
		},
		{
			name: "with valid private key",
			cfg: Config{
				ToolPrivateKey: testKey,
			},
			wantErr: false,
		},
		{
			name: "with invalid private key",
			cfg: Config{
				ToolPrivateKey: "invalid-key",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewDeepLinkingService(tt.cfg, logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewDeepLinkingService() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildDeepLinkingResponse(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	cfg := Config{
		ClientID:       "test-client-id",
		ToolPrivateKey: generateTestPrivateKey(),
	}

	svc, err := NewDeepLinkingService(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	settings := &DeepLinkingSettings{
		DeepLinkReturnURL: "https://canvas.example.com/deep_link_response",
		AcceptTypes:       []string{"ltiResourceLink"},
		Data:              "opaque-data",
	}

	items := []ContentItem{
		{
			Type:  "ltiResourceLink",
			Title: "Test Lab",
			URL:   "https://example.com/lti/launch",
			Custom: map[string]string{
				"lab_template_id": "test-template-123",
			},
			LineItem: &LineItemClaim{
				ScoreMaximum: 100,
				Label:        "Test Lab",
				Tag:          "kootenai",
			},
		},
	}

	jwtToken, err := svc.BuildDeepLinkingResponse(
		context.Background(),
		settings,
		items,
		"deployment-123",
		"test-client-id",
		"https://canvas.instructure.com",
	)

	if err != nil {
		t.Fatalf("BuildDeepLinkingResponse() error = %v", err)
	}

	if jwtToken == "" {
		t.Error("BuildDeepLinkingResponse() returned empty token")
	}

	// Parse and verify token structure
	token, _, err := new(jwt.Parser).ParseUnverified(jwtToken, jwt.MapClaims{})
	if err != nil {
		t.Fatalf("Failed to parse token: %v", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatal("Invalid claims type")
	}

	// Verify message type
	if claims[ClaimMessageType] != MessageTypeLtiDeepLinkingRes {
		t.Errorf("Expected message type %s, got %v", MessageTypeLtiDeepLinkingRes, claims[ClaimMessageType])
	}

	// Verify deployment ID
	if claims[ClaimDeploymentID] != "deployment-123" {
		t.Errorf("Expected deployment ID deployment-123, got %v", claims[ClaimDeploymentID])
	}

	// Verify data is echoed back
	if claims[ClaimData] != "opaque-data" {
		t.Errorf("Expected data opaque-data, got %v", claims[ClaimData])
	}

	// Verify content items
	contentItems, ok := claims[ClaimContentItems].([]any)
	if !ok || len(contentItems) != 1 {
		t.Errorf("Expected 1 content item, got %v", claims[ClaimContentItems])
	}
}

func TestGetDeepLinkingSettings(t *testing.T) {
	tests := []struct {
		name    string
		claims  map[string]any
		wantErr bool
	}{
		{
			name:    "missing settings claim",
			claims:  map[string]any{},
			wantErr: true,
		},
		{
			name: "invalid settings format",
			claims: map[string]any{
				ClaimDeepLinkSettings: "not-a-map",
			},
			wantErr: true,
		},
		{
			name: "valid settings",
			claims: map[string]any{
				ClaimDeepLinkSettings: map[string]any{
					"deep_link_return_url":                 "https://canvas.example.com/return",
					"accept_types":                         []any{"ltiResourceLink"},
					"accept_presentation_document_targets": []any{"iframe", "window"},
					"accept_multiple":                      false,
					"auto_create":                          true,
					"data":                                 "test-data",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings, err := GetDeepLinkingSettings(tt.claims)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetDeepLinkingSettings() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && settings != nil {
				if settings.DeepLinkReturnURL != "https://canvas.example.com/return" {
					t.Errorf("Expected return URL https://canvas.example.com/return, got %s", settings.DeepLinkReturnURL)
				}
				if len(settings.AcceptTypes) != 1 || settings.AcceptTypes[0] != "ltiResourceLink" {
					t.Errorf("Expected accept types [ltiResourceLink], got %v", settings.AcceptTypes)
				}
				if settings.Data != "test-data" {
					t.Errorf("Expected data test-data, got %s", settings.Data)
				}
			}
		})
	}
}

func TestCreateResourceLinkItem(t *testing.T) {
	item := CreateResourceLinkItem(
		"Test Lab",
		"A test lab description",
		"https://example.com/lti/launch",
		"template-123",
		"resource-link-456",
		100.0,
	)

	if item.Type != "ltiResourceLink" {
		t.Errorf("Expected type ltiResourceLink, got %s", item.Type)
	}

	if item.Title != "Test Lab" {
		t.Errorf("Expected title Test Lab, got %s", item.Title)
	}

	if item.URL != "https://example.com/lti/launch" {
		t.Errorf("Expected URL https://example.com/lti/launch, got %s", item.URL)
	}

	if item.Custom["lab_template_id"] != "template-123" {
		t.Errorf("Expected lab_template_id template-123, got %s", item.Custom["lab_template_id"])
	}

	if item.LineItem == nil {
		t.Fatal("Expected LineItem to be set")
	}

	if item.LineItem.ScoreMaximum != 100.0 {
		t.Errorf("Expected ScoreMaximum 100, got %f", item.LineItem.ScoreMaximum)
	}
}

func TestBuildAutoSubmitForm(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	svc, _ := NewDeepLinkingService(Config{}, logger)

	html := svc.BuildAutoSubmitForm("https://canvas.example.com/return", "test-jwt-token")

	if html == "" {
		t.Error("BuildAutoSubmitForm() returned empty string")
	}

	// Check that the form contains the return URL
	if !strings.Contains(html, "https://canvas.example.com/return") {
		t.Error("Form does not contain return URL")
	}

	// Check that the form contains the JWT
	if !strings.Contains(html, "test-jwt-token") {
		t.Error("Form does not contain JWT token")
	}

	// Check that it auto-submits
	if !strings.Contains(html, ".submit()") {
		t.Error("Form does not have auto-submit script")
	}
}
