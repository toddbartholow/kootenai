package oauth2

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// =============================================================================
// Generic OIDC Provider
// =============================================================================

// OIDCProvider implements a generic OpenID Connect provider
type OIDCProvider struct {
	*BaseProvider
}

// NewOIDCProvider creates a new generic OIDC provider
func NewOIDCProvider(cfg ProviderConfig) (*OIDCProvider, error) {
	cfg.Type = ProviderGenericOIDC
	if cfg.Name == "" {
		cfg.Name = "OpenID Connect"
	}

	base, err := NewBaseProvider(cfg)
	if err != nil {
		return nil, err
	}

	return &OIDCProvider{BaseProvider: base}, nil
}

// ValidateIDToken validates an ID token (simplified - production should use JWKS)
func (p *OIDCProvider) ValidateIDToken(ctx context.Context, idToken, nonce string) (*UserInfo, error) {
	// For now, fetch userinfo instead of validating ID token
	// Full implementation would validate JWT signature using JWKS
	return nil, fmt.Errorf("ID token validation not implemented - use UserInfo endpoint")
}

// =============================================================================
// Google Provider
// =============================================================================

// #nosec G101 -- These are OAuth2 endpoint URLs, not credentials
const (
	googleAuthURL     = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL    = "https://oauth2.googleapis.com/token"
	googleUserInfoURL = "https://www.googleapis.com/oauth2/v3/userinfo"
	googleIssuer      = "https://accounts.google.com"
)

// GoogleProvider implements Google OAuth2/OIDC
type GoogleProvider struct {
	*BaseProvider
}

// NewGoogleProvider creates a new Google provider
func NewGoogleProvider(cfg ProviderConfig) (*GoogleProvider, error) {
	cfg.Type = ProviderGoogle
	if cfg.Name == "" {
		cfg.Name = "Google"
	}

	// Set Google-specific endpoints
	cfg.IssuerURL = googleIssuer
	cfg.AuthURL = googleAuthURL
	cfg.TokenURL = googleTokenURL
	cfg.UserInfoURL = googleUserInfoURL

	// Default scopes for Google
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = []string{"openid", "email", "profile"}
	}

	base, err := NewBaseProvider(cfg)
	if err != nil {
		return nil, err
	}

	return &GoogleProvider{BaseProvider: base}, nil
}

// AuthURL generates the Google authorization URL with additional Google-specific params
func (p *GoogleProvider) AuthURL(state, nonce string) string {
	baseURL := p.BaseProvider.AuthURL(state, nonce)
	if baseURL == "" {
		return ""
	}

	// Add Google-specific parameters
	u, err := url.Parse(baseURL)
	if err != nil {
		return baseURL
	}

	q := u.Query()
	q.Set("access_type", "offline") // Request refresh token
	q.Set("prompt", "consent")      // Force consent screen to get refresh token
	u.RawQuery = q.Encode()

	return u.String()
}

// ValidateIDToken validates a Google ID token
func (p *GoogleProvider) ValidateIDToken(ctx context.Context, idToken, nonce string) (*UserInfo, error) {
	// Google provides a tokeninfo endpoint for validation
	tokenInfoURL := "https://oauth2.googleapis.com/tokeninfo?id_token=" + idToken

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenInfoURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating tokeninfo request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("validating ID token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxOAuthResponseSize))
		return nil, fmt.Errorf("ID token validation failed: %s", string(body))
	}

	var claims map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&claims); err != nil {
		return nil, fmt.Errorf("decoding tokeninfo response: %w", err)
	}

	// Verify audience
	if aud, ok := claims["aud"].(string); ok {
		if aud != p.config.ClientID {
			return nil, fmt.Errorf("invalid audience: expected %s, got %s", p.config.ClientID, aud)
		}
	}

	// Verify nonce if present
	if nonce != "" {
		if tokenNonce, ok := claims["nonce"].(string); ok {
			if tokenNonce != nonce {
				return nil, fmt.Errorf("nonce mismatch")
			}
		}
	}

	return p.parseUserInfo(claims), nil
}

// =============================================================================
// Microsoft Provider (Entra ID / Azure AD)
// =============================================================================

const (
	microsoftGraphURL = "https://graph.microsoft.com/v1.0/me"
)

// MicrosoftProvider implements Microsoft Entra ID OAuth2/OIDC
type MicrosoftProvider struct {
	*BaseProvider
	tenantID string
}

// NewMicrosoftProvider creates a new Microsoft provider
func NewMicrosoftProvider(cfg ProviderConfig, tenantID string) (*MicrosoftProvider, error) {
	cfg.Type = ProviderMicrosoft
	if cfg.Name == "" {
		cfg.Name = "Microsoft"
	}

	// Set Microsoft-specific endpoints
	if tenantID == "" {
		tenantID = "common" // Multi-tenant by default
	}

	cfg.AuthURL = fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/authorize", tenantID)
	cfg.TokenURL = fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", tenantID)
	cfg.UserInfoURL = microsoftGraphURL
	cfg.IssuerURL = fmt.Sprintf("https://login.microsoftonline.com/%s/v2.0", tenantID)

	// Default scopes for Microsoft
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = []string{"openid", "email", "profile", "User.Read"}
	}

	base, err := NewBaseProvider(cfg)
	if err != nil {
		return nil, err
	}

	return &MicrosoftProvider{
		BaseProvider: base,
		tenantID:     tenantID,
	}, nil
}

// AuthURL generates the Microsoft authorization URL
func (p *MicrosoftProvider) AuthURL(state, nonce string) string {
	baseURL := p.BaseProvider.AuthURL(state, nonce)
	if baseURL == "" {
		return ""
	}

	// Add Microsoft-specific parameters
	u, err := url.Parse(baseURL)
	if err != nil {
		return baseURL
	}

	q := u.Query()
	q.Set("response_mode", "query")
	u.RawQuery = q.Encode()

	return u.String()
}

// UserInfo fetches user info from Microsoft Graph API
func (p *MicrosoftProvider) UserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, microsoftGraphURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating graph request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching user from graph: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxOAuthResponseSize))
		return nil, fmt.Errorf("graph request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var graphUser struct {
		ID                string `json:"id"`
		DisplayName       string `json:"displayName"`
		GivenName         string `json:"givenName"`
		Surname           string `json:"surname"`
		Mail              string `json:"mail"`
		UserPrincipalName string `json:"userPrincipalName"`
		JobTitle          string `json:"jobTitle"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&graphUser); err != nil {
		return nil, fmt.Errorf("decoding graph response: %w", err)
	}

	// Microsoft Graph uses mail or userPrincipalName for email
	email := graphUser.Mail
	if email == "" {
		email = graphUser.UserPrincipalName
	}

	return &UserInfo{
		Sub:        graphUser.ID,
		Email:      email,
		Name:       graphUser.DisplayName,
		GivenName:  graphUser.GivenName,
		FamilyName: graphUser.Surname,
		Roles:      []string{p.config.DefaultRole},
		RawClaims: map[string]any{
			"sub":         graphUser.ID,
			"email":       email,
			"name":        graphUser.DisplayName,
			"given_name":  graphUser.GivenName,
			"family_name": graphUser.Surname,
			"job_title":   graphUser.JobTitle,
		},
	}, nil
}

// ValidateIDToken validates a Microsoft ID token
func (p *MicrosoftProvider) ValidateIDToken(ctx context.Context, idToken, nonce string) (*UserInfo, error) {
	// Microsoft ID tokens should be validated using JWKS
	// For now, use userinfo endpoint
	return nil, fmt.Errorf("ID token validation not implemented - use UserInfo endpoint")
}

// =============================================================================
// GitHub Provider
// =============================================================================

// #nosec G101 -- These are OAuth2 endpoint URLs, not credentials
const (
	githubAuthURL      = "https://github.com/login/oauth/authorize"
	githubTokenURL     = "https://github.com/login/oauth/access_token"
	githubUserURL      = "https://api.github.com/user"
	githubUserEmailURL = "https://api.github.com/user/emails"
)

// GitHubProvider implements GitHub OAuth2
type GitHubProvider struct {
	*BaseProvider
}

// NewGitHubProvider creates a new GitHub provider
func NewGitHubProvider(cfg ProviderConfig) (*GitHubProvider, error) {
	cfg.Type = ProviderGitHub
	if cfg.Name == "" {
		cfg.Name = "GitHub"
	}

	// Set GitHub-specific endpoints
	cfg.AuthURL = githubAuthURL
	cfg.TokenURL = githubTokenURL
	cfg.UserInfoURL = githubUserURL

	// Default scopes for GitHub
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = []string{"user:email", "read:user"}
	}

	base, err := NewBaseProvider(cfg)
	if err != nil {
		return nil, err
	}

	return &GitHubProvider{BaseProvider: base}, nil
}

// Exchange exchanges an authorization code for tokens (GitHub-specific)
func (p *GitHubProvider) Exchange(ctx context.Context, code string) (*TokenResponse, error) {
	data := url.Values{
		"client_id":     {p.config.ClientID},
		"client_secret": {p.config.ClientSecret},
		"code":          {code},
		"redirect_uri":  {p.config.RedirectURL},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, githubTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("exchanging code: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxOAuthResponseSize))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Scope       string `json:"scope"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}

	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("decoding token response: %w", err)
	}

	if tokenResp.Error != "" {
		return nil, fmt.Errorf("GitHub error: %s - %s", tokenResp.Error, tokenResp.ErrorDesc)
	}

	return &TokenResponse{
		AccessToken: tokenResp.AccessToken,
		TokenType:   tokenResp.TokenType,
		Scope:       tokenResp.Scope,
	}, nil
}

// UserInfo fetches user info from GitHub API
func (p *GitHubProvider) UserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	// Fetch user profile
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubUserURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating user request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxOAuthResponseSize))
		return nil, fmt.Errorf("user request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var ghUser struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ghUser); err != nil {
		return nil, fmt.Errorf("decoding user response: %w", err)
	}

	// If email is empty, fetch from emails endpoint
	email := ghUser.Email
	if email == "" {
		var emailErr error
		email, emailErr = p.fetchPrimaryEmail(ctx, accessToken)
		if emailErr != nil {
			return nil, fmt.Errorf("fetching primary email: %w", emailErr)
		}
	}

	name := ghUser.Name
	if name == "" {
		name = ghUser.Login
	}

	return &UserInfo{
		Sub:     fmt.Sprintf("%d", ghUser.ID),
		Email:   email,
		Name:    name,
		Picture: ghUser.AvatarURL,
		Roles:   []string{p.config.DefaultRole},
		RawClaims: map[string]any{
			"sub":    fmt.Sprintf("%d", ghUser.ID),
			"login":  ghUser.Login,
			"email":  email,
			"name":   name,
			"avatar": ghUser.AvatarURL,
		},
	}, nil
}

// fetchPrimaryEmail fetches the user's primary email from GitHub
func (p *GitHubProvider) fetchPrimaryEmail(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubUserEmailURL, http.NoBody)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("emails request failed with status %d", resp.StatusCode)
	}

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", err
	}

	// Find primary verified email
	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}

	// Fallback to first verified email
	for _, e := range emails {
		if e.Verified {
			return e.Email, nil
		}
	}

	// Fallback to first email
	if len(emails) > 0 {
		return emails[0].Email, nil
	}

	return "", nil
}

// ValidateIDToken - GitHub doesn't use OIDC/ID tokens
func (p *GitHubProvider) ValidateIDToken(ctx context.Context, idToken, nonce string) (*UserInfo, error) {
	return nil, fmt.Errorf("GitHub does not support OIDC ID tokens")
}

// RefreshToken - GitHub doesn't support refresh tokens by default
func (p *GitHubProvider) RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	return nil, fmt.Errorf("GitHub does not support refresh tokens")
}
