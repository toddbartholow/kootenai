// Package assessment provides Packet Tracer-style assessment verification
package assessment

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/regexutil"
)

// Pre-compiled regexes for output parsing (avoid per-call compilation).
var (
	reDefaultGateway = regexp.MustCompile(`(?:via|Gateway of last resort is)\s+(\d+\.\d+\.\d+\.\d+)`)
	reCiscoRoute     = regexp.MustCompile(`([CDOSRBEI*])\s+(\d+\.\d+\.\d+\.\d+(?:/\d+)?)\s+(?:is directly connected,|via\s+(\d+\.\d+\.\d+\.\d+),?)?\s*(\S+)?`)
	reLinuxInterface = regexp.MustCompile(`\d+:\s+(\S+)\s+inet\s+(\d+\.\d+\.\d+\.\d+)/(\d+)`)
	reLinuxDefaultGW = regexp.MustCompile(`default via (\d+\.\d+\.\d+\.\d+)`)
)

// ErrCommandNotAllowed is returned when a command is not in the allowlist
var ErrCommandNotAllowed = errors.New("command not in allowlist")

// allowedCommands is a whitelist of safe commands that can be executed
// Commands with parameters use %s placeholders that must be sanitized
var allowedCommands = map[string]bool{
	// Cisco IOS commands
	"show ip interface brief": true,
	"show running-config":     true,
	"show ip route":           true,
	"show ip route 0.0.0.0":   true,
	// Linux commands
	"ip -o addr show":       true,
	"ip route show default": true,
	"ip route show":         true,
}

// allowedCommandPrefixes for commands that take sanitized parameters
var allowedCommandPrefixes = []string{
	"show interface ",
	"systemctl is-active ",
	"cat ",
	"ping -c 1 -W 2 ",
}

// dangerousChars that could be used for command injection
var dangerousChars = regexp.MustCompile(`[;&|$` + "`" + `\\<>(){}[\]!#*?~\n\r]`)

// Verifier performs assessment checks against devices
type Verifier struct {
	logger     *slog.Logger
	sshTimeout time.Duration
	sshUser    string
	sshKeyPath string
}

// NewVerifier creates a new assessment verifier
func NewVerifier(logger *slog.Logger) *Verifier {
	return &Verifier{
		logger:     logger,
		sshTimeout: 10 * time.Second,
		sshUser:    "root",
		sshKeyPath: "",
	}
}

// WithSSHConfig sets SSH configuration
func (v *Verifier) WithSSHConfig(user, keyPath string, timeout time.Duration) *Verifier {
	v.sshUser = user
	v.sshKeyPath = keyPath
	v.sshTimeout = timeout
	return v
}

// VerifyDevice runs all checks for a device and returns results
func (v *Verifier) VerifyDevice(ctx context.Context, device models.DeviceAssessment, deviceIP string) (*models.DeviceResult, error) {
	result := &models.DeviceResult{
		Name:       device.Name,
		Type:       device.Type,
		Status:     models.AssessmentStatusPending,
		Checks:     make([]models.CheckResult, 0),
		Interfaces: make([]models.InterfaceResult, 0),
	}

	// Get device configuration once
	config, err := v.getDeviceConfig(ctx, device.Type, deviceIP)
	if err != nil {
		v.logger.Warn("Failed to get device config", "device", device.Name, "error", err)
		// Continue with individual checks - some may still work
	}

	// Run device-level checks
	for _, check := range device.Checks {
		checkResult := v.runCheck(ctx, check, deviceIP, "", config)
		result.Checks = append(result.Checks, checkResult)
		result.MaxPoints += check.Points
		result.TotalItems++
		if checkResult.Status == models.AssessmentStatusCorrect {
			result.EarnedPoints += check.Points
			result.PassedItems++
		}
	}

	// Run interface checks
	for _, iface := range device.Interfaces {
		ifaceResult := v.verifyInterface(ctx, iface, deviceIP, config)
		result.Interfaces = append(result.Interfaces, ifaceResult)
		result.MaxPoints += ifaceResult.MaxPoints
		result.TotalItems += ifaceResult.TotalItems
		result.EarnedPoints += ifaceResult.EarnedPoints
		result.PassedItems += ifaceResult.PassedItems
	}

	// Calculate aggregate status
	switch {
	case result.PassedItems == result.TotalItems:
		result.Status = models.AssessmentStatusCorrect
	case result.PassedItems > 0:
		result.Status = models.AssessmentStatusIncomplete
	default:
		result.Status = models.AssessmentStatusIncorrect
	}

	return result, nil
}

// verifyInterface runs all checks for an interface
func (v *Verifier) verifyInterface(ctx context.Context, iface models.InterfaceAssessment, deviceIP string, config *models.DeviceConfig) models.InterfaceResult {
	result := models.InterfaceResult{
		Name:   iface.Name,
		Status: models.AssessmentStatusPending,
		Checks: make([]models.CheckResult, 0),
	}

	for _, check := range iface.Checks {
		checkResult := v.runCheck(ctx, check, deviceIP, iface.Name, config)
		result.Checks = append(result.Checks, checkResult)
		result.MaxPoints += check.Points
		result.TotalItems++
		if checkResult.Status == models.AssessmentStatusCorrect {
			result.EarnedPoints += check.Points
			result.PassedItems++
		}
	}

	// Calculate aggregate status
	switch {
	case result.PassedItems == result.TotalItems:
		result.Status = models.AssessmentStatusCorrect
	case result.PassedItems > 0:
		result.Status = models.AssessmentStatusIncomplete
	default:
		result.Status = models.AssessmentStatusIncorrect
	}

	return result
}

// runCheck executes a single assessment check
func (v *Verifier) runCheck(ctx context.Context, check models.AssessmentCheck, deviceIP, interfaceName string, config *models.DeviceConfig) models.CheckResult {
	now := time.Now()
	result := models.CheckResult{
		ID:          check.ID,
		Description: check.Description,
		Component:   check.Component,
		Points:      check.Points,
		Expected:    check.Verify.Expected,
		CheckedAt:   &now,
	}

	var actual string
	var err error

	switch check.Verify.Type {
	case models.VerifyTypeConfigValue:
		actual, err = v.verifyConfigValue(ctx, deviceIP, check.Verify, config)

	case models.VerifyTypeInterfaceStatus:
		actual, err = v.verifyInterfaceStatus(ctx, deviceIP, interfaceName, config)

	case models.VerifyTypeInterfaceIP:
		actual, err = v.verifyInterfaceIP(ctx, deviceIP, interfaceName, config)

	case models.VerifyTypeSubnetMask:
		actual, err = v.verifySubnetMask(ctx, deviceIP, interfaceName, config)

	case models.VerifyTypeDefaultGateway:
		actual, err = v.verifyDefaultGateway(ctx, deviceIP, config)

	case models.VerifyTypeRouteExists:
		actual, err = v.verifyRouteExists(ctx, deviceIP, check.Verify, config)

	case models.VerifyTypeServiceRunning:
		actual, err = v.verifyServiceRunning(ctx, deviceIP, check.Verify)

	case models.VerifyTypeFileContent:
		actual, err = v.verifyFileContent(ctx, deviceIP, check.Verify)

	case models.VerifyTypeConnectivity:
		actual, err = v.verifyConnectivity(ctx, deviceIP, check.Verify)

	case models.VerifyTypeCommand:
		actual, err = v.verifyCommand(ctx, deviceIP, check.Verify)

	default:
		err = fmt.Errorf("unknown verify type: %s", check.Verify.Type)
	}

	result.Actual = actual

	if err != nil {
		result.Status = models.AssessmentStatusError
		result.Error = err.Error()
		result.Feedback = fmt.Sprintf("Verification failed: %v", err)
		return result
	}

	// Compare actual vs expected
	if v.compareValues(actual, check.Verify.Expected, check.Verify.Operator, check.Verify.Regex) {
		result.Status = models.AssessmentStatusCorrect
		result.EarnedPoints = check.Points
		result.Feedback = check.Description + "..."
	} else {
		result.Status = models.AssessmentStatusIncorrect
		result.Feedback = fmt.Sprintf("Expected: %s, Got: %s", check.Verify.Expected, actual)
	}

	return result
}

// compareValues compares actual vs expected using the specified operator
func (v *Verifier) compareValues(actual, expected, operator, regexPattern string) bool {
	actual = strings.TrimSpace(actual)
	expected = strings.TrimSpace(expected)

	switch operator {
	case "ne":
		return actual != expected
	case "contains":
		return strings.Contains(actual, expected)
	case "matches":
		if regexPattern != "" {
			re, err := regexutil.CachedCompile(regexPattern)
			if err != nil {
				return false
			}
			return re.MatchString(actual)
		}
		return false
	case "gt":
		a, _ := strconv.ParseFloat(actual, 64)
		e, _ := strconv.ParseFloat(expected, 64)
		return a > e
	case "lt":
		a, _ := strconv.ParseFloat(actual, 64)
		e, _ := strconv.ParseFloat(expected, 64)
		return a < e
	case "eq", "":
		// Case-insensitive comparison for IP addresses and status
		return strings.EqualFold(actual, expected)
	default:
		return actual == expected
	}
}

// -----------------------------------------------------------------------------
// Device Configuration Retrieval
// -----------------------------------------------------------------------------

// getDeviceConfig retrieves and parses device configuration
func (v *Verifier) getDeviceConfig(ctx context.Context, deviceType, deviceIP string) (*models.DeviceConfig, error) {
	switch deviceType {
	case "router", "switch":
		return v.getCiscoConfig(ctx, deviceIP)
	case "host", "linux":
		return v.getLinuxConfig(ctx, deviceIP)
	default:
		return nil, fmt.Errorf("unsupported device type: %s", deviceType)
	}
}

// getCiscoConfig retrieves configuration from Cisco IOS device
func (v *Verifier) getCiscoConfig(ctx context.Context, deviceIP string) (*models.DeviceConfig, error) {
	config := &models.DeviceConfig{
		Interfaces: make(map[string]models.InterfaceConfig),
		Routes:     make([]models.Route, 0),
		VLANs:      make(map[int]string),
	}

	// Get interface brief
	output, err := v.runSSHCommand(ctx, deviceIP, "show ip interface brief")
	if err != nil {
		return nil, fmt.Errorf("failed to get interface brief: %w", err)
	}
	v.parseCiscoInterfaceBrief(output, config)

	// Get running config for more details
	output, err = v.runSSHCommand(ctx, deviceIP, "show running-config")
	if err == nil {
		v.parseCiscoRunningConfig(output, config)
	}

	// Get routing table
	output, err = v.runSSHCommand(ctx, deviceIP, "show ip route")
	if err == nil {
		v.parseCiscoRoutes(output, config)
	}

	return config, nil
}

// getLinuxConfig retrieves configuration from Linux host
func (v *Verifier) getLinuxConfig(ctx context.Context, deviceIP string) (*models.DeviceConfig, error) {
	config := &models.DeviceConfig{
		Interfaces: make(map[string]models.InterfaceConfig),
		Routes:     make([]models.Route, 0),
		Services:   make(map[string]bool),
	}

	// Get interface info
	output, err := v.runSSHCommand(ctx, deviceIP, "ip -o addr show")
	if err != nil {
		return nil, fmt.Errorf("failed to get interfaces: %w", err)
	}
	v.parseLinuxInterfaces(output, config)

	// Get default gateway
	output, err = v.runSSHCommand(ctx, deviceIP, "ip route show default")
	if err == nil {
		v.parseLinuxDefaultGateway(output, config)
	}

	// Get routing table
	output, err = v.runSSHCommand(ctx, deviceIP, "ip route show")
	if err == nil {
		v.parseLinuxRoutes(output, config)
	}

	return config, nil
}

// -----------------------------------------------------------------------------
// Verification Methods
// -----------------------------------------------------------------------------

func (v *Verifier) verifyConfigValue(ctx context.Context, deviceIP string, verify models.VerifyConfig, config *models.DeviceConfig) (string, error) {
	if verify.Command != "" {
		return v.runSSHCommand(ctx, deviceIP, verify.Command)
	}
	return "", fmt.Errorf("no command specified for config_value check")
}

func (v *Verifier) verifyInterfaceStatus(ctx context.Context, deviceIP, interfaceName string, config *models.DeviceConfig) (string, error) {
	if config != nil {
		if iface, ok := config.Interfaces[interfaceName]; ok {
			return fmt.Sprintf("%s/%s", iface.Status, iface.Protocol), nil
		}
	}

	// Fallback to direct query
	cmd := fmt.Sprintf("show interface %s | include line protocol", interfaceName)
	output, err := v.runSSHCommand(ctx, deviceIP, cmd)
	if err != nil {
		return "", err
	}

	// Parse "GigabitEthernet0/0 is up, line protocol is up"
	if strings.Contains(output, "is up") && strings.Contains(output, "line protocol is up") {
		return "up/up", nil
	}
	return "down/down", nil
}

func (v *Verifier) verifyInterfaceIP(ctx context.Context, deviceIP, interfaceName string, config *models.DeviceConfig) (string, error) {
	if config != nil {
		if iface, ok := config.Interfaces[interfaceName]; ok {
			return iface.IPAddress, nil
		}
	}
	return "", fmt.Errorf("interface %s not found", interfaceName)
}

func (v *Verifier) verifySubnetMask(ctx context.Context, deviceIP, interfaceName string, config *models.DeviceConfig) (string, error) {
	if config != nil {
		if iface, ok := config.Interfaces[interfaceName]; ok {
			return iface.SubnetMask, nil
		}
	}
	return "", fmt.Errorf("interface %s not found", interfaceName)
}

func (v *Verifier) verifyDefaultGateway(ctx context.Context, deviceIP string, config *models.DeviceConfig) (string, error) {
	if config != nil && config.DefaultGateway != "" {
		return config.DefaultGateway, nil
	}

	// Try to get from routing table
	output, err := v.runSSHCommand(ctx, deviceIP, "show ip route 0.0.0.0")
	if err != nil {
		// Try Linux style
		output, err = v.runSSHCommand(ctx, deviceIP, "ip route show default")
	}

	if err != nil {
		return "", err
	}

	// Parse default route
	matches := reDefaultGateway.FindStringSubmatch(output)
	if len(matches) > 1 {
		return matches[1], nil
	}

	return "", fmt.Errorf("default gateway not found")
}

func (v *Verifier) verifyRouteExists(ctx context.Context, deviceIP string, verify models.VerifyConfig, config *models.DeviceConfig) (string, error) {
	if config != nil {
		for _, route := range config.Routes {
			if route.Network == verify.Expected ||
				(route.Network+"/"+route.Mask) == verify.Expected {
				return "exists", nil
			}
		}
	}
	return "not found", nil
}

func (v *Verifier) verifyServiceRunning(ctx context.Context, deviceIP string, verify models.VerifyConfig) (string, error) {
	cmd := fmt.Sprintf("systemctl is-active %s", verify.Expected)
	output, err := v.runSSHCommand(ctx, deviceIP, cmd)
	if err != nil {
		return "inactive", nil
	}
	return strings.TrimSpace(output), nil
}

func (v *Verifier) verifyFileContent(ctx context.Context, deviceIP string, verify models.VerifyConfig) (string, error) {
	if verify.Path == "" {
		return "", fmt.Errorf("no path specified for file_content check")
	}

	cmd := fmt.Sprintf("cat %s", verify.Path)
	output, err := v.runSSHCommand(ctx, deviceIP, cmd)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// If checking for contains, return whether it contains the expected value
	if verify.Operator == "contains" {
		if strings.Contains(output, verify.Expected) {
			return verify.Expected, nil
		}
		return "", nil
	}

	return strings.TrimSpace(output), nil
}

func (v *Verifier) verifyConnectivity(ctx context.Context, deviceIP string, verify models.VerifyConfig) (string, error) {
	target := verify.Expected
	if target == "" {
		return "", fmt.Errorf("no target specified for connectivity check")
	}

	// Try TCP connection first
	conn, err := net.DialTimeout("tcp", target, 5*time.Second)
	if err == nil {
		conn.Close()
		return "reachable", nil
	}

	// Try ping via SSH
	cmd := fmt.Sprintf("ping -c 1 -W 2 %s", strings.Split(target, ":")[0])
	output, err := v.runSSHCommand(ctx, deviceIP, cmd)
	if err == nil && strings.Contains(output, "1 received") {
		return "reachable", nil
	}

	return "unreachable", nil
}

func (v *Verifier) verifyCommand(ctx context.Context, deviceIP string, verify models.VerifyConfig) (string, error) {
	if verify.Command == "" {
		return "", fmt.Errorf("no command specified")
	}
	return v.runSSHCommand(ctx, deviceIP, verify.Command)
}

// -----------------------------------------------------------------------------
// SSH Command Execution
// -----------------------------------------------------------------------------

// isCommandAllowed checks if a command is in the allowlist
func (v *Verifier) isCommandAllowed(command string) bool {
	// Check exact match first
	if allowedCommands[command] {
		return true
	}

	// Check prefixes for parameterized commands
	for _, prefix := range allowedCommandPrefixes {
		if strings.HasPrefix(command, prefix) {
			// Extract the parameter
			param := strings.TrimPrefix(command, prefix)

			// For "show interface X | include Y" pattern, allow safe pipe usage
			// but reject dangerous pipe usage like "| bash"
			if strings.Contains(param, "|") {
				parts := strings.SplitN(param, " |", 2)
				param = parts[0]
				if len(parts) > 1 {
					suffix := strings.TrimSpace(parts[1])
					// Only allow safe suffixes like "include X" or "grep X"
					if !isAllowedPipeSuffix(suffix) {
						v.logger.Warn("Command has disallowed pipe suffix",
							"command", command,
							"suffix", suffix,
						)
						return false
					}
				}
			}

			// Check param for dangerous characters
			if dangerousChars.MatchString(param) {
				v.logger.Warn("Command parameter contains dangerous characters",
					"command", command,
					"param", param,
				)
				return false
			}
			return true
		}
	}

	return false
}

// isAllowedPipeSuffix checks if a pipe suffix is safe
func isAllowedPipeSuffix(suffix string) bool {
	// Allowed pipe suffixes for Cisco IOS commands
	allowedSuffixes := []string{
		"include ",
		"exclude ",
		"begin ",
		"section ",
	}

	suffixLower := strings.ToLower(suffix)
	for _, allowed := range allowedSuffixes {
		if strings.HasPrefix(suffixLower, allowed) {
			// Check the argument after the suffix keyword
			arg := strings.TrimPrefix(suffixLower, allowed)
			// Only allow simple text patterns, no dangerous chars
			if !dangerousChars.MatchString(arg) {
				return true
			}
		}
	}
	return false
}

// sanitizeCommandParam removes or escapes dangerous characters from a parameter
func sanitizeCommandParam(param string) string {
	// Remove any dangerous characters
	return dangerousChars.ReplaceAllString(param, "")
}

// validateIPAddress checks if a string is a valid IP address
func validateIPAddress(ip string) bool {
	return net.ParseIP(ip) != nil
}

func (v *Verifier) runSSHCommand(ctx context.Context, deviceIP, command string) (string, error) {
	// Validate device IP to prevent injection via IP field
	if !validateIPAddress(deviceIP) {
		return "", fmt.Errorf("invalid device IP address: %s", deviceIP)
	}

	// Validate command against allowlist
	if !v.isCommandAllowed(command) {
		v.logger.Warn("Command not in allowlist",
			"command", command,
			"deviceIP", deviceIP,
		)
		return "", fmt.Errorf("%w: %s", ErrCommandNotAllowed, command)
	}

	args := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", fmt.Sprintf("ConnectTimeout=%d", int(v.sshTimeout.Seconds())),
		"-o", "BatchMode=yes", // Prevent interactive prompts
	}

	if v.sshKeyPath != "" {
		args = append(args, "-i", v.sshKeyPath)
	}

	args = append(args, fmt.Sprintf("%s@%s", v.sshUser, deviceIP), command)

	ctx, cancel := context.WithTimeout(ctx, v.sshTimeout+2*time.Second)
	defer cancel()

	// #nosec G204 -- SSH arguments are constructed safely without shell interpolation.
	// deviceIP is from internal pod allocation, command is from verifier configuration.
	cmd := exec.CommandContext(ctx, "ssh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("SSH command failed (stderr: %s): %w", stderr.String(), err)
	}

	return stdout.String(), nil
}

// -----------------------------------------------------------------------------
// Output Parsers
// -----------------------------------------------------------------------------

func (v *Verifier) parseCiscoInterfaceBrief(output string, config *models.DeviceConfig) {
	// Parse "show ip interface brief" output
	// Interface              IP-Address      OK? Method Status                Protocol
	// GigabitEthernet0/0     10.0.0.1        YES NVRAM  up                    up
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 6 && net.ParseIP(fields[1]) != nil {
			config.Interfaces[fields[0]] = models.InterfaceConfig{
				Name:      fields[0],
				IPAddress: fields[1],
				Status:    fields[4],
				Protocol:  fields[5],
			}
		}
	}
}

func (v *Verifier) parseCiscoRunningConfig(output string, config *models.DeviceConfig) {
	lines := strings.Split(output, "\n")
	var currentInterface string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Hostname
		if strings.HasPrefix(line, "hostname ") {
			config.Hostname = strings.TrimPrefix(line, "hostname ")
		}

		// Default gateway (for switches)
		if strings.HasPrefix(line, "ip default-gateway ") {
			config.DefaultGateway = strings.TrimPrefix(line, "ip default-gateway ")
		}

		// Interface configuration
		if strings.HasPrefix(line, "interface ") {
			currentInterface = strings.TrimPrefix(line, "interface ")
		}

		// IP address within interface
		if currentInterface != "" && strings.HasPrefix(line, "ip address ") {
			parts := strings.Fields(strings.TrimPrefix(line, "ip address "))
			if len(parts) >= 2 {
				iface := config.Interfaces[currentInterface]
				iface.Name = currentInterface
				iface.IPAddress = parts[0]
				iface.SubnetMask = parts[1]
				config.Interfaces[currentInterface] = iface
			}
		}

		// Exit interface config
		if line == "!" && currentInterface != "" {
			currentInterface = ""
		}
	}
}

func (v *Verifier) parseCiscoRoutes(output string, config *models.DeviceConfig) {
	// Parse "show ip route" output
	for _, match := range reCiscoRoute.FindAllStringSubmatch(output, -1) {
		if len(match) < 3 {
			continue
		}
		route := models.Route{
			Network: match[2],
		}
		if len(match) > 3 {
			route.NextHop = match[3]
		}
		if len(match) > 4 {
			route.Interface = match[4]
		}

		switch match[1] {
		case "C":
			route.Protocol = "connected"
		case "S":
			route.Protocol = "static"
		case "O":
			route.Protocol = "ospf"
		case "D":
			route.Protocol = "eigrp"
		case "B":
			route.Protocol = "bgp"
		case "R":
			route.Protocol = "rip"
		}

		if strings.HasPrefix(match[2], "0.0.0.0") {
			route.IsDefault = true
			config.DefaultGateway = route.NextHop
		}

		config.Routes = append(config.Routes, route)
	}
}

func (v *Verifier) parseLinuxInterfaces(output string, config *models.DeviceConfig) {
	// Parse "ip -o addr show" output
	// 2: eth0    inet 10.0.0.10/24 brd 10.0.0.255 scope global eth0
	for _, match := range reLinuxInterface.FindAllStringSubmatch(output, -1) {
		if len(match) >= 4 {
			cidr, _ := strconv.Atoi(match[3])
			config.Interfaces[match[1]] = models.InterfaceConfig{
				Name:       match[1],
				IPAddress:  match[2],
				CIDR:       cidr,
				SubnetMask: cidrToMask(cidr),
				Status:     "up",
				Protocol:   "up",
			}
		}
	}
}

func (v *Verifier) parseLinuxDefaultGateway(output string, config *models.DeviceConfig) {
	// Parse "ip route show default"
	// default via 10.0.0.1 dev eth0
	matches := reLinuxDefaultGW.FindStringSubmatch(output)
	if len(matches) > 1 {
		config.DefaultGateway = matches[1]
	}
}

func (v *Verifier) parseLinuxRoutes(output string, config *models.DeviceConfig) {
	// Parse "ip route show"
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 3 {
			route := models.Route{
				Network: fields[0],
			}

			for i, f := range fields {
				if f == "via" && i+1 < len(fields) {
					route.NextHop = fields[i+1]
				}
				if f == "dev" && i+1 < len(fields) {
					route.Interface = fields[i+1]
				}
			}

			if fields[0] == "default" {
				route.IsDefault = true
				route.Network = "0.0.0.0/0"
			}

			config.Routes = append(config.Routes, route)
		}
	}
}

// cidrToMask converts CIDR notation to dotted decimal subnet mask
func cidrToMask(cidr int) string {
	mask := net.CIDRMask(cidr, 32)
	return fmt.Sprintf("%d.%d.%d.%d", mask[0], mask[1], mask[2], mask[3])
}

// -----------------------------------------------------------------------------
// Concurrent Assessment Runner
// -----------------------------------------------------------------------------

// AssessmentRunner manages concurrent device verification
type AssessmentRunner struct {
	verifier    *Verifier
	logger      *slog.Logger
	maxParallel int
}

// NewAssessmentRunner creates a new assessment runner
func NewAssessmentRunner(verifier *Verifier, logger *slog.Logger) *AssessmentRunner {
	return &AssessmentRunner{
		verifier:    verifier,
		logger:      logger,
		maxParallel: 5,
	}
}

// RunAssessment runs the complete assessment for a session
func (r *AssessmentRunner) RunAssessment(ctx context.Context, template models.AssessmentTemplate, vmIPs map[string]string, updateCh chan<- models.AssessmentUpdate) (*models.AssessmentResult, error) {
	result := &models.AssessmentResult{
		StartedAt:  time.Now(),
		Status:     "in_progress",
		Components: make([]models.ComponentResult, len(template.Components)),
		Devices:    make([]models.DeviceResult, 0, len(template.Devices)),
	}

	// Initialize component results
	componentMap := make(map[string]*models.ComponentResult)
	for i, comp := range template.Components {
		result.Components[i] = models.ComponentResult{
			ID:          comp.ID,
			Description: comp.Description,
			MaxPoints:   comp.Weight,
		}
		componentMap[comp.ID] = &result.Components[i]
	}

	// Run device verifications concurrently
	var wg sync.WaitGroup
	var mu sync.Mutex
	sem := make(chan struct{}, r.maxParallel)

	for _, device := range template.Devices {
		deviceIP, ok := vmIPs[device.Name]
		if !ok {
			r.logger.Warn("No IP for device", "device", device.Name)
			continue
		}

		wg.Add(1)
		go func(dev models.DeviceAssessment, ip string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			deviceResult, err := r.verifier.VerifyDevice(ctx, dev, ip)
			if err != nil {
				r.logger.Error("Device verification failed", "device", dev.Name, "error", err)
				return
			}

			mu.Lock()
			result.Devices = append(result.Devices, *deviceResult)

			// Update component totals
			for _, check := range deviceResult.Checks {
				if comp, ok := componentMap[check.Component]; ok {
					comp.TotalItems++
					if check.Status == models.AssessmentStatusCorrect {
						comp.PassedItems++
						comp.EarnedPoints += check.EarnedPoints
					}
				}
			}
			for _, iface := range deviceResult.Interfaces {
				for _, check := range iface.Checks {
					if comp, ok := componentMap[check.Component]; ok {
						comp.TotalItems++
						if check.Status == models.AssessmentStatusCorrect {
							comp.PassedItems++
							comp.EarnedPoints += check.EarnedPoints
						}
					}
				}
			}
			mu.Unlock()

			// Send update
			if updateCh != nil {
				updateCh <- models.AssessmentUpdate{
					Type:         "device_update",
					DeviceName:   dev.Name,
					Status:       deviceResult.Status,
					EarnedPoints: deviceResult.EarnedPoints,
					Timestamp:    time.Now(),
				}
			}
		}(device, deviceIP)
	}

	wg.Wait()

	// Calculate totals
	for _, device := range result.Devices {
		result.MaxScore += device.MaxPoints
		result.Score += device.EarnedPoints
		result.ItemCount += device.TotalItems
		result.PassedCount += device.PassedItems
	}

	if result.MaxScore > 0 {
		result.Percentage = float64(result.Score) / float64(result.MaxScore) * 100
	}

	result.LastChecked = time.Now()
	result.TimeElapsed = result.LastChecked.Sub(result.StartedAt).Round(time.Second).String()
	result.Status = "completed"

	// Calculate component percentages
	for i := range result.Components {
		if result.Components[i].MaxPoints > 0 {
			result.Components[i].Percentage = float64(result.Components[i].EarnedPoints) / float64(result.Components[i].MaxPoints) * 100
		}
	}

	return result, nil
}
