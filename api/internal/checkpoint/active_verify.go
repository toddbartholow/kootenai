// Package checkpoint provides checkpoint evaluation and verification
package checkpoint

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/orchestrator"
	"github.com/toddbartholow/kootenai/api/internal/regexutil"
)

// Pre-compiled regexes for input sanitization (avoid per-call compilation).
var (
	rePath        = regexp.MustCompile(`[^a-zA-Z0-9/._\-]`)
	reServiceName = regexp.MustCompile(`[^a-zA-Z0-9\-_.@]`)
	rePackageName = regexp.MustCompile(`[^a-zA-Z0-9\-_.+]`)
	reUsername    = regexp.MustCompile(`[^a-zA-Z0-9\-_]`)
)

// ActiveVerifier performs active verification of checkpoint conditions
// by directly querying VMs rather than relying on passive event detection.
// This provides a secondary verification mechanism that is harder to spoof.
type ActiveVerifier struct {
	orchestrator orchestrator.Client
	logger       *slog.Logger
	config       ActiveVerifyConfig
}

// ActiveVerifyConfig configures the active verifier
type ActiveVerifyConfig struct {
	// Timeout for verification commands
	Timeout time.Duration `yaml:"timeout"`

	// RetryCount for failed verifications
	RetryCount int `yaml:"retry_count"`

	// RetryDelay between retries
	RetryDelay time.Duration `yaml:"retry_delay"`

	// ParallelChecks limits concurrent verification commands
	ParallelChecks int `yaml:"parallel_checks"`
}

// DefaultActiveVerifyConfig returns sensible defaults
func DefaultActiveVerifyConfig() ActiveVerifyConfig {
	return ActiveVerifyConfig{
		Timeout:        30 * time.Second,
		RetryCount:     2,
		RetryDelay:     time.Second,
		ParallelChecks: 5,
	}
}

// VerificationResult holds the result of an active verification
type VerificationResult struct {
	CheckpointID string         `json:"checkpoint_id"`
	Passed       bool           `json:"passed"`
	Method       string         `json:"method"` // "file_check", "command", "service_status", etc.
	Output       string         `json:"output,omitempty"`
	Error        string         `json:"error,omitempty"`
	Duration     time.Duration  `json:"duration"`
	Timestamp    time.Time      `json:"timestamp"`
	Details      map[string]any `json:"details,omitempty"`
}

// NewActiveVerifier creates a new active verifier
func NewActiveVerifier(orch orchestrator.Client, logger *slog.Logger, config ActiveVerifyConfig) *ActiveVerifier {
	return &ActiveVerifier{
		orchestrator: orch,
		logger:       logger,
		config:       config,
	}
}

// VerifyCheckpoint actively verifies a single checkpoint condition
func (v *ActiveVerifier) VerifyCheckpoint(ctx context.Context, podID string, checkpoint *models.Checkpoint) (*VerificationResult, error) {
	start := time.Now()
	result := &VerificationResult{
		CheckpointID: checkpoint.ID,
		Timestamp:    start,
		Details:      make(map[string]any),
	}

	// Get the VM target from the checkpoint triggers
	var vmName string
	for _, trigger := range checkpoint.Triggers {
		if trigger.Target != "" {
			vmName = trigger.Target
			break
		}
	}

	if vmName == "" {
		result.Error = "no target VM specified in checkpoint"
		result.Duration = time.Since(start)
		return result, fmt.Errorf("no target VM in checkpoint %s", checkpoint.ID)
	}

	// Verify based on trigger types
	for _, trigger := range checkpoint.Triggers {
		var verified bool
		var err error

		switch trigger.Type {
		case models.TriggerTypeFileExists:
			verified, err = v.verifyFileExists(ctx, podID, vmName, trigger.Match.Path)
			result.Method = "file_exists"

		case models.TriggerTypeFileContent:
			verified, err = v.verifyFileContent(ctx, podID, vmName, trigger.Match.Path, trigger.Match.Contains, trigger.Match.Regex)
			result.Method = "file_content"

		case models.TriggerTypeFileDeleted:
			verified, err = v.verifyFileDeleted(ctx, podID, vmName, trigger.Match.Path)
			result.Method = "file_deleted"

		case models.TriggerTypeService:
			verified, err = v.verifyServiceState(ctx, podID, vmName, trigger.Match.Name, trigger.Match.State)
			result.Method = "service_status"

		case models.TriggerTypePackage:
			verified, err = v.verifyPackageInstalled(ctx, podID, vmName, trigger.Match.Package)
			result.Method = "package_installed"

		case models.TriggerTypeUserCreated:
			verified, err = v.verifyUserExists(ctx, podID, vmName, trigger.Match.User)
			result.Method = "user_exists"

		case models.TriggerTypePermission:
			verified, err = v.verifyFilePermissions(ctx, podID, vmName, trigger.Match.Path, trigger.Match.Mode, trigger.Match.Owner)
			result.Method = "file_permissions"

		default:
			// Skip triggers that can't be actively verified
			continue
		}

		if err != nil {
			result.Error = err.Error()
			v.logger.Warn("Active verification failed",
				"checkpoint_id", checkpoint.ID,
				"trigger_type", trigger.Type,
				"error", err,
			)
		}

		if verified {
			result.Passed = true
			break // At least one trigger verified
		}
	}

	result.Duration = time.Since(start)
	return result, nil
}

// VerifyAllCheckpoints verifies all checkpoints for a session
func (v *ActiveVerifier) VerifyAllCheckpoints(ctx context.Context, podID string, checkpoints []models.Checkpoint) ([]*VerificationResult, error) {
	results := make([]*VerificationResult, 0, len(checkpoints))
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Use semaphore to limit concurrent checks
	sem := make(chan struct{}, v.config.ParallelChecks)

	for i := range checkpoints {
		cp := &checkpoints[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}        // Acquire
			defer func() { <-sem }() // Release

			result, _ := v.VerifyCheckpoint(ctx, podID, cp)

			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}()
	}

	wg.Wait()
	return results, nil
}

// executeVMCommand executes a command on a VM and returns the output
func (v *ActiveVerifier) executeVMCommand(ctx context.Context, podID, vmName, command string) (string, error) {
	// Create timeout context
	ctx, cancel := context.WithTimeout(ctx, v.config.Timeout)
	defer cancel()

	// Execute via QEMU guest agent through orchestrator
	output, err := v.orchestrator.ExecuteCommand(ctx, podID, vmName, command)
	if err != nil {
		return "", fmt.Errorf("execute command on %s/%s: %w", podID, vmName, err)
	}

	return output, nil
}

// verifyFileExists checks if a file exists on the VM
func (v *ActiveVerifier) verifyFileExists(ctx context.Context, podID, vmName, path string) (bool, error) {
	// Sanitize path to prevent command injection
	path = sanitizePath(path)

	cmd := fmt.Sprintf("test -e %q && echo EXISTS || echo NOTFOUND", path)
	output, err := v.executeVMCommand(ctx, podID, vmName, cmd)
	if err != nil {
		return false, err
	}

	return strings.TrimSpace(output) == "EXISTS", nil
}

// verifyFileDeleted checks if a file does NOT exist on the VM
func (v *ActiveVerifier) verifyFileDeleted(ctx context.Context, podID, vmName, path string) (bool, error) {
	exists, err := v.verifyFileExists(ctx, podID, vmName, path)
	if err != nil {
		return false, err
	}
	return !exists, nil
}

// verifyFileContent checks if a file contains specific content
func (v *ActiveVerifier) verifyFileContent(ctx context.Context, podID, vmName, path, contains, regex string) (bool, error) {
	path = sanitizePath(path)

	// First check if file exists
	exists, err := v.verifyFileExists(ctx, podID, vmName, path)
	if err != nil || !exists {
		return false, err
	}

	// Read file content
	cmd := fmt.Sprintf("cat %q 2>/dev/null", path)
	output, err := v.executeVMCommand(ctx, podID, vmName, cmd)
	if err != nil {
		return false, err
	}

	// Check for contains match
	if contains != "" {
		if strings.Contains(output, contains) {
			return true, nil
		}
	}

	// Check for regex match
	if regex != "" {
		re, err := regexutil.CachedCompile(regex)
		if err != nil {
			return false, fmt.Errorf("invalid regex %q: %w", regex, err)
		}
		if re.MatchString(output) {
			return true, nil
		}
	}

	return false, nil
}

// verifyServiceState checks if a systemd service is in a specific state
func (v *ActiveVerifier) verifyServiceState(ctx context.Context, podID, vmName, serviceName, expectedState string) (bool, error) {
	serviceName = sanitizeServiceName(serviceName)

	cmd := fmt.Sprintf("systemctl is-active %q 2>/dev/null || true", serviceName)
	output, err := v.executeVMCommand(ctx, podID, vmName, cmd)
	if err != nil {
		return false, err
	}

	actualState := strings.TrimSpace(output)

	// Map expected states
	switch strings.ToLower(expectedState) {
	case "active", "running":
		return actualState == "active", nil
	case "inactive", "stopped":
		return actualState == "inactive", nil
	case "enabled":
		// Check if enabled
		cmd = fmt.Sprintf("systemctl is-enabled %q 2>/dev/null || true", serviceName)
		output, err = v.executeVMCommand(ctx, podID, vmName, cmd)
		if err != nil {
			return false, err
		}
		return strings.TrimSpace(output) == "enabled", nil
	default:
		return actualState == expectedState, nil
	}
}

// verifyPackageInstalled checks if a package is installed
func (v *ActiveVerifier) verifyPackageInstalled(ctx context.Context, podID, vmName, packageName string) (bool, error) {
	packageName = sanitizePackageName(packageName)

	// Try dpkg first (Debian/Ubuntu)
	cmd := fmt.Sprintf("dpkg -s %q 2>/dev/null | grep -q 'Status: install ok installed' && echo INSTALLED || echo NOTFOUND", packageName)
	output, err := v.executeVMCommand(ctx, podID, vmName, cmd)
	if err != nil {
		// Try rpm (RHEL/CentOS)
		cmd = fmt.Sprintf("rpm -q %q 2>/dev/null && echo INSTALLED || echo NOTFOUND", packageName)
		output, err = v.executeVMCommand(ctx, podID, vmName, cmd)
		if err != nil {
			return false, err
		}
	}

	return strings.Contains(output, "INSTALLED"), nil
}

// verifyUserExists checks if a user exists on the system
func (v *ActiveVerifier) verifyUserExists(ctx context.Context, podID, vmName, username string) (bool, error) {
	username = sanitizeUsername(username)

	cmd := fmt.Sprintf("id %q >/dev/null 2>&1 && echo EXISTS || echo NOTFOUND", username)
	output, err := v.executeVMCommand(ctx, podID, vmName, cmd)
	if err != nil {
		return false, err
	}

	return strings.TrimSpace(output) == "EXISTS", nil
}

// verifyFilePermissions checks file ownership and permissions
func (v *ActiveVerifier) verifyFilePermissions(ctx context.Context, podID, vmName, path, mode, owner string) (bool, error) {
	path = sanitizePath(path)

	// Get file stats
	cmd := fmt.Sprintf("stat -c '%%a %%U' %q 2>/dev/null", path)
	output, err := v.executeVMCommand(ctx, podID, vmName, cmd)
	if err != nil {
		return false, err
	}

	parts := strings.Fields(strings.TrimSpace(output))
	if len(parts) < 2 {
		return false, fmt.Errorf("unexpected stat output: %s", output)
	}

	actualMode := parts[0]
	actualOwner := parts[1]

	// Check mode if specified
	if mode != "" {
		// Normalize mode (remove leading zeros)
		expectedMode := strings.TrimLeft(mode, "0")
		actualMode = strings.TrimLeft(actualMode, "0")
		if actualMode != expectedMode {
			return false, nil
		}
	}

	// Check owner if specified
	if owner != "" && actualOwner != owner {
		return false, nil
	}

	return true, nil
}

// Sanitization helpers to prevent command injection

func sanitizePath(path string) string {
	return rePath.ReplaceAllString(path, "")
}

func sanitizeServiceName(name string) string {
	return reServiceName.ReplaceAllString(name, "")
}

func sanitizePackageName(name string) string {
	return rePackageName.ReplaceAllString(name, "")
}

func sanitizeUsername(name string) string {
	return reUsername.ReplaceAllString(name, "")
}

// CrossVerify compares passive (Wazuh event) results with active verification
// Returns true if both methods agree the checkpoint is passed
func (v *ActiveVerifier) CrossVerify(ctx context.Context, podID string, checkpoint *models.Checkpoint, passedByEvent bool) (*VerificationResult, bool, error) {
	// Perform active verification
	result, err := v.VerifyCheckpoint(ctx, podID, checkpoint)
	if err != nil {
		return result, false, err
	}

	// Log discrepancy if passive and active don't match
	if passedByEvent != result.Passed {
		v.logger.Warn("Checkpoint verification mismatch",
			"checkpoint_id", checkpoint.ID,
			"passed_by_event", passedByEvent,
			"passed_by_active", result.Passed,
			"pod_id", podID,
			"method", result.Method,
		)

		// In case of mismatch, trust active verification (harder to spoof)
		result.Details["passive_result"] = passedByEvent
		result.Details["verification_mismatch"] = true
	}

	return result, result.Passed, nil
}

// VerificationReport summarizes verification results
type VerificationReport struct {
	SessionID        string                `json:"session_id"`
	PodID            string                `json:"pod_id"`
	Timestamp        time.Time             `json:"timestamp"`
	TotalCheckpoints int                   `json:"total_checkpoints"`
	PassedCount      int                   `json:"passed_count"`
	FailedCount      int                   `json:"failed_count"`
	SkippedCount     int                   `json:"skipped_count"`
	Mismatches       int                   `json:"mismatches"`
	Results          []*VerificationResult `json:"results"`
	Duration         time.Duration         `json:"duration"`
}

// GenerateReport creates a verification report for a session
func (v *ActiveVerifier) GenerateReport(ctx context.Context, sessionID, podID string, checkpoints []models.Checkpoint, passedCheckpoints map[string]bool) (*VerificationReport, error) {
	start := time.Now()

	report := &VerificationReport{
		SessionID:        sessionID,
		PodID:            podID,
		Timestamp:        start,
		TotalCheckpoints: len(checkpoints),
		Results:          make([]*VerificationResult, 0, len(checkpoints)),
	}

	for i := range checkpoints {
		cp := &checkpoints[i]
		passedByEvent := passedCheckpoints[cp.ID]

		result, passed, err := v.CrossVerify(ctx, podID, cp, passedByEvent)
		switch {
		case err != nil:
			report.SkippedCount++
		case passed:
			report.PassedCount++
		default:
			report.FailedCount++
		}

		if passedByEvent != passed {
			report.Mismatches++
		}

		report.Results = append(report.Results, result)
	}

	report.Duration = time.Since(start)

	// Log summary
	v.logger.Info("Verification report generated",
		"session_id", sessionID,
		"total", report.TotalCheckpoints,
		"passed", report.PassedCount,
		"failed", report.FailedCount,
		"mismatches", report.Mismatches,
		"duration", report.Duration,
	)

	return report, nil
}

// ToJSON serializes a report to JSON
func (r *VerificationReport) ToJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}
