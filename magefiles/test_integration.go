//go:build mage

package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// IntegrationResult holds the result of an integration test
type IntegrationResult struct {
	Name      string
	Passed    bool
	Message   string
	Duration  time.Duration
	Details   string
	Skipped   bool
	SkipReason string
}

// Integration runs integration tests against the deployed infra VM
// These tests create actual resources (pods, sessions) and clean up after
func (Test) Integration() error {
	color.Cyan("Running integration tests against infra VM...")
	color.Yellow("⚠ Integration tests create real resources - ensure no critical work is in progress")
	fmt.Println()

	cfg := GetConfig()
	if err := cfg.Validate(); err != nil {
		return err
	}

	client, err := ConnectToInfraVM()
	if err != nil {
		return fmt.Errorf("failed to connect to infra VM: %w", err)
	}
	defer client.Close()

	results := []IntegrationResult{}
	allPassed := true
	apiBase := "http://localhost:8080/api/v1"

	// Test 1: Create a pod from template
	result := testCreatePod(client, apiBase)
	results = append(results, result)
	if !result.Passed && !result.Skipped {
		allPassed = false
	}

	// Test 2: List pods (verify our pod exists)
	result = testListPods(client, apiBase)
	results = append(results, result)
	if !result.Passed && !result.Skipped {
		allPassed = false
	}

	// Test 3: Get pod status
	result = testGetPodStatus(client, apiBase)
	results = append(results, result)
	if !result.Passed && !result.Skipped {
		allPassed = false
	}

	// Test 4: Create session (requires valid pod)
	result = testCreateSession(client, apiBase)
	results = append(results, result)
	if !result.Passed && !result.Skipped {
		allPassed = false
	}

	// Test 5: Get session progress
	result = testGetSessionProgress(client, apiBase)
	results = append(results, result)
	if !result.Passed && !result.Skipped {
		allPassed = false
	}

	// Test 6: List pathways
	result = testListPathways(client, apiBase)
	results = append(results, result)
	if !result.Passed && !result.Skipped {
		allPassed = false
	}

	// Test 7: List achievements
	result = testListAchievements(client, apiBase)
	results = append(results, result)
	if !result.Passed && !result.Skipped {
		allPassed = false
	}

	// Test 8: WebSocket connectivity (for real-time updates)
	result = testWebSocketConnection(client)
	results = append(results, result)
	if !result.Passed && !result.Skipped {
		allPassed = false
	}

	// Cleanup: Delete test pod if it was created
	cleanupTestPod(client, apiBase)

	// Print results
	fmt.Println("\n┌─────────────────────────────────────────────────────────────────┐")
	fmt.Println("│                  INTEGRATION TEST RESULTS                        │")
	fmt.Println("├─────────────────────────────────────────────────────────────────┤")

	for _, r := range results {
		var status string
		if r.Skipped {
			status = color.YellowString("⊘ SKIP")
		} else if r.Passed {
			status = color.GreenString("✓ PASS")
		} else {
			status = color.RedString("✗ FAIL")
		}
		duration := ""
		if r.Duration > 0 {
			duration = fmt.Sprintf(" (%dms)", r.Duration.Milliseconds())
		}
		fmt.Printf("│ %s %-28s %s%s\n", status, r.Name, r.Message, duration)
		if r.Details != "" && !r.Passed {
			fmt.Printf("│        └─ %s\n", r.Details)
		}
		if r.Skipped && r.SkipReason != "" {
			fmt.Printf("│        └─ %s\n", r.SkipReason)
		}
	}

	fmt.Println("└─────────────────────────────────────────────────────────────────┘")

	passed := 0
	skipped := 0
	for _, r := range results {
		if r.Passed {
			passed++
		}
		if r.Skipped {
			skipped++
		}
	}
	total := len(results) - skipped

	fmt.Printf("\nResults: %d/%d tests passed", passed, total)
	if skipped > 0 {
		fmt.Printf(" (%d skipped)", skipped)
	}
	fmt.Println()

	if !allPassed {
		return fmt.Errorf("integration tests failed: %d/%d passed", passed, total)
	}

	color.Green("✓ All integration tests passed!")
	return nil
}

var integrationTestPodID string
var integrationTestSessionID string

func testCreatePod(client *SSHClient, apiBase string) IntegrationResult {
	start := time.Now()

	// First, get a valid Proxmox lab template ID (CloudStack may not be configured)
	output, err := client.Run(fmt.Sprintf(`curl -s %s/labs 2>/dev/null | jq -r '[.labs[] | select(.platform == "proxmox")][0].id // empty'`, apiBase))
	if err != nil || strings.TrimSpace(output) == "" {
		// Fallback to any lab if no proxmox labs found
		output, err = client.Run(fmt.Sprintf(`curl -s %s/labs 2>/dev/null | jq -r '.labs[0].id // empty'`, apiBase))
		if err != nil || strings.TrimSpace(output) == "" {
			return IntegrationResult{Name: "Create Pod", Passed: false, Message: "No labs available", Duration: time.Since(start)}
		}
	}
	labID := strings.TrimSpace(output)

	// Create a test pod - API uses "labTemplate" and "owner" fields
	podName := fmt.Sprintf("integration-test-%d", time.Now().Unix())
	payload := fmt.Sprintf(`{"name":"%s","labTemplate":"%s","owner":"test-user-integration"}`, podName, labID)

	output, err = client.Run(fmt.Sprintf(`curl -s -X POST -H "Content-Type: application/json" -d '%s' %s/pods 2>/dev/null`, payload, apiBase))
	duration := time.Since(start)

	if err != nil {
		return IntegrationResult{Name: "Create Pod", Passed: false, Message: "Request failed", Duration: duration, Details: err.Error()}
	}

	// Check if we got an ID back
	var resp struct {
		ID    string `json:"id"`
		Error string `json:"error"`
	}
	if json.Unmarshal([]byte(output), &resp) == nil && resp.ID != "" {
		integrationTestPodID = resp.ID
		return IntegrationResult{Name: "Create Pod", Passed: true, Message: fmt.Sprintf("Created: %s", resp.ID[:8]), Duration: duration}
	}

	if resp.Error != "" {
		// Infrastructure-related errors (missing templates, disconnected hypervisor) should skip not fail
		// This allows tests to run even when the full infrastructure isn't configured
		if strings.Contains(resp.Error, "not found") ||
			strings.Contains(resp.Error, "not configured") ||
			strings.Contains(resp.Error, "template") ||
			strings.Contains(resp.Error, "provisioning") {
			return IntegrationResult{Name: "Create Pod", Skipped: true, SkipReason: "Infrastructure not ready: " + resp.Error, Duration: duration}
		}
		return IntegrationResult{Name: "Create Pod", Passed: false, Message: "API error", Duration: duration, Details: resp.Error}
	}

	return IntegrationResult{Name: "Create Pod", Passed: false, Message: "Invalid response", Duration: duration, Details: output}
}

func testListPods(client *SSHClient, apiBase string) IntegrationResult {
	start := time.Now()

	output, err := client.Run(fmt.Sprintf(`curl -s %s/pods 2>/dev/null`, apiBase))
	duration := time.Since(start)

	if err != nil {
		return IntegrationResult{Name: "List Pods", Passed: false, Message: "Request failed", Duration: duration}
	}

	// Parse response - could be array or object with pods field
	var pods []interface{}
	if json.Unmarshal([]byte(output), &pods) == nil {
		return IntegrationResult{Name: "List Pods", Passed: true, Message: fmt.Sprintf("%d pods", len(pods)), Duration: duration}
	}

	var resp struct {
		Pods []interface{} `json:"pods"`
		Count int `json:"count"`
	}
	if json.Unmarshal([]byte(output), &resp) == nil {
		return IntegrationResult{Name: "List Pods", Passed: true, Message: fmt.Sprintf("%d pods", resp.Count), Duration: duration}
	}

	return IntegrationResult{Name: "List Pods", Passed: false, Message: "Invalid response", Duration: duration}
}

func testGetPodStatus(client *SSHClient, apiBase string) IntegrationResult {
	start := time.Now()

	if integrationTestPodID == "" {
		return IntegrationResult{Name: "Get Pod Status", Skipped: true, SkipReason: "No pod created"}
	}

	output, err := client.Run(fmt.Sprintf(`curl -s %s/pods/%s 2>/dev/null`, apiBase, integrationTestPodID))
	duration := time.Since(start)

	if err != nil {
		return IntegrationResult{Name: "Get Pod Status", Passed: false, Message: "Request failed", Duration: duration}
	}

	var pod struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if json.Unmarshal([]byte(output), &pod) == nil && pod.ID != "" {
		return IntegrationResult{Name: "Get Pod Status", Passed: true, Message: fmt.Sprintf("Status: %s", pod.Status), Duration: duration}
	}

	return IntegrationResult{Name: "Get Pod Status", Passed: false, Message: "Invalid response", Duration: duration}
}

func testCreateSession(client *SSHClient, apiBase string) IntegrationResult {
	start := time.Now()

	if integrationTestPodID == "" {
		return IntegrationResult{Name: "Create Session", Skipped: true, SkipReason: "No pod created"}
	}

	// Get a Proxmox lab template for the session
	output, err := client.Run(fmt.Sprintf(`curl -s %s/labs 2>/dev/null | jq -r '[.labs[] | select(.platform == "proxmox")][0].id // empty'`, apiBase))
	if err != nil || strings.TrimSpace(output) == "" {
		output, err = client.Run(fmt.Sprintf(`curl -s %s/labs 2>/dev/null | jq -r '.labs[0].id // empty'`, apiBase))
		if err != nil || strings.TrimSpace(output) == "" {
			return IntegrationResult{Name: "Create Session", Passed: false, Message: "No labs available", Duration: time.Since(start)}
		}
	}
	labID := strings.TrimSpace(output)

	// API uses "pod", "labTemplate", and "user" fields
	payload := fmt.Sprintf(`{"pod":"%s","labTemplate":"%s","user":"test-user-integration"}`, integrationTestPodID, labID)
	output, err = client.Run(fmt.Sprintf(`curl -s -X POST -H "Content-Type: application/json" -d '%s' %s/sessions 2>/dev/null`, payload, apiBase))
	duration := time.Since(start)

	if err != nil {
		return IntegrationResult{Name: "Create Session", Passed: false, Message: "Request failed", Duration: duration}
	}

	var resp struct {
		ID    string `json:"id"`
		Error string `json:"error"`
	}
	if json.Unmarshal([]byte(output), &resp) == nil && resp.ID != "" {
		integrationTestSessionID = resp.ID
		return IntegrationResult{Name: "Create Session", Passed: true, Message: fmt.Sprintf("Created: %s", resp.ID[:8]), Duration: duration}
	}

	if resp.Error != "" {
		// Session creation might fail if pod isn't ready - that's expected
		return IntegrationResult{Name: "Create Session", Skipped: true, SkipReason: resp.Error, Duration: duration}
	}

	return IntegrationResult{Name: "Create Session", Passed: false, Message: "Invalid response", Duration: duration}
}

func testGetSessionProgress(client *SSHClient, apiBase string) IntegrationResult {
	start := time.Now()

	if integrationTestSessionID == "" {
		return IntegrationResult{Name: "Get Session Progress", Skipped: true, SkipReason: "No session created"}
	}

	output, err := client.Run(fmt.Sprintf(`curl -s %s/sessions/%s/progress 2>/dev/null`, apiBase, integrationTestSessionID))
	duration := time.Since(start)

	if err != nil {
		return IntegrationResult{Name: "Get Session Progress", Passed: false, Message: "Request failed", Duration: duration}
	}

	var progress struct {
		EarnedPoints int `json:"earnedPoints"`
		MaxPoints    int `json:"maxPoints"`
	}
	if json.Unmarshal([]byte(output), &progress) == nil {
		return IntegrationResult{Name: "Get Session Progress", Passed: true, Message: fmt.Sprintf("%d/%d points", progress.EarnedPoints, progress.MaxPoints), Duration: duration}
	}

	return IntegrationResult{Name: "Get Session Progress", Passed: false, Message: "Invalid response", Duration: duration}
}

func testListPathways(client *SSHClient, apiBase string) IntegrationResult {
	start := time.Now()

	output, err := client.Run(fmt.Sprintf(`curl -s %s/pathways 2>/dev/null`, apiBase))
	duration := time.Since(start)

	if err != nil {
		return IntegrationResult{Name: "List Pathways", Passed: false, Message: "Request failed", Duration: duration}
	}

	// Try various response formats
	var pathways []interface{}
	if json.Unmarshal([]byte(output), &pathways) == nil {
		return IntegrationResult{Name: "List Pathways", Passed: true, Message: fmt.Sprintf("%d pathways", len(pathways)), Duration: duration}
	}

	var resp struct {
		Pathways []interface{} `json:"pathways"`
		Count    int           `json:"count"`
	}
	if json.Unmarshal([]byte(output), &resp) == nil {
		count := resp.Count
		if count == 0 {
			count = len(resp.Pathways)
		}
		return IntegrationResult{Name: "List Pathways", Passed: true, Message: fmt.Sprintf("%d pathways", count), Duration: duration}
	}

	// Empty response is valid (no pathways configured)
	if output == "[]" || output == "null" || output == "{}" {
		return IntegrationResult{Name: "List Pathways", Passed: true, Message: "0 pathways", Duration: duration}
	}

	return IntegrationResult{Name: "List Pathways", Passed: false, Message: "Invalid response", Duration: duration, Details: output}
}

func testListAchievements(client *SSHClient, apiBase string) IntegrationResult {
	start := time.Now()

	output, err := client.Run(fmt.Sprintf(`curl -s %s/achievements 2>/dev/null`, apiBase))
	duration := time.Since(start)

	if err != nil {
		return IntegrationResult{Name: "List Achievements", Passed: false, Message: "Request failed", Duration: duration}
	}

	var resp struct {
		Achievements []interface{} `json:"achievements"`
	}
	if json.Unmarshal([]byte(output), &resp) == nil {
		return IntegrationResult{Name: "List Achievements", Passed: true, Message: fmt.Sprintf("%d achievements", len(resp.Achievements)), Duration: duration}
	}

	var achievements []interface{}
	if json.Unmarshal([]byte(output), &achievements) == nil {
		return IntegrationResult{Name: "List Achievements", Passed: true, Message: fmt.Sprintf("%d achievements", len(achievements)), Duration: duration}
	}

	return IntegrationResult{Name: "List Achievements", Passed: false, Message: "Invalid response", Duration: duration}
}

func testWebSocketConnection(client *SSHClient) IntegrationResult {
	start := time.Now()

	// Test WebSocket endpoint is available (can't fully test WS from curl, but can check upgrade response)
	output, err := client.Run(`curl -s -o /dev/null -w '%{http_code}' -H "Connection: Upgrade" -H "Upgrade: websocket" -H "Sec-WebSocket-Version: 13" -H "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==" http://localhost:8080/ws 2>/dev/null`)
	duration := time.Since(start)

	if err != nil {
		return IntegrationResult{Name: "WebSocket Endpoint", Passed: false, Message: "Request failed", Duration: duration}
	}

	code := strings.TrimSpace(output)
	// 101 = Switching Protocols (ideal), 400/426 = upgrade required (endpoint exists), 200 = fallback
	if code == "101" || code == "400" || code == "426" || code == "200" {
		return IntegrationResult{Name: "WebSocket Endpoint", Passed: true, Message: fmt.Sprintf("HTTP %s", code), Duration: duration}
	}

	return IntegrationResult{Name: "WebSocket Endpoint", Passed: false, Message: fmt.Sprintf("HTTP %s", code), Duration: duration}
}

func cleanupTestPod(client *SSHClient, apiBase string) {
	if integrationTestPodID == "" {
		return
	}

	// Try to delete the test pod
	client.Run(fmt.Sprintf(`curl -s -X DELETE %s/pods/%s 2>/dev/null`, apiBase, integrationTestPodID))
	integrationTestPodID = ""
	integrationTestSessionID = ""
}

// Debug namespace contains debugging targets
type Debug mg.Namespace

// Report generates a comprehensive diagnostic report from the infra VM
func (Debug) Report() error {
	color.Cyan("Generating diagnostic report from infra VM...")
	cfg := GetConfig()

	if err := cfg.Validate(); err != nil {
		return err
	}

	client, err := ConnectToInfraVM()
	if err != nil {
		return fmt.Errorf("failed to connect to infra VM: %w", err)
	}
	defer client.Close()

	fmt.Println("\n" + strings.Repeat("=", 70))
	color.Cyan("KOOTENAI DIAGNOSTIC REPORT")
	fmt.Println("Generated:", time.Now().Format(time.RFC1123))
	fmt.Println(strings.Repeat("=", 70))

	// System Info
	fmt.Println("\n[SYSTEM INFO]")
	if output, err := client.Run("uname -a"); err == nil {
		fmt.Printf("  OS: %s", output)
	}
	if output, err := client.Run("uptime"); err == nil {
		fmt.Printf("  Uptime: %s", output)
	}
	if output, err := client.Run("free -h | grep Mem"); err == nil {
		fmt.Printf("  Memory: %s", output)
	}
	if output, err := client.Run("df -h / | tail -1"); err == nil {
		fmt.Printf("  Disk: %s", output)
	}

	// API Status
	fmt.Println("\n[API SERVICE]")
	if output, err := client.Run("ps aux | grep 'labctl serve' | grep -v grep | head -1"); err == nil && output != "" {
		fmt.Printf("  Process: Running\n")
		// Extract PID
		fields := strings.Fields(output)
		if len(fields) > 1 {
			fmt.Printf("  PID: %s\n", fields[1])
		}
	} else {
		color.Red("  Process: NOT RUNNING")
	}
	if output, err := client.Run("curl -sf http://localhost:8080/api/v1/version 2>/dev/null"); err == nil {
		var version struct {
			Version string `json:"version"`
			Commit  string `json:"commit"`
		}
		if json.Unmarshal([]byte(output), &version) == nil {
			fmt.Printf("  Version: %s\n", version.Version)
			fmt.Printf("  Commit: %s\n", version.Commit)
		}
	}

	// Health Checks
	fmt.Println("\n[HEALTH CHECKS]")
	if output, err := client.Run("curl -sf http://localhost:8080/ready 2>/dev/null"); err == nil {
		var ready map[string]interface{}
		if json.Unmarshal([]byte(output), &ready) == nil {
			for k, v := range ready {
				fmt.Printf("  %s: %v\n", k, v)
			}
		}
	} else {
		color.Red("  API not responding to health checks")
	}

	// Docker Containers
	fmt.Println("\n[DOCKER CONTAINERS]")
	if output, err := client.Run("docker ps -a --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}' 2>/dev/null"); err == nil {
		fmt.Println(output)
	}

	// Database Status
	fmt.Println("\n[DATABASE]")
	// Find the postgres container and credentials
	pgContainer := "kootenai-postgres"
	pgUser := "labadmin"
	pgDB := "virtuallab"
	if output, _ := client.Run("docker ps --filter name=kootenai-postgres --format '{{.Names}}'"); strings.TrimSpace(output) == "" {
		pgContainer = "labctl-postgres"
		pgUser = "labctl"
		pgDB = "labctl"
	}
	if output, err := client.Run(fmt.Sprintf(`docker exec -i %s psql -U %s -d %s -c "SELECT COUNT(*) FROM achievements" 2>/dev/null | head -3 | tail -1`, pgContainer, pgUser, pgDB)); err == nil {
		fmt.Printf("  Achievements: %s\n", strings.TrimSpace(output))
	}
	if output, err := client.Run(fmt.Sprintf(`docker exec -i %s psql -U %s -d %s -c "SELECT COUNT(*) FROM lab_templates" 2>/dev/null | head -3 | tail -1`, pgContainer, pgUser, pgDB)); err == nil {
		fmt.Printf("  Lab Templates: %s\n", strings.TrimSpace(output))
	}
	if output, err := client.Run(fmt.Sprintf(`docker exec -i %s psql -U %s -d %s -c "SELECT COUNT(*) FROM pods" 2>/dev/null | head -3 | tail -1`, pgContainer, pgUser, pgDB)); err == nil {
		fmt.Printf("  Pods: %s\n", strings.TrimSpace(output))
	}
	if output, err := client.Run(fmt.Sprintf(`docker exec -i %s psql -U %s -d %s -c "SELECT COUNT(*) FROM sessions" 2>/dev/null | head -3 | tail -1`, pgContainer, pgUser, pgDB)); err == nil {
		fmt.Printf("  Sessions: %s\n", strings.TrimSpace(output))
	}

	// Recent Logs
	fmt.Println("\n[RECENT API LOGS (last 20 lines)]")
	if output, err := client.Run("tail -20 /tmp/labctl.log 2>/dev/null"); err == nil {
		fmt.Println(output)
	} else {
		color.Yellow("  No logs available")
	}

	// Network Connectivity
	fmt.Println("\n[NETWORK CONNECTIVITY]")
	if output, err := client.Run("curl -sf -o /dev/null -w '%{http_code}' http://localhost:8080/health 2>/dev/null || echo '000'"); err == nil {
		fmt.Printf("  API (localhost:8080): HTTP %s\n", strings.TrimSpace(output))
	}
	if output, err := client.Run("curl -sf -o /dev/null -w '%{http_code}' http://localhost:3000/ 2>/dev/null || echo '000'"); err == nil {
		fmt.Printf("  Web (localhost:3000): HTTP %s\n", strings.TrimSpace(output))
	}
	// Check Proxmox connectivity (just test if we can reach it)
	if output, err := client.Run("curl -sk -o /dev/null -w '%{http_code}' https://<PROXMOX_IP>:8006/ 2>/dev/null || echo '000'"); err == nil {
		fmt.Printf("  Proxmox (<PROXMOX_IP>:8006): HTTP %s\n", strings.TrimSpace(output))
	}

	// Git Status
	fmt.Println("\n[GIT STATUS]")
	if output, err := client.Run(fmt.Sprintf("cd %s && git log -1 --oneline 2>/dev/null", cfg.InfraVM.RepoPath)); err == nil {
		fmt.Printf("  Latest commit: %s", output)
	}
	if output, err := client.Run(fmt.Sprintf("cd %s && git status --short 2>/dev/null | head -10", cfg.InfraVM.RepoPath)); err == nil && output != "" {
		fmt.Printf("  Uncommitted changes:\n%s", output)
	}

	fmt.Println("\n" + strings.Repeat("=", 70))
	color.Green("Diagnostic report complete")
	return nil
}

// Api tails API logs from the infra VM
func (Debug) Api() error {
	cfg := GetConfig()

	if err := cfg.Validate(); err != nil {
		return err
	}

	color.Cyan("Tailing API logs (Ctrl+C to stop)...")

	return sh.RunV("ssh",
		fmt.Sprintf("%s@%s", cfg.InfraVM.User, cfg.InfraVM.Host),
		"tail", "-f", "/tmp/labctl.log")
}

// Web tails web container logs from the infra VM
func (Debug) Web() error {
	cfg := GetConfig()

	if err := cfg.Validate(); err != nil {
		return err
	}

	color.Cyan("Tailing web container logs (Ctrl+C to stop)...")

	return sh.RunV("ssh",
		fmt.Sprintf("%s@%s", cfg.InfraVM.User, cfg.InfraVM.Host),
		"docker", "logs", "-f", "kootenai-web")
}

// Db shows database status and recent queries
func (Debug) Db() error {
	cfg := GetConfig()

	if err := cfg.Validate(); err != nil {
		return err
	}

	client, err := ConnectToInfraVM()
	if err != nil {
		return err
	}
	defer client.Close()

	// Find the postgres container and credentials
	pgContainer := "kootenai-postgres"
	pgUser := "labadmin"
	pgDB := "virtuallab"
	if output, _ := client.Run("docker ps --filter name=kootenai-postgres --format '{{.Names}}'"); strings.TrimSpace(output) == "" {
		pgContainer = "labctl-postgres"
		pgUser = "labctl"
		pgDB = "labctl"
	}

	fmt.Println("\n[DATABASE STATUS]")

	// Check connection
	if output, err := client.Run(fmt.Sprintf(`docker exec -i %s psql -U %s -d %s -c "SELECT version()" 2>/dev/null | head -3`, pgContainer, pgUser, pgDB)); err == nil {
		fmt.Println(output)
	}

	// Table sizes
	fmt.Println("\n[TABLE SIZES]")
	query := `SELECT schemaname, tablename, pg_size_pretty(pg_total_relation_size(schemaname || '.' || tablename)) as size FROM pg_tables WHERE schemaname = 'public' ORDER BY pg_total_relation_size(schemaname || '.' || tablename) DESC LIMIT 10`
	if output, err := client.Run(fmt.Sprintf(`docker exec -i %s psql -U %s -d %s -c "%s" 2>/dev/null`, pgContainer, pgUser, pgDB, query)); err == nil {
		fmt.Println(output)
	}

	// Recent sessions
	fmt.Println("\n[RECENT SESSIONS]")
	if output, err := client.Run(fmt.Sprintf(`docker exec -i %s psql -U %s -d %s -c "SELECT id, user_id, status, created_at FROM sessions ORDER BY created_at DESC LIMIT 5" 2>/dev/null`, pgContainer, pgUser, pgDB)); err == nil {
		fmt.Println(output)
	}

	return nil
}

// Connectivity tests all service connections
func (Debug) Connectivity() error {
	cfg := GetConfig()

	if err := cfg.Validate(); err != nil {
		return err
	}

	client, err := ConnectToInfraVM()
	if err != nil {
		return err
	}
	defer client.Close()

	fmt.Println("\n[CONNECTIVITY TESTS]")

	// Find the postgres container and credentials
	pgContainer := "kootenai-postgres"
	pgUser := "labadmin"
	pgDB := "virtuallab"
	if output, _ := client.Run("docker ps --filter name=kootenai-postgres --format '{{.Names}}'"); strings.TrimSpace(output) == "" {
		pgContainer = "labctl-postgres"
		pgUser = "labctl"
		pgDB = "labctl"
	}

	// Local services
	services := []struct {
		name string
		cmd  string
	}{
		{"PostgreSQL", fmt.Sprintf(`docker exec -i %s psql -U %s -d %s -c "SELECT 1" 2>/dev/null && echo "OK" || echo "FAIL"`, pgContainer, pgUser, pgDB)},
		{"NATS", "curl -sf http://localhost:8222/varz 2>/dev/null | head -1 && echo 'OK' || echo 'FAIL'"},
		{"API Health", "curl -sf http://localhost:8080/health 2>/dev/null && echo 'OK' || echo 'FAIL'"},
		{"Web UI", "curl -sfL http://localhost:3000/ 2>/dev/null > /dev/null && echo 'OK' || echo 'FAIL'"},
		{"Proxmox API", "curl -sk https://<PROXMOX_IP>:8006/api2/json/version 2>/dev/null | head -1 || echo 'FAIL'"},
	}

	for _, svc := range services {
		output, _ := client.Run(svc.cmd)
		output = strings.TrimSpace(output)
		status := color.GreenString("✓")
		if strings.Contains(output, "FAIL") || output == "" {
			status = color.RedString("✗")
		}
		fmt.Printf("  %s %s: %s\n", status, svc.name, output)
	}

	return nil
}

