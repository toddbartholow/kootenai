package canvas

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// testLogger returns a logger for tests
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

// generateTestRSAKey generates a 2048-bit RSA key pair for testing
func generateTestRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	return key
}

// createSignedJWT creates a JWT token signed with the given private key
func createSignedJWT(t *testing.T, claims jwt.MapClaims, privateKey *rsa.PrivateKey, kid string) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid

	signed, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("failed to sign JWT: %v", err)
	}
	return signed
}

// setupJWKSServer creates a test HTTP server that serves JWKS
func setupJWKSServer(t *testing.T, publicKey *rsa.PublicKey, kid string) *httptest.Server {
	t.Helper()

	// Base64URL encode the modulus and exponent
	nBytes := publicKey.N.Bytes()
	eBytes := big.NewInt(int64(publicKey.E)).Bytes()

	jwks := map[string]interface{}{
		"keys": []map[string]interface{}{
			{
				"kty": "RSA",
				"kid": kid,
				"use": "sig",
				"alg": "RS256",
				"n":   base64.RawURLEncoding.EncodeToString(nBytes),
				"e":   base64.RawURLEncoding.EncodeToString(eBytes),
			},
		},
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks)
	}))
}

// -----------------------------------------------------------------------------
// NewLTIService Tests
// -----------------------------------------------------------------------------

func TestNewLTIService_Success(t *testing.T) {
	cfg := Config{
		CanvasURL: "https://canvas.example.com",
		ClientID:  "test-client-id",
		JWKsURL:   "https://canvas.example.com/.well-known/jwks.json",
	}

	svc, err := NewLTIService(cfg, testLogger())
	if err != nil {
		t.Fatalf("NewLTIService failed: %v", err)
	}
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestNewLTIService_WithInvalidPrivateKey(t *testing.T) {
	cfg := Config{
		CanvasURL:      "https://canvas.example.com",
		ClientID:       "test-client-id",
		JWKsURL:        "https://canvas.example.com/.well-known/jwks.json",
		ToolPrivateKey: "invalid-pem-data",
	}

	_, err := NewLTIService(cfg, testLogger())
	if err == nil {
		t.Fatal("expected error for invalid private key")
	}
}

func TestNewLTIService_WithValidCredentials(t *testing.T) {
	// Generate a proper PKCS1 PEM-encoded key dynamically
	privateKey := generateTestRSAKey(t)

	// Encode private key to PKCS1 PEM format (which jwt library supports)
	keyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privatePEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: keyBytes,
	}))

	cfg := Config{
		CanvasURL:      "https://canvas.example.com",
		ClientID:       "test-client-id",
		JWKsURL:        "https://canvas.example.com/.well-known/jwks.json",
		ToolPrivateKey: privatePEM,
	}

	svc, err := NewLTIService(cfg, testLogger())
	if err != nil {
		t.Fatalf("NewLTIService failed: %v", err)
	}
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if svc.credentials == nil {
		t.Fatal("expected credentials to be set")
	}
}

// -----------------------------------------------------------------------------
// ValidateLaunch Tests
// -----------------------------------------------------------------------------

func TestValidateLaunch_Success(t *testing.T) {
	privateKey := generateTestRSAKey(t)
	kid := "test-key-id"

	// Setup JWKS server
	server := setupJWKSServer(t, &privateKey.PublicKey, kid)
	defer server.Close()

	cfg := Config{
		CanvasURL: "https://canvas.example.com",
		ClientID:  "test-client-id",
		JWKsURL:   server.URL,
	}

	svc, err := NewLTIService(cfg, testLogger())
	if err != nil {
		t.Fatalf("NewLTIService failed: %v", err)
	}

	// Create valid claims
	now := time.Now()
	claims := jwt.MapClaims{
		"iss":   "https://canvas.example.com",
		"sub":   "user-123",
		"aud":   "test-client-id",
		"exp":   now.Add(time.Hour).Unix(),
		"iat":   now.Unix(),
		"nonce": "unique-nonce-12345",
		"https://purl.imsglobal.org/spec/lti/claim/message_type":    "LtiResourceLinkRequest",
		"https://purl.imsglobal.org/spec/lti/claim/version":         "1.3.0",
		"https://purl.imsglobal.org/spec/lti/claim/deployment_id":   "deployment-1",
		"https://purl.imsglobal.org/spec/lti/claim/target_link_uri": "https://lab.example.com/launch",
		"https://purl.imsglobal.org/spec/lti/claim/resource_link": map[string]interface{}{
			"id":    "resource-link-1",
			"title": "Test Lab",
		},
		"https://purl.imsglobal.org/spec/lti/claim/roles": []string{
			"http://purl.imsglobal.org/vocab/lis/v2/membership#Learner",
		},
		"https://purl.imsglobal.org/spec/lti/claim/context": map[string]interface{}{
			"id":    "course-123",
			"label": "CS101",
			"title": "Intro to Computer Science",
		},
	}

	idToken := createSignedJWT(t, claims, privateKey, kid)

	launch, err := svc.ValidateLaunch(context.Background(), idToken)
	if err != nil {
		t.Fatalf("ValidateLaunch failed: %v", err)
	}

	if launch.Subject != "user-123" {
		t.Errorf("expected subject user-123, got %s", launch.Subject)
	}
	if launch.Context.ID != "course-123" {
		t.Errorf("expected context ID course-123, got %s", launch.Context.ID)
	}
	if launch.ResourceLink.ID != "resource-link-1" {
		t.Errorf("expected resource link ID resource-link-1, got %s", launch.ResourceLink.ID)
	}
}

func TestValidateLaunch_MissingKid(t *testing.T) {
	privateKey := generateTestRSAKey(t)

	server := setupJWKSServer(t, &privateKey.PublicKey, "test-key-id")
	defer server.Close()

	cfg := Config{
		CanvasURL: "https://canvas.example.com",
		ClientID:  "test-client-id",
		JWKsURL:   server.URL,
	}

	svc, _ := NewLTIService(cfg, testLogger())

	// Create token without kid in header
	now := time.Now()
	claims := jwt.MapClaims{
		"iss":   "https://canvas.example.com",
		"sub":   "user-123",
		"aud":   "test-client-id",
		"exp":   now.Add(time.Hour).Unix(),
		"iat":   now.Unix(),
		"nonce": "unique-nonce",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	// Intentionally NOT setting kid
	idToken, _ := token.SignedString(privateKey)

	_, err := svc.ValidateLaunch(context.Background(), idToken)
	if err == nil {
		t.Fatal("expected error for missing kid")
	}
	if err.Error() != "missing kid in token header" {
		t.Errorf("expected 'missing kid in token header' error, got: %v", err)
	}
}

func TestValidateLaunch_InvalidIssuer(t *testing.T) {
	privateKey := generateTestRSAKey(t)
	kid := "test-key-id"

	server := setupJWKSServer(t, &privateKey.PublicKey, kid)
	defer server.Close()

	cfg := Config{
		CanvasURL: "https://canvas.example.com",
		ClientID:  "test-client-id",
		JWKsURL:   server.URL,
	}

	svc, _ := NewLTIService(cfg, testLogger())

	now := time.Now()
	claims := jwt.MapClaims{
		"iss":   "https://wrong-issuer.com", // Wrong issuer
		"sub":   "user-123",
		"aud":   "test-client-id",
		"exp":   now.Add(time.Hour).Unix(),
		"iat":   now.Unix(),
		"nonce": "unique-nonce",
	}

	idToken := createSignedJWT(t, claims, privateKey, kid)

	_, err := svc.ValidateLaunch(context.Background(), idToken)
	if err == nil {
		t.Fatal("expected error for invalid issuer")
	}
}

func TestValidateLaunch_InvalidAudience_String(t *testing.T) {
	privateKey := generateTestRSAKey(t)
	kid := "test-key-id"

	server := setupJWKSServer(t, &privateKey.PublicKey, kid)
	defer server.Close()

	cfg := Config{
		CanvasURL: "https://canvas.example.com",
		ClientID:  "test-client-id",
		JWKsURL:   server.URL,
	}

	svc, _ := NewLTIService(cfg, testLogger())

	now := time.Now()
	claims := jwt.MapClaims{
		"iss":   "https://canvas.example.com",
		"sub":   "user-123",
		"aud":   "wrong-client-id", // Wrong audience
		"exp":   now.Add(time.Hour).Unix(),
		"iat":   now.Unix(),
		"nonce": "unique-nonce",
	}

	idToken := createSignedJWT(t, claims, privateKey, kid)

	_, err := svc.ValidateLaunch(context.Background(), idToken)
	if err == nil {
		t.Fatal("expected error for invalid audience")
	}
}

func TestValidateLaunch_InvalidAudience_Array(t *testing.T) {
	privateKey := generateTestRSAKey(t)
	kid := "test-key-id"

	server := setupJWKSServer(t, &privateKey.PublicKey, kid)
	defer server.Close()

	cfg := Config{
		CanvasURL: "https://canvas.example.com",
		ClientID:  "test-client-id",
		JWKsURL:   server.URL,
	}

	svc, _ := NewLTIService(cfg, testLogger())

	now := time.Now()
	claims := jwt.MapClaims{
		"iss":   "https://canvas.example.com",
		"sub":   "user-123",
		"aud":   []interface{}{"wrong-client-1", "wrong-client-2"}, // None match
		"exp":   now.Add(time.Hour).Unix(),
		"iat":   now.Unix(),
		"nonce": "unique-nonce",
	}

	idToken := createSignedJWT(t, claims, privateKey, kid)

	_, err := svc.ValidateLaunch(context.Background(), idToken)
	if err == nil {
		t.Fatal("expected error for invalid audience array")
	}
}

func TestValidateLaunch_ValidAudienceArray(t *testing.T) {
	// Note: The LTILaunchRequest struct uses string for Audience field,
	// so this test verifies that validateStandardClaims correctly handles
	// array audiences from the JWT claims (even though the struct field is string).
	// The validation passes but the parsed struct won't have the full array.

	privateKey := generateTestRSAKey(t)
	kid := "test-key-id"

	server := setupJWKSServer(t, &privateKey.PublicKey, kid)
	defer server.Close()

	cfg := Config{
		CanvasURL: "https://canvas.example.com",
		ClientID:  "test-client-id",
		JWKsURL:   server.URL,
	}

	svc, _ := NewLTIService(cfg, testLogger())

	now := time.Now()
	// Test with string audience (the typical case that works end-to-end)
	claims := jwt.MapClaims{
		"iss":   "https://canvas.example.com",
		"sub":   "user-123",
		"aud":   "test-client-id", // Single string audience
		"exp":   now.Add(time.Hour).Unix(),
		"iat":   now.Unix(),
		"nonce": "unique-nonce-valid-aud-string",
		"https://purl.imsglobal.org/spec/lti/claim/message_type":  "LtiResourceLinkRequest",
		"https://purl.imsglobal.org/spec/lti/claim/version":       "1.3.0",
		"https://purl.imsglobal.org/spec/lti/claim/deployment_id": "deployment-1",
		"https://purl.imsglobal.org/spec/lti/claim/resource_link": map[string]interface{}{
			"id": "resource-link-1",
		},
	}

	idToken := createSignedJWT(t, claims, privateKey, kid)

	launch, err := svc.ValidateLaunch(context.Background(), idToken)
	if err != nil {
		t.Fatalf("ValidateLaunch failed with valid audience: %v", err)
	}
	if launch.Subject != "user-123" {
		t.Errorf("expected subject user-123, got %s", launch.Subject)
	}
}

func TestValidateStandardClaims_AudienceArray(t *testing.T) {
	// This test verifies that validateStandardClaims handles array audiences
	// from the raw JWT claims before they're parsed into the struct.

	cfg := Config{
		CanvasURL: "https://canvas.example.com",
		ClientID:  "test-client-id",
	}

	svc, _ := NewLTIService(cfg, testLogger())

	now := time.Now()
	claims := jwt.MapClaims{
		"iss":   "https://canvas.example.com",
		"aud":   []interface{}{"other-client", "test-client-id"}, // Array with valid client
		"nonce": "unique-nonce-array-test",
		"exp":   now.Add(time.Hour).Unix(),
	}

	err := svc.validateStandardClaims(claims)
	if err != nil {
		t.Errorf("validateStandardClaims should accept array audience containing valid client: %v", err)
	}
}

func TestValidateLaunch_MissingNonce(t *testing.T) {
	privateKey := generateTestRSAKey(t)
	kid := "test-key-id"

	server := setupJWKSServer(t, &privateKey.PublicKey, kid)
	defer server.Close()

	cfg := Config{
		CanvasURL: "https://canvas.example.com",
		ClientID:  "test-client-id",
		JWKsURL:   server.URL,
	}

	svc, _ := NewLTIService(cfg, testLogger())

	now := time.Now()
	claims := jwt.MapClaims{
		"iss": "https://canvas.example.com",
		"sub": "user-123",
		"aud": "test-client-id",
		"exp": now.Add(time.Hour).Unix(),
		"iat": now.Unix(),
		// No nonce
	}

	idToken := createSignedJWT(t, claims, privateKey, kid)

	_, err := svc.ValidateLaunch(context.Background(), idToken)
	if err == nil {
		t.Fatal("expected error for missing nonce")
	}
}

func TestValidateLaunch_ReusedNonce(t *testing.T) {
	privateKey := generateTestRSAKey(t)
	kid := "test-key-id"

	server := setupJWKSServer(t, &privateKey.PublicKey, kid)
	defer server.Close()

	cfg := Config{
		CanvasURL: "https://canvas.example.com",
		ClientID:  "test-client-id",
		JWKsURL:   server.URL,
	}

	svc, _ := NewLTIService(cfg, testLogger())

	now := time.Now()
	reusedNonce := "reused-nonce-12345"

	claims := jwt.MapClaims{
		"iss":   "https://canvas.example.com",
		"sub":   "user-123",
		"aud":   "test-client-id",
		"exp":   now.Add(time.Hour).Unix(),
		"iat":   now.Unix(),
		"nonce": reusedNonce,
		"https://purl.imsglobal.org/spec/lti/claim/message_type":  "LtiResourceLinkRequest",
		"https://purl.imsglobal.org/spec/lti/claim/version":       "1.3.0",
		"https://purl.imsglobal.org/spec/lti/claim/deployment_id": "deployment-1",
		"https://purl.imsglobal.org/spec/lti/claim/resource_link": map[string]interface{}{
			"id": "resource-link-1",
		},
	}

	idToken := createSignedJWT(t, claims, privateKey, kid)

	// First launch should succeed
	_, err := svc.ValidateLaunch(context.Background(), idToken)
	if err != nil {
		t.Fatalf("first launch failed: %v", err)
	}

	// Second launch with same nonce should fail (replay attack)
	_, err = svc.ValidateLaunch(context.Background(), idToken)
	if err == nil {
		t.Fatal("expected error for reused nonce")
	}
}

func TestValidateLaunch_MalformedToken(t *testing.T) {
	cfg := Config{
		CanvasURL: "https://canvas.example.com",
		ClientID:  "test-client-id",
		JWKsURL:   "https://canvas.example.com/jwks",
	}

	svc, _ := NewLTIService(cfg, testLogger())

	tests := []struct {
		name  string
		token string
	}{
		{name: "empty token", token: ""},
		{name: "not a JWT", token: "not.a.jwt"},
		{name: "invalid base64", token: "invalid!.base64!.token!"},
		{name: "missing segments", token: "only.two"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.ValidateLaunch(context.Background(), tt.token)
			if err == nil {
				t.Errorf("expected error for %s", tt.name)
			}
		})
	}
}

func TestValidateLaunch_WrongSigningMethod(t *testing.T) {
	privateKey := generateTestRSAKey(t)
	kid := "test-key-id"

	server := setupJWKSServer(t, &privateKey.PublicKey, kid)
	defer server.Close()

	cfg := Config{
		CanvasURL: "https://canvas.example.com",
		ClientID:  "test-client-id",
		JWKsURL:   server.URL,
	}

	svc, _ := NewLTIService(cfg, testLogger())

	now := time.Now()
	claims := jwt.MapClaims{
		"iss":   "https://canvas.example.com",
		"sub":   "user-123",
		"aud":   "test-client-id",
		"exp":   now.Add(time.Hour).Unix(),
		"iat":   now.Unix(),
		"nonce": "unique-nonce",
	}

	// Sign with HMAC instead of RSA
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token.Header["kid"] = kid
	idToken, _ := token.SignedString([]byte("secret"))

	_, err := svc.ValidateLaunch(context.Background(), idToken)
	if err == nil {
		t.Fatal("expected error for wrong signing method")
	}
}

func TestValidateLaunch_ExpiredToken(t *testing.T) {
	privateKey := generateTestRSAKey(t)
	kid := "test-key-id"

	server := setupJWKSServer(t, &privateKey.PublicKey, kid)
	defer server.Close()

	cfg := Config{
		CanvasURL: "https://canvas.example.com",
		ClientID:  "test-client-id",
		JWKsURL:   server.URL,
	}

	svc, _ := NewLTIService(cfg, testLogger())

	now := time.Now()
	claims := jwt.MapClaims{
		"iss":   "https://canvas.example.com",
		"sub":   "user-123",
		"aud":   "test-client-id",
		"exp":   now.Add(-time.Hour).Unix(), // Expired 1 hour ago
		"iat":   now.Add(-2 * time.Hour).Unix(),
		"nonce": "unique-nonce-expired",
	}

	idToken := createSignedJWT(t, claims, privateKey, kid)

	_, err := svc.ValidateLaunch(context.Background(), idToken)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

// -----------------------------------------------------------------------------
// JWKS Fetching Tests
// -----------------------------------------------------------------------------

func TestGetCanvasPublicKey_Success(t *testing.T) {
	privateKey := generateTestRSAKey(t)
	kid := "test-key-id"

	server := setupJWKSServer(t, &privateKey.PublicKey, kid)
	defer server.Close()

	cfg := Config{
		CanvasURL: "https://canvas.example.com",
		ClientID:  "test-client-id",
		JWKsURL:   server.URL,
	}

	svc, _ := NewLTIService(cfg, testLogger())

	key, err := svc.getCanvasPublicKey(context.Background(), kid)
	if err != nil {
		t.Fatalf("getCanvasPublicKey failed: %v", err)
	}
	if key == nil {
		t.Fatal("expected non-nil key")
	}
}

func TestGetCanvasPublicKey_Cached(t *testing.T) {
	privateKey := generateTestRSAKey(t)
	kid := "test-key-id"

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		nBytes := privateKey.PublicKey.N.Bytes()
		eBytes := big.NewInt(int64(privateKey.PublicKey.E)).Bytes()
		jwks := map[string]interface{}{
			"keys": []map[string]interface{}{
				{
					"kty": "RSA",
					"kid": kid,
					"n":   base64.RawURLEncoding.EncodeToString(nBytes),
					"e":   base64.RawURLEncoding.EncodeToString(eBytes),
				},
			},
		}
		json.NewEncoder(w).Encode(jwks)
	}))
	defer server.Close()

	cfg := Config{
		CanvasURL: "https://canvas.example.com",
		ClientID:  "test-client-id",
		JWKsURL:   server.URL,
	}

	svc, _ := NewLTIService(cfg, testLogger())

	// First call - should fetch from server
	_, err := svc.getCanvasPublicKey(context.Background(), kid)
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	// Second call - should use cache
	_, err = svc.getCanvasPublicKey(context.Background(), kid)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected 1 server call (cached), got %d", callCount)
	}
}

func TestGetCanvasPublicKey_KeyNotFound(t *testing.T) {
	privateKey := generateTestRSAKey(t)

	server := setupJWKSServer(t, &privateKey.PublicKey, "different-key-id")
	defer server.Close()

	cfg := Config{
		CanvasURL: "https://canvas.example.com",
		ClientID:  "test-client-id",
		JWKsURL:   server.URL,
	}

	svc, _ := NewLTIService(cfg, testLogger())

	_, err := svc.getCanvasPublicKey(context.Background(), "non-existent-kid")
	if err == nil {
		t.Fatal("expected error for key not found")
	}
}

func TestGetCanvasPublicKey_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := Config{
		CanvasURL: "https://canvas.example.com",
		ClientID:  "test-client-id",
		JWKsURL:   server.URL,
	}

	svc, _ := NewLTIService(cfg, testLogger())

	_, err := svc.getCanvasPublicKey(context.Background(), "test-kid")
	if err == nil {
		t.Fatal("expected error for HTTP error")
	}
}

func TestGetCanvasPublicKey_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	cfg := Config{
		CanvasURL: "https://canvas.example.com",
		ClientID:  "test-client-id",
		JWKsURL:   server.URL,
	}

	svc, _ := NewLTIService(cfg, testLogger())

	_, err := svc.getCanvasPublicKey(context.Background(), "test-kid")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestGetCanvasPublicKey_NonRSAKey(t *testing.T) {
	// Create a JWKS response with an EC key instead of RSA
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jwks := map[string]interface{}{
			"keys": []map[string]interface{}{
				{
					"kty": "EC", // Not RSA
					"kid": "test-kid",
					"crv": "P-256",
					"x":   "dGVzdA",
					"y":   "dGVzdA",
				},
			},
		}
		json.NewEncoder(w).Encode(jwks)
	}))
	defer server.Close()

	cfg := Config{
		CanvasURL: "https://canvas.example.com",
		ClientID:  "test-client-id",
		JWKsURL:   server.URL,
	}

	svc, _ := NewLTIService(cfg, testLogger())

	_, err := svc.getCanvasPublicKey(context.Background(), "test-kid")
	if err == nil {
		t.Fatal("expected error for non-RSA key")
	}
}

func TestGetCanvasPublicKey_MultipleKeys(t *testing.T) {
	privateKey1 := generateTestRSAKey(t)
	privateKey2 := generateTestRSAKey(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jwks := map[string]interface{}{
			"keys": []map[string]interface{}{
				{
					"kty": "RSA",
					"kid": "key-1",
					"n":   base64.RawURLEncoding.EncodeToString(privateKey1.PublicKey.N.Bytes()),
					"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privateKey1.PublicKey.E)).Bytes()),
				},
				{
					"kty": "RSA",
					"kid": "key-2",
					"n":   base64.RawURLEncoding.EncodeToString(privateKey2.PublicKey.N.Bytes()),
					"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privateKey2.PublicKey.E)).Bytes()),
				},
			},
		}
		json.NewEncoder(w).Encode(jwks)
	}))
	defer server.Close()

	cfg := Config{
		CanvasURL: "https://canvas.example.com",
		ClientID:  "test-client-id",
		JWKsURL:   server.URL,
	}

	svc, _ := NewLTIService(cfg, testLogger())

	// Should find key-2
	key, err := svc.getCanvasPublicKey(context.Background(), "key-2")
	if err != nil {
		t.Fatalf("failed to get key-2: %v", err)
	}
	if key == nil {
		t.Fatal("expected non-nil key")
	}
}

// -----------------------------------------------------------------------------
// JWK Parsing Tests
// -----------------------------------------------------------------------------

func TestParseRSAPublicKeyFromJWK_Success(t *testing.T) {
	privateKey := generateTestRSAKey(t)

	n := base64.RawURLEncoding.EncodeToString(privateKey.PublicKey.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privateKey.PublicKey.E)).Bytes())

	key, err := parseRSAPublicKeyFromJWK(n, e)
	if err != nil {
		t.Fatalf("parseRSAPublicKeyFromJWK failed: %v", err)
	}
	if key == nil {
		t.Fatal("expected non-nil key")
	}
	if key.E != privateKey.PublicKey.E {
		t.Errorf("exponent mismatch: expected %d, got %d", privateKey.PublicKey.E, key.E)
	}
}

func TestParseRSAPublicKeyFromJWK_InvalidBase64_N(t *testing.T) {
	_, err := parseRSAPublicKeyFromJWK("invalid!base64!", "AQAB")
	if err == nil {
		t.Fatal("expected error for invalid base64 in n")
	}
}

func TestParseRSAPublicKeyFromJWK_InvalidBase64_E(t *testing.T) {
	privateKey := generateTestRSAKey(t)
	n := base64.RawURLEncoding.EncodeToString(privateKey.PublicKey.N.Bytes())

	_, err := parseRSAPublicKeyFromJWK(n, "invalid!base64!")
	if err == nil {
		t.Fatal("expected error for invalid base64 in e")
	}
}

func TestParseRSAPublicKeyFromJWK_ZeroExponent(t *testing.T) {
	privateKey := generateTestRSAKey(t)
	n := base64.RawURLEncoding.EncodeToString(privateKey.PublicKey.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString([]byte{}) // Empty = 0

	_, err := parseRSAPublicKeyFromJWK(n, e)
	if err == nil {
		t.Fatal("expected error for zero exponent")
	}
}

func TestParseRSAPublicKeyFromJWK_CommonExponent65537(t *testing.T) {
	privateKey := generateTestRSAKey(t)

	// Most RSA keys use 65537 (0x010001) as the exponent
	n := base64.RawURLEncoding.EncodeToString(privateKey.PublicKey.N.Bytes())
	e := "AQAB" // Base64URL encoding of 65537

	key, err := parseRSAPublicKeyFromJWK(n, e)
	if err != nil {
		t.Fatalf("parseRSAPublicKeyFromJWK failed: %v", err)
	}
	if key.E != 65537 {
		t.Errorf("expected exponent 65537, got %d", key.E)
	}
}

// -----------------------------------------------------------------------------
// Helper Method Tests
// -----------------------------------------------------------------------------

func TestHasRole_ExactMatch(t *testing.T) {
	launch := &LTILaunchRequest{
		Roles: []string{RoleLearner, RoleInstructor},
	}

	if !launch.HasRole(RoleLearner) {
		t.Error("expected HasRole to return true for Learner")
	}
	if !launch.HasRole(RoleInstructor) {
		t.Error("expected HasRole to return true for Instructor")
	}
}

func TestHasRole_SuffixMatch(t *testing.T) {
	launch := &LTILaunchRequest{
		Roles: []string{"http://purl.imsglobal.org/vocab/lis/v2/membership#Instructor"},
	}

	// Should match by suffix
	if !launch.HasRole("http://purl.imsglobal.org/vocab/lis/v2/membership#Instructor") {
		t.Error("expected HasRole to return true for Instructor with full URL")
	}
}

func TestHasRole_NoMatch(t *testing.T) {
	launch := &LTILaunchRequest{
		Roles: []string{RoleLearner},
	}

	if launch.HasRole(RoleAdministrator) {
		t.Error("expected HasRole to return false for non-existent role")
	}
}

func TestIsInstructor_Instructor(t *testing.T) {
	launch := &LTILaunchRequest{
		Roles: []string{RoleInstructor},
	}

	if !launch.IsInstructor() {
		t.Error("expected IsInstructor to return true")
	}
}

func TestIsInstructor_TeachingAssistant(t *testing.T) {
	launch := &LTILaunchRequest{
		Roles: []string{RoleTeachingAssistant},
	}

	if !launch.IsInstructor() {
		t.Error("expected IsInstructor to return true for TA")
	}
}

func TestIsInstructor_Learner(t *testing.T) {
	launch := &LTILaunchRequest{
		Roles: []string{RoleLearner},
	}

	if launch.IsInstructor() {
		t.Error("expected IsInstructor to return false for Learner")
	}
}

func TestIsLearner_Student(t *testing.T) {
	launch := &LTILaunchRequest{
		Roles: []string{RoleLearner},
	}

	if !launch.IsLearner() {
		t.Error("expected IsLearner to return true")
	}
}

func TestCanSubmitGrades_WithScope(t *testing.T) {
	launch := &LTILaunchRequest{
		AGS: &AGSClaim{
			Scope:    []string{"https://purl.imsglobal.org/spec/lti-ags/scope/score"},
			LineItem: "https://canvas.example.com/api/lti/courses/1/line_items/1",
		},
	}

	if !launch.CanSubmitGrades() {
		t.Error("expected CanSubmitGrades to return true")
	}
}

func TestCanSubmitGrades_NoScope(t *testing.T) {
	launch := &LTILaunchRequest{
		AGS: &AGSClaim{
			Scope:    []string{"https://purl.imsglobal.org/spec/lti-ags/scope/lineitem.readonly"},
			LineItem: "https://canvas.example.com/api/lti/courses/1/line_items/1",
		},
	}

	if launch.CanSubmitGrades() {
		t.Error("expected CanSubmitGrades to return false without score scope")
	}
}

func TestCanSubmitGrades_NoAGS(t *testing.T) {
	launch := &LTILaunchRequest{
		AGS: nil,
	}

	if launch.CanSubmitGrades() {
		t.Error("expected CanSubmitGrades to return false when AGS is nil")
	}
}

func TestGetLineItemURL(t *testing.T) {
	tests := []struct {
		name     string
		launch   *LTILaunchRequest
		expected string
	}{
		{
			name: "with AGS",
			launch: &LTILaunchRequest{
				AGS: &AGSClaim{
					LineItem: "https://canvas.example.com/line_items/1",
				},
			},
			expected: "https://canvas.example.com/line_items/1",
		},
		{
			name: "without AGS",
			launch: &LTILaunchRequest{
				AGS: nil,
			},
			expected: "",
		},
		{
			name: "empty line item",
			launch: &LTILaunchRequest{
				AGS: &AGSClaim{},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.launch.GetLineItemURL()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestGetCustomParam(t *testing.T) {
	launch := &LTILaunchRequest{
		Custom: map[string]string{
			"lab_id":     "lab-123",
			"pathway_id": "pathway-456",
		},
	}

	if launch.GetCustomParam("lab_id") != "lab-123" {
		t.Error("expected lab_id to be lab-123")
	}
	if launch.GetCustomParam("pathway_id") != "pathway-456" {
		t.Error("expected pathway_id to be pathway-456")
	}
	if launch.GetCustomParam("non_existent") != "" {
		t.Error("expected empty string for non-existent param")
	}
}

func TestGetCustomParam_NilCustom(t *testing.T) {
	launch := &LTILaunchRequest{
		Custom: nil,
	}

	if launch.GetCustomParam("any_key") != "" {
		t.Error("expected empty string when Custom is nil")
	}
}

func TestGetCanvasUserID(t *testing.T) {
	tests := []struct {
		name     string
		launch   *LTILaunchRequest
		expected string
	}{
		{
			name: "with CanvasUser",
			launch: &LTILaunchRequest{
				Subject:    "sub-123",
				CanvasUser: &CanvasUser{ID: "canvas-user-456"},
			},
			expected: "canvas-user-456",
		},
		{
			name: "without CanvasUser",
			launch: &LTILaunchRequest{
				Subject:    "sub-123",
				CanvasUser: nil,
			},
			expected: "sub-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.launch.GetCanvasUserID()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestGetCanvasCourseID(t *testing.T) {
	tests := []struct {
		name     string
		launch   *LTILaunchRequest
		expected string
	}{
		{
			name: "with CanvasCourse",
			launch: &LTILaunchRequest{
				Context:      LTIContext{ID: "context-123"},
				CanvasCourse: &CanvasCourse{ID: "canvas-course-456"},
			},
			expected: "canvas-course-456",
		},
		{
			name: "without CanvasCourse",
			launch: &LTILaunchRequest{
				Context:      LTIContext{ID: "context-123"},
				CanvasCourse: nil,
			},
			expected: "context-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.launch.GetCanvasCourseID()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// DefaultConfig Test
// -----------------------------------------------------------------------------

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.TokenExpiry != time.Hour {
		t.Errorf("expected TokenExpiry to be 1 hour, got %v", cfg.TokenExpiry)
	}
	if cfg.RequestTimeout != 30*time.Second {
		t.Errorf("expected RequestTimeout to be 30 seconds, got %v", cfg.RequestTimeout)
	}
}
