package checkpoint

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/regexutil"
)

// Validation errors
var (
	ErrInvalidIP     = errors.New("invalid IP address")
	ErrInvalidPath   = errors.New("invalid path: contains shell metacharacters")
	ErrInvalidTarget = errors.New("invalid target: contains shell metacharacters")
)

// validateIP ensures the given string is a valid IPv4 or IPv6 address.
// This prevents command injection through malicious vmIP values.
func validateIP(ip string) error {
	if ip == "" {
		return ErrInvalidIP
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ErrInvalidIP
	}
	return nil
}

// validatePath ensures the path doesn't contain shell metacharacters that could
// be used for command injection. Paths come from admin-defined lab templates,
// but this adds defense-in-depth.
func validatePath(path string) error {
	if path == "" {
		return ErrInvalidPath
	}
	// Disallow shell metacharacters that could be used for injection
	// These characters have special meaning in bash: ; | & $ ` ( ) { } < > \ " ' * ? [ ] ! # ~
	dangerousChars := []string{";", "|", "&", "$", "`", "(", ")", "{", "}", "<", ">", "\\", "\"", "'", "*", "?", "[", "]", "!", "#", "~", "\n", "\r"}
	for _, char := range dangerousChars {
		if strings.Contains(path, char) {
			return fmt.Errorf("%w: contains '%s'", ErrInvalidPath, char)
		}
	}
	return nil
}

// validateTarget ensures the target string doesn't contain shell metacharacters
// that could be used for command injection via ${TARGET} substitution in scripts.
// Targets come from admin-defined lab templates, but this adds defense-in-depth.
func validateTarget(target string) error {
	if target == "" {
		return nil // empty target is allowed (not all scripts use ${TARGET})
	}
	dangerousChars := []string{";", "|", "&", "$", "`", "(", ")", "{", "}", "<", ">", "\\", "\"", "'", "*", "?", "[", "]", "!", "#", "~", "\n", "\r"}
	for _, char := range dangerousChars {
		if strings.Contains(target, char) {
			return fmt.Errorf("%w: contains '%s'", ErrInvalidTarget, char)
		}
	}
	return nil
}

// ActiveChecker performs active verification of checkpoint conditions
type ActiveChecker struct {
	logger       *slog.Logger
	httpClient   *http.Client
	checkTimeout time.Duration
}

// NewActiveChecker creates a new active checker
func NewActiveChecker(logger *slog.Logger) *ActiveChecker {
	return &ActiveChecker{
		logger: logger,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		checkTimeout: 30 * time.Second,
	}
}

// CheckResult represents the result of an active check
type CheckResult struct {
	Passed   bool
	Message  string
	Details  map[string]any
	Duration time.Duration
}

// RunCheck executes an active check based on trigger type
func (ac *ActiveChecker) RunCheck(ctx context.Context, trigger models.CheckpointTrigger, vmIP string) (*CheckResult, error) {
	ctx, cancel := context.WithTimeout(ctx, ac.checkTimeout)
	defer cancel()

	start := time.Now()

	var result *CheckResult
	var err error

	switch trigger.Type {
	case models.TriggerTypeActiveCheck:
		result, err = ac.runScriptCheck(ctx, trigger, vmIP)
	case models.TriggerTypeNetwork:
		result, err = ac.runNetworkCheck(ctx, trigger, vmIP)
	case models.TriggerTypeService:
		result, err = ac.runServiceCheck(ctx, trigger, vmIP)
	case models.TriggerTypeFileExists:
		result, err = ac.runFileExistsCheck(ctx, trigger, vmIP)
	case models.TriggerTypeFileContent:
		result, err = ac.runFileContentCheck(ctx, trigger, vmIP)
	default:
		return nil, fmt.Errorf("unsupported active check type: %s", trigger.Type)
	}

	if err != nil {
		return nil, err
	}

	result.Duration = time.Since(start)
	return result, nil
}

// runScriptCheck executes a custom script check
func (ac *ActiveChecker) runScriptCheck(ctx context.Context, trigger models.CheckpointTrigger, vmIP string) (*CheckResult, error) {
	if trigger.Script == "" {
		return nil, fmt.Errorf("script is required for active_check trigger")
	}

	// Validate IP address to prevent injection
	if err := validateIP(vmIP); err != nil {
		return nil, fmt.Errorf("invalid vmIP: %w", err)
	}

	// Validate target to prevent injection via ${TARGET} substitution
	if err := validateTarget(trigger.Target); err != nil {
		return nil, fmt.Errorf("invalid target: %w", err)
	}

	// Replace placeholder variables in script
	script := trigger.Script
	script = strings.ReplaceAll(script, "${VM_IP}", vmIP)
	script = strings.ReplaceAll(script, "${TARGET}", trigger.Target)

	// Execute the script
	// #nosec G204 -- Scripts are defined in admin-managed lab templates (YAML files),
	// not from user input. Only trusted administrators can create/modify lab templates.
	cmd := exec.CommandContext(ctx, "bash", "-c", script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := &CheckResult{
		Passed:  err == nil,
		Message: stdout.String(),
		Details: map[string]any{
			"stdout":   stdout.String(),
			"stderr":   stderr.String(),
			"exitCode": cmd.ProcessState.ExitCode(),
		},
	}

	if err != nil {
		result.Message = fmt.Sprintf("Check failed: %s", stderr.String())
	}

	return result, nil
}

// runNetworkCheck verifies network connectivity/port availability
func (ac *ActiveChecker) runNetworkCheck(ctx context.Context, trigger models.CheckpointTrigger, vmIP string) (*CheckResult, error) {
	match := trigger.Match

	// Build target address
	target := vmIP
	if match.Destination != "" {
		target = match.Destination
	}

	// Validate target IP to prevent injection
	if err := validateIP(target); err != nil {
		return nil, fmt.Errorf("invalid target address: %w", err)
	}

	port := match.Port
	if port == 0 {
		return nil, fmt.Errorf("port is required for network check")
	}

	protocol := strings.ToLower(match.Protocol)
	if protocol == "" {
		protocol = "tcp"
	}

	addr := fmt.Sprintf("%s:%d", target, port)

	// Attempt connection
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, protocol, addr)

	result := &CheckResult{
		Details: map[string]any{
			"address":  addr,
			"protocol": protocol,
		},
	}

	if err != nil {
		result.Passed = false
		result.Message = fmt.Sprintf("Connection to %s failed: %v", addr, err)
	} else {
		conn.Close()
		result.Passed = true
		result.Message = fmt.Sprintf("Successfully connected to %s", addr)
	}

	return result, nil
}

// runServiceCheck verifies a service is responding
func (ac *ActiveChecker) runServiceCheck(ctx context.Context, trigger models.CheckpointTrigger, vmIP string) (*CheckResult, error) {
	match := trigger.Match

	// Validate vmIP to prevent injection
	if err := validateIP(vmIP); err != nil {
		return nil, fmt.Errorf("invalid vmIP: %w", err)
	}

	serviceName := match.Name
	if serviceName == "" {
		return nil, fmt.Errorf("service name is required")
	}

	expectedState := match.State
	if expectedState == "" {
		expectedState = "active"
	}

	// For common services, we can check their default ports
	servicePort := getServicePort(serviceName)
	if servicePort == 0 {
		return &CheckResult{
			Passed:  false,
			Message: fmt.Sprintf("Unknown service port for %s; use network check with explicit port", serviceName),
		}, nil
	}

	// Try to connect to the service
	addr := fmt.Sprintf("%s:%d", vmIP, servicePort)
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", addr)

	result := &CheckResult{
		Details: map[string]any{
			"service":       serviceName,
			"expectedState": expectedState,
			"port":          servicePort,
		},
	}

	if err != nil {
		result.Passed = expectedState == "inactive" || expectedState == "stopped"
		if !result.Passed {
			result.Message = fmt.Sprintf("Service %s is not responding on port %d", serviceName, servicePort)
		} else {
			result.Message = fmt.Sprintf("Service %s is correctly stopped", serviceName)
		}
	} else {
		conn.Close()
		result.Passed = expectedState == "active" || expectedState == "running"
		if result.Passed {
			result.Message = fmt.Sprintf("Service %s is running on port %d", serviceName, servicePort)
		} else {
			result.Message = fmt.Sprintf("Service %s is running but expected to be stopped", serviceName)
		}
	}

	return result, nil
}

// runFileExistsCheck checks if a file exists via SSH or API
func (ac *ActiveChecker) runFileExistsCheck(ctx context.Context, trigger models.CheckpointTrigger, vmIP string) (*CheckResult, error) {
	path := trigger.Match.Path
	if path == "" {
		return nil, fmt.Errorf("path is required for file_exists check")
	}

	// Validate vmIP and path to prevent command injection
	if err := validateIP(vmIP); err != nil {
		return nil, fmt.Errorf("invalid vmIP: %w", err)
	}
	if err := validatePath(path); err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	// Execute via SSH
	// vmIP is from internal pod allocation (now validated), path is from lab template (now validated).
	// Only trusted administrators can create/modify lab templates.
	script := fmt.Sprintf("ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 %s 'test -e %s'", vmIP, path)
	cmd := exec.CommandContext(ctx, "bash", "-c", script) // #nosec G204
	err := cmd.Run()

	result := &CheckResult{
		Details: map[string]any{
			"path": path,
			"vmIP": vmIP,
		},
	}

	if err == nil {
		result.Passed = true
		result.Message = fmt.Sprintf("File %s exists", path)
	} else {
		result.Passed = false
		result.Message = fmt.Sprintf("File %s does not exist", path)
	}

	return result, nil
}

// runFileContentCheck checks file contents via SSH
func (ac *ActiveChecker) runFileContentCheck(ctx context.Context, trigger models.CheckpointTrigger, vmIP string) (*CheckResult, error) {
	match := trigger.Match
	if match.Path == "" {
		return nil, fmt.Errorf("path is required for file_content check")
	}

	// Validate vmIP and path to prevent command injection
	if err := validateIP(vmIP); err != nil {
		return nil, fmt.Errorf("invalid vmIP: %w", err)
	}
	if err := validatePath(match.Path); err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	// Read file via SSH
	// vmIP is from internal pod allocation (now validated), path is from lab template (now validated).
	// Only trusted administrators can create/modify lab templates.
	script := fmt.Sprintf("ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 %s 'cat %s'", vmIP, match.Path)
	cmd := exec.CommandContext(ctx, "bash", "-c", script) // #nosec G204
	output, err := cmd.Output()

	result := &CheckResult{
		Details: map[string]any{
			"path": match.Path,
			"vmIP": vmIP,
		},
	}

	if err != nil {
		result.Passed = false
		result.Message = fmt.Sprintf("Failed to read file %s: %v", match.Path, err)
		return result, nil
	}

	content := string(output)
	result.Details["contentLength"] = len(content)

	// Check for required content
	if match.Contains != "" {
		if strings.Contains(content, match.Contains) {
			result.Passed = true
			result.Message = fmt.Sprintf("File contains expected text: %s", match.Contains)
		} else {
			result.Passed = false
			result.Message = fmt.Sprintf("File does not contain expected text: %s", match.Contains)
		}
		return result, nil
	}

	// Check with regex
	if match.Regex != "" {
		re, err := regexutil.CachedCompile(match.Regex)
		if err != nil {
			return nil, fmt.Errorf("invalid regex: %w", err)
		}

		if re.MatchString(content) {
			result.Passed = true
			result.Message = fmt.Sprintf("File matches pattern: %s", match.Regex)
		} else {
			result.Passed = false
			result.Message = fmt.Sprintf("File does not match pattern: %s", match.Regex)
		}
		return result, nil
	}

	// Just check file exists and is readable
	result.Passed = true
	result.Message = fmt.Sprintf("File %s exists and is readable", match.Path)

	return result, nil
}

// getServicePort returns the default port for common services
func getServicePort(service string) int {
	ports := map[string]int{
		"ssh":        22,
		"sshd":       22,
		"http":       80,
		"httpd":      80,
		"apache2":    80,
		"nginx":      80,
		"https":      443,
		"mysql":      3306,
		"mariadb":    3306,
		"postgres":   5432,
		"postgresql": 5432,
		"redis":      6379,
		"mongodb":    27017,
		"dns":        53,
		"bind":       53,
		"named":      53,
		"smtp":       25,
		"postfix":    25,
		"ftp":        21,
		"vsftpd":     21,
		"telnet":     23,
		"ldap":       389,
		"ldaps":      636,
		"ntp":        123,
		"snmp":       161,
		"docker":     2375,
	}

	return ports[strings.ToLower(service)]
}

// -----------------------------------------------------------------------------
// Scheduled Active Check Runner
// -----------------------------------------------------------------------------

// VMIPLookup provides VM IP address lookup
type VMIPLookup interface {
	GetVMIP(ctx context.Context, vmPlatformID string) (string, error)
}

// ActiveCheckRunner periodically runs active checks for sessions
type ActiveCheckRunner struct {
	checker    *ActiveChecker
	evaluator  *Evaluator
	vmIPLookup VMIPLookup
	logger     *slog.Logger
	interval   time.Duration
	sessions   map[string]*activeCheckSession
	mu         sync.RWMutex
	stopCh     chan struct{}
}

// activeCheckSession tracks active check state for a session
type activeCheckSession struct {
	sessionID     string
	podID         string
	checkpoints   []models.Checkpoint
	vmIPs         map[string]string // VM name -> IP mapping
	vmPlatformIDs map[string]string // VM name -> platform ID mapping
	lastCheck     time.Time
}

// NewActiveCheckRunner creates a new active check runner
func NewActiveCheckRunner(evaluator *Evaluator, logger *slog.Logger, opts ...ActiveCheckRunnerOption) *ActiveCheckRunner {
	r := &ActiveCheckRunner{
		checker:   NewActiveChecker(logger),
		evaluator: evaluator,
		logger:    logger,
		interval:  30 * time.Second,
		sessions:  make(map[string]*activeCheckSession),
		stopCh:    make(chan struct{}),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// ActiveCheckRunnerOption configures an ActiveCheckRunner
type ActiveCheckRunnerOption func(*ActiveCheckRunner)

// WithVMIPLookup sets the VM IP lookup interface
func WithVMIPLookup(lookup VMIPLookup) ActiveCheckRunnerOption {
	return func(r *ActiveCheckRunner) {
		r.vmIPLookup = lookup
	}
}

// Start starts the active check runner
func (r *ActiveCheckRunner) Start(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	r.logger.Info("Active check runner started", "interval", r.interval)

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("Active check runner stopping")
			return
		case <-r.stopCh:
			r.logger.Info("Active check runner stopped")
			return
		case <-ticker.C:
			r.runChecks(ctx)
		}
	}
}

// Stop stops the active check runner
func (r *ActiveCheckRunner) Stop() {
	close(r.stopCh)
}

// RegisterSession registers a session for active checks
func (r *ActiveCheckRunner) RegisterSession(sessionID, podID string, checkpoints []models.Checkpoint, vmIPs, vmPlatformIDs map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Filter to only checkpoints with triggers that can be actively checked
	var activeCheckpoints []models.Checkpoint
	for _, cp := range checkpoints {
		for _, trigger := range cp.Triggers {
			if trigger.Type == models.TriggerTypeActiveCheck ||
				trigger.Type == models.TriggerTypeNetwork ||
				trigger.Type == models.TriggerTypeFileExists ||
				trigger.Type == models.TriggerTypeFileContent ||
				(trigger.Type == models.TriggerTypeService && trigger.Match.State != "") {
				activeCheckpoints = append(activeCheckpoints, cp)
				break
			}
		}
	}

	if len(activeCheckpoints) == 0 {
		return
	}

	r.sessions[sessionID] = &activeCheckSession{
		sessionID:     sessionID,
		podID:         podID,
		checkpoints:   activeCheckpoints,
		vmIPs:         vmIPs,
		vmPlatformIDs: vmPlatformIDs,
	}

	r.logger.Info("Registered session for active checks",
		"sessionId", sessionID,
		"checkpoints", len(activeCheckpoints),
		"vmIPs", vmIPs,
		"vmPlatformIDs", vmPlatformIDs,
		"hasVMIPLookup", r.vmIPLookup != nil,
	)
}

// UnregisterSession removes a session from active checks
func (r *ActiveCheckRunner) UnregisterSession(sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.sessions, sessionID)
	r.logger.Debug("Unregistered session from active checks", "sessionId", sessionID)
}

// FindSessionByVMIP finds a session by matching a VM's IP address.
// Returns the session ID and pod ID if found, or empty strings if not found.
func (r *ActiveCheckRunner) FindSessionByVMIP(ip string) (sessionID, podID string) {
	if ip == "" {
		return "", ""
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, session := range r.sessions {
		for _, vmIP := range session.vmIPs {
			if vmIP == ip {
				return session.sessionID, session.podID
			}
		}
	}

	return "", ""
}

// runChecks runs all pending active checks
func (r *ActiveCheckRunner) runChecks(ctx context.Context) {
	r.mu.RLock()
	sessions := make([]*activeCheckSession, 0, len(r.sessions))
	for _, s := range r.sessions {
		sessions = append(sessions, s)
	}
	r.mu.RUnlock()

	for _, session := range sessions {
		r.checkSession(ctx, session)
	}
}

// checkSession runs active checks for a single session
func (r *ActiveCheckRunner) checkSession(ctx context.Context, session *activeCheckSession) {
	for _, checkpoint := range session.checkpoints {
		// Check if already passed
		progress, err := r.evaluator.GetSessionProgress(session.sessionID)
		if err != nil {
			continue
		}

		alreadyPassed := false
		for _, cp := range progress.Checkpoints {
			if cp.CheckpointID == checkpoint.ID && cp.Status == models.CheckpointStatusPassed {
				alreadyPassed = true
				break
			}
		}

		if alreadyPassed {
			continue
		}

		// Run triggers
		allPassed := true
		for _, trigger := range checkpoint.Triggers {
			// Get VM IP - try cached first, then lookup dynamically
			vmIP := session.vmIPs[trigger.Target]
			if vmIP == "" && r.vmIPLookup != nil {
				platformID := session.vmPlatformIDs[trigger.Target]
				r.logger.Debug("Looking up VM IP", "vm", trigger.Target, "platformId", platformID)
				if platformID != "" {
					ip, err := r.vmIPLookup.GetVMIP(ctx, platformID)
					if err != nil {
						r.logger.Debug("VM IP lookup failed", "vm", trigger.Target, "platformId", platformID, "error", err)
					} else if ip != "" {
						vmIP = ip
						// Cache for future checks
						if session.vmIPs == nil {
							session.vmIPs = make(map[string]string)
						}
						session.vmIPs[trigger.Target] = ip
						r.logger.Debug("Resolved VM IP", "vm", trigger.Target, "platformId", platformID, "ip", ip)
					}
				}
			}
			if vmIP == "" {
				r.logger.Warn("No IP for VM", "vm", trigger.Target, "sessionId", session.sessionID)
				allPassed = false
				continue
			}

			// Run check
			result, err := r.checker.RunCheck(ctx, trigger, vmIP)
			if err != nil {
				r.logger.Error("Active check error",
					"checkpointId", checkpoint.ID,
					"trigger", trigger.Type,
					"error", err,
				)
				allPassed = false
				continue
			}

			if !result.Passed {
				allPassed = false
			}

			r.logger.Debug("Active check completed",
				"checkpointId", checkpoint.ID,
				"trigger", trigger.Type,
				"passed", result.Passed,
				"duration", result.Duration,
			)
		}

		// Mark checkpoint if all triggers passed
		if allPassed {
			r.logger.Info("Checkpoint passed via active check",
				"sessionId", session.sessionID,
				"checkpointId", checkpoint.ID,
			)
			// Update the evaluator state
			if err := r.evaluator.MarkCheckpointPassed(ctx, session.sessionID, checkpoint.ID); err != nil {
				r.logger.Error("Failed to mark checkpoint as passed",
					"sessionId", session.sessionID,
					"checkpointId", checkpoint.ID,
					"error", err,
				)
			}
		}
	}

	session.lastCheck = time.Now()
}
