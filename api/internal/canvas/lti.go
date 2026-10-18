package canvas

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// LTIService handles LTI 1.3 authentication and launch
type LTIService struct {
	config            Config
	credentials       *Credentials
	jwksCache         *jwksCache
	jwksMu            sync.RWMutex
	nonceStore        *nonceStore
	logger            *slog.Logger
	httpClient        *http.Client
	allowInsecureKeys bool // For local development with Canvas's 512-bit keys
}

// jwksCache caches Canvas public keys
type jwksCache struct {
	keys      map[string]*rsa.PublicKey
	expiresAt time.Time
}

// isProductionMode checks if the application is running in production mode
func isProductionMode() bool {
	env := os.Getenv("GO_ENV")
	return env == "production" || env == "prod"
}

// nonceStore prevents replay attacks
type nonceStore struct {
	mu     sync.Mutex
	nonces map[string]time.Time
}

// NewLTIService creates a new LTI service
func NewLTIService(cfg Config, logger *slog.Logger) (*LTIService, error) {
	return NewLTIServiceWithOptions(cfg, logger, false)
}

// NewLTIServiceWithOptions creates a new LTI service with options
func NewLTIServiceWithOptions(cfg Config, logger *slog.Logger, allowInsecureKeys bool) (*LTIService, error) {
	// SECURITY: Block insecure keys in production mode
	if allowInsecureKeys && isProductionMode() {
		logger.Error("SECURITY: LTI insecure keys are not allowed in production mode",
			slog.String("env", "production"),
			slog.String("action", "blocking_insecure_keys"),
		)
		return nil, fmt.Errorf("LTI_ALLOW_INSECURE_KEYS cannot be enabled in production mode (GO_ENV=production)")
	}

	svc := &LTIService{
		config: cfg,
		nonceStore: &nonceStore{
			nonces: make(map[string]time.Time),
		},
		logger:            logger,
		httpClient:        &http.Client{Timeout: 30 * time.Second},
		allowInsecureKeys: allowInsecureKeys,
	}

	if allowInsecureKeys {
		logger.Warn("SECURITY: LTI service configured to allow insecure keys",
			slog.String("warning", "DO NOT USE IN PRODUCTION"),
			slog.String("reason", "JWT signature validation is disabled"),
		)
	}

	// Parse credentials if provided
	if cfg.ToolPrivateKey != "" {
		creds, err := parseCredentials(cfg.ToolPrivateKey, cfg.ToolPublicKey)
		if err != nil {
			return nil, fmt.Errorf("parsing credentials: %w", err)
		}
		svc.credentials = creds
	}

	return svc, nil
}

// parseCredentials parses PEM-encoded RSA keys
func parseCredentials(privateKeyPEM, publicKeyPEM string) (*Credentials, error) {
	creds := &Credentials{}

	if privateKeyPEM != "" {
		key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(privateKeyPEM))
		if err != nil {
			return nil, fmt.Errorf("parsing private key: %w", err)
		}
		creds.PrivateKey = key
	}

	if publicKeyPEM != "" {
		key, err := jwt.ParseRSAPublicKeyFromPEM([]byte(publicKeyPEM))
		if err != nil {
			return nil, fmt.Errorf("parsing public key: %w", err)
		}
		creds.PublicKey = key
	}

	return creds, nil
}

// ValidateLaunch validates an LTI 1.3 launch request
func (s *LTIService) ValidateLaunch(ctx context.Context, idToken string) (*LTILaunchRequest, error) {
	var claims jwt.MapClaims
	var token *jwt.Token
	var err error

	if s.allowInsecureKeys {
		// For local development with Canvas's insecure 512-bit keys
		// Skip signature validation entirely
		// AUDIT: Log every use of insecure validation for security tracking
		s.logger.Warn("SECURITY AUDIT: LTI launch with insecure key validation",
			slog.String("action", "lti_insecure_validation"),
			slog.String("warning", "JWT signature validation skipped"),
			slog.Time("timestamp", time.Now()),
		)
		token, _, err = new(jwt.Parser).ParseUnverified(idToken, jwt.MapClaims{})
		if err != nil {
			return nil, fmt.Errorf("parsing token: %w", err)
		}
		claims, _ = token.Claims.(jwt.MapClaims)
	} else {
		// Normal flow with full signature validation
		// Parse the JWT without validation first to get the header
		token, _, err = new(jwt.Parser).ParseUnverified(idToken, jwt.MapClaims{})
		if err != nil {
			return nil, fmt.Errorf("parsing token: %w", err)
		}

		// Get the key ID from header
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, fmt.Errorf("missing kid in token header")
		}

		// Get public key from Canvas JWKS
		publicKey, err := s.getCanvasPublicKey(ctx, kid)
		if err != nil {
			return nil, fmt.Errorf("getting public key: %w", err)
		}

		// Parse and validate the token
		claims = jwt.MapClaims{}
		token, err = jwt.ParseWithClaims(idToken, claims, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return publicKey, nil
		})

		if err != nil {
			return nil, fmt.Errorf("validating token: %w", err)
		}

		if !token.Valid {
			return nil, fmt.Errorf("invalid token")
		}
	}

	// Validate standard claims
	if err := s.validateStandardClaims(claims); err != nil {
		return nil, err
	}

	// Convert to LTILaunchRequest
	launch, err := s.parseLaunchClaims(claims)
	if err != nil {
		return nil, fmt.Errorf("parsing launch claims: %w", err)
	}

	s.logger.Info("LTI launch validated",
		"user", launch.Subject,
		"context", launch.Context.ID,
		"resourceLink", launch.ResourceLink.ID,
	)

	return launch, nil
}

// validateStandardClaims validates required JWT claims
func (s *LTIService) validateStandardClaims(claims jwt.MapClaims) error {
	// Validate issuer
	iss, ok := claims["iss"].(string)
	if !ok {
		return fmt.Errorf("missing issuer")
	}

	// Canvas uses https://canvas.instructure.com as standard issuer even for local installs
	validIssuers := []string{s.config.CanvasURL, "https://canvas.instructure.com"}
	issuerValid := false
	for _, validIss := range validIssuers {
		if iss == validIss {
			issuerValid = true
			break
		}
	}

	if !issuerValid && !s.allowInsecureKeys {
		return fmt.Errorf("invalid issuer: expected %s, got %s", s.config.CanvasURL, iss)
	}

	if !issuerValid {
		s.logger.Warn("Accepting non-standard issuer due to allowInsecureKeys", "issuer", iss)
	}

	// Validate audience
	aud, ok := claims["aud"]
	if !ok {
		return fmt.Errorf("missing audience")
	}

	// Audience can be string or array
	validAud := false
	switch v := aud.(type) {
	case string:
		validAud = v == s.config.ClientID
	case []any:
		for _, a := range v {
			if str, ok := a.(string); ok && str == s.config.ClientID {
				validAud = true
				break
			}
		}
	}
	if !validAud {
		return fmt.Errorf("invalid audience")
	}

	// Validate nonce (replay protection)
	nonce, ok := claims["nonce"].(string)
	if !ok || nonce == "" {
		return fmt.Errorf("missing nonce")
	}

	s.nonceStore.mu.Lock()
	defer s.nonceStore.mu.Unlock()

	if s.nonceStore.nonces[nonce].After(time.Now().Add(-5 * time.Minute)) {
		return fmt.Errorf("nonce already used")
	}
	s.nonceStore.nonces[nonce] = time.Now()

	// Clean old nonces
	for n, t := range s.nonceStore.nonces {
		if t.Before(time.Now().Add(-10 * time.Minute)) {
			delete(s.nonceStore.nonces, n)
		}
	}

	return nil
}

// parseLaunchClaims converts JWT claims to LTILaunchRequest
func (s *LTIService) parseLaunchClaims(claims jwt.MapClaims) (*LTILaunchRequest, error) {
	// Re-encode and decode to struct
	data, err := json.Marshal(claims)
	if err != nil {
		return nil, err
	}

	var launch LTILaunchRequest
	if err := json.Unmarshal(data, &launch); err != nil {
		return nil, err
	}

	return &launch, nil
}

// getCanvasPublicKey retrieves a public key from Canvas JWKS
func (s *LTIService) getCanvasPublicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	// Check cache
	s.jwksMu.RLock()
	if s.jwksCache != nil && time.Now().Before(s.jwksCache.expiresAt) {
		if key, ok := s.jwksCache.keys[kid]; ok {
			s.jwksMu.RUnlock()
			return key, nil
		}
	}
	s.jwksMu.RUnlock()

	// Fetch JWKS from Canvas
	req, err := http.NewRequestWithContext(ctx, "GET", s.config.JWKsURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS request failed with status %d", resp.StatusCode)
	}

	var jwks struct {
		Keys []json.RawMessage `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("decoding JWKS: %w", err)
	}

	// Parse and cache keys
	s.jwksMu.Lock()
	s.jwksCache = &jwksCache{
		keys:      make(map[string]*rsa.PublicKey),
		expiresAt: time.Now().Add(time.Hour),
	}

	for _, keyData := range jwks.Keys {
		var keyInfo struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			N   string `json:"n"`
			E   string `json:"e"`
			Alg string `json:"alg"`
			Use string `json:"use"`
		}
		if err := json.Unmarshal(keyData, &keyInfo); err != nil {
			s.logger.Warn("Failed to parse JWK key info", "error", err)
			continue
		}

		if keyInfo.Kty != "RSA" {
			continue
		}

		// Parse RSA public key from JWK
		pubKey, err := parseRSAPublicKeyFromJWK(keyInfo.N, keyInfo.E)
		if err != nil {
			s.logger.Warn("Failed to parse RSA key from JWK", "kid", keyInfo.Kid, "error", err)
			continue
		}

		s.jwksCache.keys[keyInfo.Kid] = pubKey
		s.logger.Debug("Cached JWK public key", "kid", keyInfo.Kid, "alg", keyInfo.Alg)
	}

	key, ok := s.jwksCache.keys[kid]
	s.jwksMu.Unlock()
	if !ok {
		return nil, fmt.Errorf("key %s not found in JWKS", kid)
	}

	return key, nil
}

// parseRSAPublicKeyFromJWK parses an RSA public key from JWK n and e values
func parseRSAPublicKeyFromJWK(nBase64, eBase64 string) (*rsa.PublicKey, error) {
	// JWK uses base64url encoding (RFC 4648 Section 5)
	// The n and e values are base64url-encoded big-endian unsigned integers

	// Decode n (modulus)
	nBytes, err := base64.RawURLEncoding.DecodeString(nBase64)
	if err != nil {
		return nil, fmt.Errorf("decoding modulus (n): %w", err)
	}

	// Decode e (exponent)
	eBytes, err := base64.RawURLEncoding.DecodeString(eBase64)
	if err != nil {
		return nil, fmt.Errorf("decoding exponent (e): %w", err)
	}

	// Convert bytes to big integers
	n := new(big.Int).SetBytes(nBytes)

	// Convert exponent bytes to int (typically 65537 = 0x010001)
	var e int
	for _, b := range eBytes {
		e = e<<8 + int(b)
	}

	if e == 0 {
		return nil, fmt.Errorf("invalid exponent: zero")
	}
	if n.Sign() <= 0 {
		return nil, fmt.Errorf("invalid modulus: non-positive")
	}

	return &rsa.PublicKey{
		N: n,
		E: e,
	}, nil
}

// HasRole checks if the launch has a specific role
func (l *LTILaunchRequest) HasRole(role string) bool {
	for _, r := range l.Roles {
		if r == role || strings.HasSuffix(r, "#"+strings.Split(role, "#")[1]) {
			return true
		}
	}
	return false
}

// IsInstructor checks if the user is an instructor
func (l *LTILaunchRequest) IsInstructor() bool {
	return l.HasRole(RoleInstructor) || l.HasRole(RoleTeachingAssistant)
}

// IsLearner checks if the user is a learner/student
func (l *LTILaunchRequest) IsLearner() bool {
	return l.HasRole(RoleLearner)
}

// GetCustomParam retrieves a custom launch parameter
func (l *LTILaunchRequest) GetCustomParam(key string) string {
	if l.Custom == nil {
		return ""
	}
	return l.Custom[key]
}

// CanSubmitGrades checks if the launch allows grade submission
func (l *LTILaunchRequest) CanSubmitGrades() bool {
	if l.AGS == nil {
		return false
	}

	for _, scope := range l.AGS.Scope {
		if strings.Contains(scope, "score") {
			return true
		}
	}
	return false
}

// GetLineItemURL returns the line item URL for grade submission
func (l *LTILaunchRequest) GetLineItemURL() string {
	if l.AGS == nil {
		return ""
	}
	return l.AGS.LineItem
}

// GetCanvasUserID returns the Canvas user ID
func (l *LTILaunchRequest) GetCanvasUserID() string {
	if l.CanvasUser != nil {
		return l.CanvasUser.ID
	}
	return l.Subject
}

// GetCanvasCourseID returns the Canvas course ID
func (l *LTILaunchRequest) GetCanvasCourseID() string {
	if l.CanvasCourse != nil {
		return l.CanvasCourse.ID
	}
	return l.Context.ID
}
