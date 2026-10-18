// Package canvas provides Canvas LMS LTI 1.3 integration
package canvas

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// LTI Deep Linking claim URIs
const (
	ClaimMessageType      = "https://purl.imsglobal.org/spec/lti/claim/message_type"
	ClaimVersion          = "https://purl.imsglobal.org/spec/lti/claim/version"
	ClaimDeploymentID     = "https://purl.imsglobal.org/spec/lti/claim/deployment_id"
	ClaimDeepLinkSettings = "https://purl.imsglobal.org/spec/lti-dl/claim/deep_linking_settings"
	ClaimContentItems     = "https://purl.imsglobal.org/spec/lti-dl/claim/content_items"
	ClaimData             = "https://purl.imsglobal.org/spec/lti-dl/claim/data"
)

// Message type constants
const (
	MessageTypeLtiResourceLink   = "LtiResourceLinkRequest"
	MessageTypeLtiDeepLinking    = "LtiDeepLinkingRequest"
	MessageTypeLtiDeepLinkingRes = "LtiDeepLinkingResponse"
)

// DeepLinkingSettings represents the deep linking settings from an LTI launch
type DeepLinkingSettings struct {
	DeepLinkReturnURL         string   `json:"deep_link_return_url"`
	AcceptTypes               []string `json:"accept_types"`
	AcceptPresentationTargets []string `json:"accept_presentation_document_targets"`
	AcceptMultiple            bool     `json:"accept_multiple"`
	AutoCreate                bool     `json:"auto_create"`
	Title                     string   `json:"title,omitempty"`
	Text                      string   `json:"text,omitempty"`
	Data                      string   `json:"data,omitempty"` // Opaque data to echo back
}

// ContentItem represents an LTI resource link content item for deep linking response
type ContentItem struct {
	Type     string            `json:"type"` // "ltiResourceLink"
	Title    string            `json:"title"`
	Text     string            `json:"text,omitempty"`
	URL      string            `json:"url,omitempty"`
	Custom   map[string]string `json:"custom,omitempty"`
	LineItem *LineItemClaim    `json:"lineItem,omitempty"`
	Window   *WindowClaim      `json:"window,omitempty"`
	Iframe   *IframeClaim      `json:"iframe,omitempty"`
}

// LineItemClaim for automatic gradebook column creation
type LineItemClaim struct {
	ScoreMaximum float64 `json:"scoreMaximum"`
	Label        string  `json:"label,omitempty"`
	Tag          string  `json:"tag,omitempty"`
	ResourceID   string  `json:"resourceId,omitempty"`
}

// WindowClaim for window target specification
type WindowClaim struct {
	TargetName string `json:"targetName,omitempty"`
}

// IframeClaim for iframe dimensions
type IframeClaim struct {
	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`
}

// DeepLinkingService handles LTI deep linking operations
type DeepLinkingService struct {
	config     Config
	privateKey *rsa.PrivateKey
	logger     *slog.Logger
}

// NewDeepLinkingService creates a new deep linking service
func NewDeepLinkingService(cfg Config, logger *slog.Logger) (*DeepLinkingService, error) {
	svc := &DeepLinkingService{
		config: cfg,
		logger: logger,
	}

	// Parse private key for signing responses
	if cfg.ToolPrivateKey != "" {
		key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(cfg.ToolPrivateKey))
		if err != nil {
			return nil, fmt.Errorf("parsing private key: %w", err)
		}
		svc.privateKey = key
	}

	return svc, nil
}

// BuildDeepLinkingResponse creates a signed JWT for deep linking response
func (s *DeepLinkingService) BuildDeepLinkingResponse(
	ctx context.Context,
	settings *DeepLinkingSettings,
	items []ContentItem,
	deploymentID string,
	issuer string,
	audience string,
) (string, error) {
	if s.privateKey == nil {
		return "", fmt.Errorf("private key not configured for deep linking responses")
	}

	now := time.Now()
	nonce := uuid.New().String()

	claims := jwt.MapClaims{
		"iss":             s.config.ClientID, // Tool client ID
		"aud":             audience,          // Canvas issuer
		"iat":             now.Unix(),
		"exp":             now.Add(5 * time.Minute).Unix(),
		"nonce":           nonce,
		ClaimMessageType:  MessageTypeLtiDeepLinkingRes,
		ClaimVersion:      "1.3.0",
		ClaimDeploymentID: deploymentID,
		ClaimContentItems: items,
	}

	// Echo back opaque data if provided
	if settings.Data != "" {
		claims[ClaimData] = settings.Data
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	// Set key ID in header
	token.Header["kid"] = "kootenai-lti-key"

	signedToken, err := token.SignedString(s.privateKey)
	if err != nil {
		return "", fmt.Errorf("signing token: %w", err)
	}

	s.logger.Info("Built deep linking response",
		"nonce", nonce,
		"itemCount", len(items),
		"deploymentId", deploymentID,
	)

	return signedToken, nil
}

// BuildAutoSubmitForm creates an HTML form that auto-submits the deep linking response
func (s *DeepLinkingService) BuildAutoSubmitForm(returnURL, jwt string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>Returning to Canvas...</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            display: flex;
            align-items: center;
            justify-content: center;
            height: 100vh;
            margin: 0;
            background: #f5f5f5;
        }
        .loading {
            text-align: center;
            color: #666;
        }
        .spinner {
            width: 40px;
            height: 40px;
            border: 3px solid #e0e0e0;
            border-top-color: #667eea;
            border-radius: 50%%;
            animation: spin 1s linear infinite;
            margin: 0 auto 16px;
        }
        @keyframes spin { to { transform: rotate(360deg); } }
    </style>
</head>
<body>
    <div class="loading">
        <div class="spinner"></div>
        <p>Creating assignment...</p>
    </div>
    <form id="deep-link-response" action="%s" method="POST" style="display:none;">
        <input type="hidden" name="JWT" value="%s">
    </form>
    <script>
        document.getElementById('deep-link-response').submit();
    </script>
</body>
</html>`, returnURL, jwt)
}

// CreateResourceLinkItem creates a content item for a lab template
func CreateResourceLinkItem(
	title string,
	description string,
	launchURL string,
	templateID string,
	resourceLinkID string,
	maxPoints float64,
) ContentItem {
	return ContentItem{
		Type:  "ltiResourceLink",
		Title: title,
		Text:  description,
		URL:   launchURL,
		Custom: map[string]string{
			"lab_template_id":  templateID,
			"resource_link_id": resourceLinkID,
		},
		LineItem: &LineItemClaim{
			ScoreMaximum: maxPoints,
			Label:        title,
			Tag:          "kootenai",
			ResourceID:   resourceLinkID,
		},
	}
}

// GetMessageType returns the LTI message type from a launch request
func (l *LTILaunchRequest) GetMessageType() string {
	return l.MessageType
}

// IsDeepLinkingRequest returns true if this is a deep linking request
func (l *LTILaunchRequest) IsDeepLinkingRequest() bool {
	return l.MessageType == MessageTypeLtiDeepLinking
}

// GetDeepLinkingSettings extracts deep linking settings from the launch claims
// This should be called after parsing the raw JWT claims
func GetDeepLinkingSettings(claims map[string]any) (*DeepLinkingSettings, error) {
	settingsRaw, ok := claims[ClaimDeepLinkSettings]
	if !ok {
		return nil, fmt.Errorf("deep linking settings claim not found")
	}

	settingsMap, ok := settingsRaw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid deep linking settings format")
	}

	settings := &DeepLinkingSettings{}

	if v, ok := settingsMap["deep_link_return_url"].(string); ok {
		settings.DeepLinkReturnURL = v
	}

	if v, ok := settingsMap["accept_types"].([]any); ok {
		for _, t := range v {
			if s, ok := t.(string); ok {
				settings.AcceptTypes = append(settings.AcceptTypes, s)
			}
		}
	}

	if v, ok := settingsMap["accept_presentation_document_targets"].([]any); ok {
		for _, t := range v {
			if s, ok := t.(string); ok {
				settings.AcceptPresentationTargets = append(settings.AcceptPresentationTargets, s)
			}
		}
	}

	if v, ok := settingsMap["accept_multiple"].(bool); ok {
		settings.AcceptMultiple = v
	}

	if v, ok := settingsMap["auto_create"].(bool); ok {
		settings.AutoCreate = v
	}

	if v, ok := settingsMap["title"].(string); ok {
		settings.Title = v
	}

	if v, ok := settingsMap["text"].(string); ok {
		settings.Text = v
	}

	if v, ok := settingsMap["data"].(string); ok {
		settings.Data = v
	}

	return settings, nil
}

// EncodeBase64URL encodes bytes to base64url without padding (for JWK)
func EncodeBase64URL(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}
