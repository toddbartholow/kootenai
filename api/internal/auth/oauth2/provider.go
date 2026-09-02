// Package oauth2 provides OAuth2/OIDC authentication support
package oauth2

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxOAuthResponseSize = 1 << 20 // 1MB max response from OAuth providers

// ProviderType identifies the OAuth2 provider
type ProviderType string

const (
	ProviderGenericOIDC ProviderType = "oidc"
	ProviderGoogle      ProviderType = "google"
	ProviderMicrosoft   ProviderType = "microsoft"
	ProviderGitHub      ProviderType = "github"
)

// ProviderConfig holds configuration for an OAuth2 provider
type ProviderConfig struct {
	// Basic OAuth2 settings
	Type         ProviderType `json:"type" yaml:"type"`
	Name         string       `json:"name" yaml:"name"` // Display name (e.g., "Sign in with Google")
	ClientID     string       `json:"clientId" yaml:"client_id"`
	ClientSecret string       `json:"clientSecret" yaml:"client_secret"`
	RedirectURL  string       `json:"redirectUrl" yaml:"redirect_url"`

	// OIDC Discovery (for generic OIDC providers)
	IssuerURL string `json:"issuerUrl" yaml:"issuer_url"` // e.g., https://accounts.google.com

	// Manual endpoint configuration (if not using discovery)
	AuthURL     string `json:"authUrl" yaml:"auth_url"`
	TokenURL    string `json:"tokenUrl" yaml:"token_url"`
	UserInfoURL string `json:"userInfoUrl" yaml:"userinfo_url"`
	JWKSURL     string `json:"jwksUrl" yaml:"jwks_url"`

	// Scopes to request
	Scopes []string `json:"scopes" yaml:"scopes"`

	// Role mapping from provider claims to platform roles
	RoleMapping map[string]string `json:"roleMapping" yaml:"role_mapping"`

	// Default role if no mapping matches
	DefaultRole string `json:"defaultRole" yaml:"default_role"`

	// Claims configuration
	EmailClaim string `json:"emailClaim" yaml:"email_claim"` // Claim name for email (default: "email")
	NameClaim  string `json:"nameClaim" yaml:"name_claim"`   // Claim name for name (default: "name")
	RoleClaim  string `json:"roleClaim" yaml:"role_claim"`   // Claim name for roles (optional)

	// Whether this provider is enabled
	Enabled bool `json:"enabled" yaml:"enabled"`
}

// OIDCDiscovery represents OpenID Connect discovery document
type OIDCDiscovery struct {
	Issuer                string   `json:"issuer"`
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	TokenEndpoint         string   `json:"token_endpoint"`
	UserInfoEndpoint      string   `json:"userinfo_endpoint"`
	JWKSURI               string   `json:"jwks_uri"`
	ScopesSupported       []string `json:"scopes_supported"`
	ClaimsSupported       []string `json:"claims_supported"`
}

// TokenResponse represents an OAuth2 token response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// UserInfo represents user information from the provider
type UserInfo struct {
	Sub           string         `json:"sub"` // Subject (unique user ID)
	Email         string         `json:"email"`
	EmailVerified bool           `json:"email_verified"`
	Name          string         `json:"name"`
	GivenName     string         `json:"given_name"`
	FamilyName    string         `json:"family_name"`
	Picture       string         `json:"picture"`
	Roles         []string       `json:"roles,omitempty"` // Mapped roles
	RawClaims     map[string]any `json:"-"`               // All claims from the token/userinfo
}

// Provider defines the interface for OAuth2/OIDC providers
type Provider interface {
	// Type returns the provider type
	Type() ProviderType

	// Name returns the display name
	Name() string

	// AuthURL returns the authorization URL for initiating login
	// state and nonce should be cryptographically random
	AuthURL(state, nonce string) string

	// Exchange exchanges an authorization code for tokens
	Exchange(ctx context.Context, code string) (*TokenResponse, error)

	// UserInfo fetches user information using the access token
	UserInfo(ctx context.Context, accessToken string) (*UserInfo, error)

	// ValidateIDToken validates an ID token and extracts claims
	ValidateIDToken(ctx context.Context, idToken, nonce string) (*UserInfo, error)

	// RefreshToken refreshes an access token
	RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error)
}

// BaseProvider provides common functionality for OAuth2 providers
type BaseProvider struct {
	config     ProviderConfig
	discovery  *OIDCDiscovery
	httpClient *http.Client
}

// NewBaseProvider creates a new base provider with common settings
func NewBaseProvider(cfg ProviderConfig) (*BaseProvider, error) {
	if cfg.ClientID == "" {
		return nil, errors.New("client ID is required")
	}
	if cfg.ClientSecret == "" {
		return nil, errors.New("client secret is required")
	}
	if cfg.RedirectURL == "" {
		return nil, errors.New("redirect URL is required")
	}

	// Set defaults
	if cfg.EmailClaim == "" {
		cfg.EmailClaim = "email"
	}
	if cfg.NameClaim == "" {
		cfg.NameClaim = "name"
	}
	if cfg.DefaultRole == "" {
		cfg.DefaultRole = "student"
	}

	p := &BaseProvider{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	return p, nil
}

// Type returns the provider type
func (p *BaseProvider) Type() ProviderType {
	return p.config.Type
}

// Name returns the display name
func (p *BaseProvider) Name() string {
	return p.config.Name
}

// Config returns the provider configuration
func (p *BaseProvider) Config() ProviderConfig {
	return p.config
}

// FetchDiscovery fetches the OIDC discovery document
func (p *BaseProvider) FetchDiscovery(ctx context.Context) (*OIDCDiscovery, error) {
	if p.discovery != nil {
		return p.discovery, nil
	}

	if p.config.IssuerURL == "" {
		return nil, errors.New("issuer URL is required for OIDC discovery")
	}

	discoveryURL := strings.TrimSuffix(p.config.IssuerURL, "/") + "/.well-known/openid-configuration"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating discovery request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching discovery document: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxOAuthResponseSize))
		return nil, fmt.Errorf("discovery request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var discovery OIDCDiscovery
	if err := json.NewDecoder(resp.Body).Decode(&discovery); err != nil {
		return nil, fmt.Errorf("decoding discovery document: %w", err)
	}

	p.discovery = &discovery
	return p.discovery, nil
}

// GetAuthURL returns the authorization URL
func (p *BaseProvider) GetAuthURL() string {
	if p.config.AuthURL != "" {
		return p.config.AuthURL
	}
	if p.discovery != nil {
		return p.discovery.AuthorizationEndpoint
	}
	return ""
}

// GetTokenURL returns the token URL
func (p *BaseProvider) GetTokenURL() string {
	if p.config.TokenURL != "" {
		return p.config.TokenURL
	}
	if p.discovery != nil {
		return p.discovery.TokenEndpoint
	}
	return ""
}

// GetUserInfoURL returns the userinfo URL
func (p *BaseProvider) GetUserInfoURL() string {
	if p.config.UserInfoURL != "" {
		return p.config.UserInfoURL
	}
	if p.discovery != nil {
		return p.discovery.UserInfoEndpoint
	}
	return ""
}

// AuthURL generates the authorization URL
func (p *BaseProvider) AuthURL(state, nonce string) string {
	authURL := p.GetAuthURL()
	if authURL == "" {
		return ""
	}

	params := url.Values{
		"client_id":     {p.config.ClientID},
		"redirect_uri":  {p.config.RedirectURL},
		"response_type": {"code"},
		"state":         {state},
	}

	// Add scopes
	scopes := p.config.Scopes
	if len(scopes) == 0 {
		scopes = []string{"openid", "email", "profile"}
	}
	params.Set("scope", strings.Join(scopes, " "))

	// Add nonce for OIDC
	if nonce != "" {
		params.Set("nonce", nonce)
	}

	return authURL + "?" + params.Encode()
}

// Exchange exchanges an authorization code for tokens
func (p *BaseProvider) Exchange(ctx context.Context, code string) (*TokenResponse, error) {
	tokenURL := p.GetTokenURL()
	if tokenURL == "" {
		return nil, errors.New("token URL not configured")
	}

	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {p.config.RedirectURL},
		"client_id":     {p.config.ClientID},
		"client_secret": {p.config.ClientSecret},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("exchanging code for token: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxOAuthResponseSize))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("decoding token response: %w", err)
	}

	return &tokenResp, nil
}

// UserInfo fetches user information from the userinfo endpoint
func (p *BaseProvider) UserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	userInfoURL := p.GetUserInfoURL()
	if userInfoURL == "" {
		return nil, errors.New("userinfo URL not configured")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userInfoURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating userinfo request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching userinfo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("userinfo request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var rawClaims map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&rawClaims); err != nil {
		return nil, fmt.Errorf("decoding userinfo response: %w", err)
	}

	return p.parseUserInfo(rawClaims), nil
}

// RefreshToken refreshes an access token
func (p *BaseProvider) RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	tokenURL := p.GetTokenURL()
	if tokenURL == "" {
		return nil, errors.New("token URL not configured")
	}

	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {p.config.ClientID},
		"client_secret": {p.config.ClientSecret},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refreshing token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token refresh failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("decoding refresh response: %w", err)
	}

	return &tokenResp, nil
}

// parseUserInfo extracts user information from raw claims
func (p *BaseProvider) parseUserInfo(claims map[string]any) *UserInfo {
	info := &UserInfo{
		RawClaims: claims,
	}

	// Extract standard claims
	if sub, ok := claims["sub"].(string); ok {
		info.Sub = sub
	}

	// Extract email using configured claim name
	if email, ok := claims[p.config.EmailClaim].(string); ok {
		info.Email = email
	}
	if verified, ok := claims["email_verified"].(bool); ok {
		info.EmailVerified = verified
	}

	// Extract name using configured claim name
	if name, ok := claims[p.config.NameClaim].(string); ok {
		info.Name = name
	}
	if given, ok := claims["given_name"].(string); ok {
		info.GivenName = given
	}
	if family, ok := claims["family_name"].(string); ok {
		info.FamilyName = family
	}
	if picture, ok := claims["picture"].(string); ok {
		info.Picture = picture
	}

	// Extract and map roles
	info.Roles = p.extractRoles(claims)

	return info
}

// extractRoles extracts and maps roles from claims
func (p *BaseProvider) extractRoles(claims map[string]any) []string {
	var roles []string

	// If a role claim is configured, try to extract it
	if p.config.RoleClaim != "" {
		switch v := claims[p.config.RoleClaim].(type) {
		case string:
			roles = append(roles, v)
		case []any:
			for _, r := range v {
				if s, ok := r.(string); ok {
					roles = append(roles, s)
				}
			}
		case []string:
			roles = append(roles, v...)
		}
	}

	// Map provider roles to platform roles
	var mappedRoles []string
	if len(p.config.RoleMapping) > 0 {
		for _, role := range roles {
			if mapped, ok := p.config.RoleMapping[role]; ok {
				mappedRoles = append(mappedRoles, mapped)
			}
		}
	}

	// Use mapped roles if any, otherwise use original roles
	if len(mappedRoles) > 0 {
		return mappedRoles
	}
	if len(roles) > 0 {
		return roles
	}

	// Return default role if no roles found
	return []string{p.config.DefaultRole}
}

// GenerateState generates a cryptographically random state parameter
func GenerateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating state: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// GenerateNonce generates a cryptographically random nonce
func GenerateNonce() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating nonce: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// GeneratePKCEVerifier generates a PKCE code verifier
func GeneratePKCEVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating PKCE verifier: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
