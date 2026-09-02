//go:build mage

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/magefile/mage/sh"
)

// E2e runs end-to-end tests using Playwright against the deployed infra VM
func (Test) E2e() error {
	color.Cyan("Running E2E tests with Playwright...")
	cfg := GetConfig()

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	e2eDir := fmt.Sprintf("%s/tests/e2e", cfg.ProjectRoot)

	// Check if tests/e2e exists
	if _, err := os.Stat(e2eDir); os.IsNotExist(err) {
		return fmt.Errorf("E2E test directory not found: %s", e2eDir)
	}

	if err := os.Chdir(e2eDir); err != nil {
		return err
	}

	// Install dependencies if needed
	if _, err := os.Stat("node_modules"); os.IsNotExist(err) {
		color.Cyan("Installing E2E test dependencies...")
		if err := sh.RunV("npm", "install"); err != nil {
			return fmt.Errorf("failed to install dependencies: %w", err)
		}
		// Install Playwright browsers
		if err := sh.RunV("npx", "playwright", "install", "chromium"); err != nil {
			return fmt.Errorf("failed to install Playwright browsers: %w", err)
		}
	}

	// Set base URL to infra VM
	baseURL := fmt.Sprintf("http://%s:3000", cfg.InfraVM.Host)
	apiURL := fmt.Sprintf("http://%s:8080", cfg.InfraVM.Host)

	// Run Playwright tests
	env := map[string]string{
		"E2E_BASE_URL": baseURL,
		"E2E_API_URL":  apiURL,
	}

	if err := sh.RunWithV(env, "npx", "playwright", "test"); err != nil {
		return fmt.Errorf("E2E tests failed: %w", err)
	}

	color.Green("✓ E2E tests passed!")
	return nil
}

// E2eReport shows the Playwright HTML test report
func (Test) E2eReport() error {
	cfg := GetConfig()

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	e2eDir := fmt.Sprintf("%s/tests/e2e", cfg.ProjectRoot)

	if err := os.Chdir(e2eDir); err != nil {
		return err
	}

	return sh.RunV("npx", "playwright", "show-report")
}

// E2eUi runs Playwright tests in UI mode for debugging
func (Test) E2eUi() error {
	color.Cyan("Running E2E tests in UI mode...")
	cfg := GetConfig()

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	e2eDir := fmt.Sprintf("%s/tests/e2e", cfg.ProjectRoot)

	if err := os.Chdir(e2eDir); err != nil {
		return err
	}

	baseURL := fmt.Sprintf("http://%s:3000", cfg.InfraVM.Host)
	apiURL := fmt.Sprintf("http://%s:8080", cfg.InfraVM.Host)

	env := map[string]string{
		"E2E_BASE_URL": baseURL,
		"E2E_API_URL":  apiURL,
	}

	return sh.RunWithV(env, "npx", "playwright", "test", "--ui")
}

// SmokeResult holds the result of a smoke test check
type SmokeResult struct {
	Name    string
	Passed  bool
	Message string
	Latency time.Duration
}

// Smoke runs smoke tests against the deployed infra VM
func (Test) Smoke() error {
	color.Cyan("Running smoke tests against infra VM...")
	cfg := GetConfig()

	if err := cfg.Validate(); err != nil {
		return err
	}

	client, err := ConnectToInfraVM()
	if err != nil {
		return fmt.Errorf("failed to connect to infra VM: %w", err)
	}
	defer client.Close()

	results := []SmokeResult{}
	allPassed := true

	// Test 1: API process running
	result := checkAPIProcess(client)
	results = append(results, result)
	if !result.Passed {
		allPassed = false
	}

	// Test 2: API health endpoint
	result = checkAPIHealth(client)
	results = append(results, result)
	if !result.Passed {
		allPassed = false
	}

	// Test 3: API ready endpoint (database, NATS, Proxmox)
	result = checkAPIReady(client)
	results = append(results, result)
	if !result.Passed {
		allPassed = false
	}

	// Test 4: Labs endpoint returns data
	result = checkLabsEndpoint(client)
	results = append(results, result)
	if !result.Passed {
		allPassed = false
	}

	// Test 5: Achievements endpoint returns data
	result = checkAchievementsEndpoint(client)
	results = append(results, result)
	if !result.Passed {
		allPassed = false
	}

	// Test 6: Web container running
	result = checkWebContainer(client)
	results = append(results, result)
	if !result.Passed {
		allPassed = false
	}

	// Test 7: Web UI accessible
	result = checkWebUI(client)
	results = append(results, result)
	if !result.Passed {
		allPassed = false
	}

	// Test 8: Database connectivity
	result = checkDatabase(client)
	results = append(results, result)
	if !result.Passed {
		allPassed = false
	}

	// Print results
	fmt.Println("\n┌─────────────────────────────────────────────────────────────┐")
	fmt.Println("│                    SMOKE TEST RESULTS                        │")
	fmt.Println("├─────────────────────────────────────────────────────────────┤")

	for _, r := range results {
		status := color.GreenString("✓ PASS")
		if !r.Passed {
			status = color.RedString("✗ FAIL")
		}
		latency := ""
		if r.Latency > 0 {
			latency = fmt.Sprintf(" (%dms)", r.Latency.Milliseconds())
		}
		fmt.Printf("│ %s %-25s %s%s\n", status, r.Name, r.Message, latency)
	}

	fmt.Println("└─────────────────────────────────────────────────────────────┘")

	passed := 0
	for _, r := range results {
		if r.Passed {
			passed++
		}
	}

	fmt.Printf("\nResults: %d/%d tests passed\n", passed, len(results))

	if !allPassed {
		return fmt.Errorf("smoke tests failed: %d/%d passed", passed, len(results))
	}

	color.Green("✓ All smoke tests passed!")
	return nil
}

func checkAPIProcess(client *SSHClient) SmokeResult {
	start := time.Now()
	output, err := client.Run("pgrep -f 'labctl serve' > /dev/null && echo 'running' || echo 'stopped'")
	latency := time.Since(start)

	if err != nil || strings.TrimSpace(output) != "running" {
		return SmokeResult{Name: "API Process", Passed: false, Message: "Not running", Latency: latency}
	}
	return SmokeResult{Name: "API Process", Passed: true, Message: "Running", Latency: latency}
}

func checkAPIHealth(client *SSHClient) SmokeResult {
	start := time.Now()
	output, err := client.Run("curl -sf http://localhost:8080/health 2>/dev/null || echo 'FAIL'")
	latency := time.Since(start)

	if err != nil || strings.Contains(output, "FAIL") {
		return SmokeResult{Name: "API Health", Passed: false, Message: "Endpoint not responding", Latency: latency}
	}
	return SmokeResult{Name: "API Health", Passed: true, Message: "OK", Latency: latency}
}

func checkAPIReady(client *SSHClient) SmokeResult {
	start := time.Now()
	output, err := client.Run("curl -sf http://localhost:8080/ready 2>/dev/null")
	latency := time.Since(start)

	if err != nil {
		return SmokeResult{Name: "API Ready", Passed: false, Message: "Endpoint not responding", Latency: latency}
	}

	// Parse the JSON response to check individual components
	var readyResp struct {
		Status string `json:"status"`
	}
	if json.Unmarshal([]byte(output), &readyResp) == nil && (readyResp.Status == "ready" || readyResp.Status == "healthy") {
		return SmokeResult{Name: "API Ready", Passed: true, Message: "All systems ready", Latency: latency}
	}

	return SmokeResult{Name: "API Ready", Passed: false, Message: "Some systems degraded", Latency: latency}
}

func checkLabsEndpoint(client *SSHClient) SmokeResult {
	start := time.Now()
	output, err := client.Run("curl -s http://localhost:8080/api/v1/labs 2>/dev/null")
	latency := time.Since(start)

	if err != nil || output == "" {
		return SmokeResult{Name: "Labs API", Passed: false, Message: "No response", Latency: latency}
	}

	// Check for error response
	var errResp struct {
		Error string `json:"error"`
	}
	if json.Unmarshal([]byte(output), &errResp) == nil && errResp.Error != "" {
		return SmokeResult{Name: "Labs API", Passed: false, Message: errResp.Error, Latency: latency}
	}

	// Try to parse as paginated response with count and labs array
	var paginatedResp struct {
		Count int           `json:"count"`
		Labs  []interface{} `json:"labs"`
	}
	if json.Unmarshal([]byte(output), &paginatedResp) == nil && paginatedResp.Labs != nil {
		return SmokeResult{Name: "Labs API", Passed: true, Message: fmt.Sprintf("%d templates", paginatedResp.Count), Latency: latency}
	}

	// Fallback: try to parse as JSON array (old format)
	var labs []interface{}
	if json.Unmarshal([]byte(output), &labs) == nil {
		return SmokeResult{Name: "Labs API", Passed: true, Message: fmt.Sprintf("%d templates", len(labs)), Latency: latency}
	}

	return SmokeResult{Name: "Labs API", Passed: false, Message: "Invalid JSON", Latency: latency}
}

func checkAchievementsEndpoint(client *SSHClient) SmokeResult {
	start := time.Now()
	output, err := client.Run("curl -s http://localhost:8080/api/v1/achievements 2>/dev/null")
	latency := time.Since(start)

	if err != nil || output == "" {
		return SmokeResult{Name: "Achievements API", Passed: false, Message: "No response", Latency: latency}
	}

	// Try to parse as JSON object with achievements array
	var response struct {
		Achievements []interface{} `json:"achievements"`
	}
	if json.Unmarshal([]byte(output), &response) == nil {
		return SmokeResult{Name: "Achievements API", Passed: true, Message: fmt.Sprintf("%d achievements", len(response.Achievements)), Latency: latency}
	}

	// Try to parse as JSON array directly (fallback)
	var achievements []interface{}
	if json.Unmarshal([]byte(output), &achievements) == nil {
		return SmokeResult{Name: "Achievements API", Passed: true, Message: fmt.Sprintf("%d achievements", len(achievements)), Latency: latency}
	}

	return SmokeResult{Name: "Achievements API", Passed: false, Message: "Invalid JSON", Latency: latency}
}

func checkWebContainer(client *SSHClient) SmokeResult {
	start := time.Now()
	output, err := client.Run("docker ps --filter name=kootenai-web --format '{{.Status}}' 2>/dev/null")
	latency := time.Since(start)

	if err != nil || !strings.Contains(output, "Up") {
		return SmokeResult{Name: "Web Container", Passed: false, Message: "Not running", Latency: latency}
	}
	return SmokeResult{Name: "Web Container", Passed: true, Message: "Running", Latency: latency}
}

func checkWebUI(client *SSHClient) SmokeResult {
	start := time.Now()
	// Don't follow redirects - just check initial response. Accept 200 or redirect codes (301/302)
	output, err := client.Run("curl -s -o /dev/null -w '%{http_code}' http://localhost:3000/ 2>/dev/null")
	latency := time.Since(start)

	code := strings.TrimSpace(output)
	if code == "" {
		code = "000"
	}
	// Accept 200, or 301/302 if server is configured for HTTPS redirect
	if err != nil || (code != "200" && code != "301" && code != "302") {
		return SmokeResult{Name: "Web UI", Passed: false, Message: fmt.Sprintf("HTTP %s", code), Latency: latency}
	}
	return SmokeResult{Name: "Web UI", Passed: true, Message: fmt.Sprintf("HTTP %s", code), Latency: latency}
}

func checkDatabase(client *SSHClient) SmokeResult {
	start := time.Now()
	// Try both container naming conventions and common user/db combos
	// Current setup uses labadmin/kootenai, but fallback to labctl/labctl for old configs
	output, err := client.Run(`(docker exec -i kootenai-postgres psql -U labadmin -d virtuallab -c "SELECT 1" 2>/dev/null || docker exec -i labctl-postgres psql -U labctl -d labctl -c "SELECT 1" 2>/dev/null) | grep -q "1 row" && echo "OK" || echo "FAIL"`)
	latency := time.Since(start)

	if err != nil || strings.TrimSpace(output) != "OK" {
		return SmokeResult{Name: "Database", Passed: false, Message: "Query failed", Latency: latency}
	}
	return SmokeResult{Name: "Database", Passed: true, Message: "Connected", Latency: latency}
}
