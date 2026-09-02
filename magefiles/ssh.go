//go:build mage

package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"golang.org/x/crypto/ssh"
)

// SSHClient wraps an SSH connection
type SSHClient struct {
	client   *ssh.Client
	config   *ssh.ClientConfig
	host     string
	user     string
	password string
	useShell bool // Use external ssh command instead of Go library
}

// SSHConnectConfig holds connection settings
type SSHConnectConfig struct {
	MaxRetries    int
	RetryInterval time.Duration
	Timeout       time.Duration
	Verbose       bool
}

// DefaultSSHConnectConfig returns sensible defaults
func DefaultSSHConnectConfig() SSHConnectConfig {
	return SSHConnectConfig{
		MaxRetries:    5,
		RetryInterval: 2 * time.Second,
		Timeout:       30 * time.Second,
		Verbose:       false,
	}
}

// NewSSHClient creates a new SSH client
func NewSSHClient(host, user, password string) (*SSHClient, error) {
	return NewSSHClientWithConfig(host, user, password, DefaultSSHConnectConfig())
}

// NewSSHClientWithConfig creates a new SSH client with custom connection settings
func NewSSHClientWithConfig(host, user, password string, connConfig SSHConnectConfig) (*SSHClient, error) {
	// First, verify the host is reachable at network level
	hostOnly := host
	if strings.Contains(host, ":") {
		hostOnly, _, _ = net.SplitHostPort(host)
	}

	if connConfig.Verbose {
		fmt.Printf("  Checking network connectivity to %s...\n", hostOnly)
	}

	if !isHostReachable(hostOnly, connConfig.Timeout) {
		// Try to provide helpful diagnostics
		return nil, fmt.Errorf("host %s is not reachable (network error). Check:\n"+
			"  1. Is the host powered on?\n"+
			"  2. Is the network configured correctly?\n"+
			"  3. Is there a firewall blocking access?\n"+
			"  Run: ping %s", hostOnly, hostOnly)
	}

	// PREFER external ssh command - it's more reliable across network configurations
	// Try key-based auth first (most common for dev environments)
	testCmd := exec.Command("ssh",
		"-o", "ConnectTimeout=5",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "BatchMode=yes",
		"-o", "PreferredAuthentications=publickey",
		fmt.Sprintf("%s@%s", user, hostOnly),
		"echo", "connected")

	if output, err := testCmd.CombinedOutput(); err == nil && strings.Contains(string(output), "connected") {
		if connConfig.Verbose {
			color.Green("  Connected via external SSH (key-based auth)")
		}
		return &SSHClient{
			host:     hostOnly,
			user:     user,
			password: password,
			useShell: true,
		}, nil
	}

	// If key auth failed and we have a password, try sshpass
	if password != "" {
		if _, err := exec.LookPath("sshpass"); err == nil {
			// Use SSHPASS env var instead of -p to avoid exposing password in process list
			os.Setenv("SSHPASS", password)
			testCmd = exec.Command("sshpass", "-e",
				"ssh",
				"-o", "ConnectTimeout=5",
				"-o", "StrictHostKeyChecking=accept-new",
				fmt.Sprintf("%s@%s", user, hostOnly),
				"echo", "connected")

			if output, err := testCmd.CombinedOutput(); err == nil && strings.Contains(string(output), "connected") {
				if connConfig.Verbose {
					color.Green("  Connected via sshpass")
				}
				return &SSHClient{
					host:     hostOnly,
					user:     user,
					password: password,
					useShell: true,
				}, nil
			}
		}
	}

	// Fallback to Go SSH library if external ssh doesn't work
	// (This can happen in restricted environments without external ssh)
	var authMethods []ssh.AuthMethod

	// Try public key auth first (from default locations)
	if signer := getSSHKeyAuth(); signer != nil {
		authMethods = append(authMethods, ssh.PublicKeys(signer))
		if connConfig.Verbose {
			fmt.Printf("  Trying Go SSH library with key auth\n")
		}
	}

	// Add password auth as fallback
	if password != "" {
		authMethods = append(authMethods,
			ssh.Password(password),
			ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range answers {
					answers[i] = password
				}
				return answers, nil
			}),
		)
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no authentication methods available (no SSH key found and no password provided)")
	}

	config := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		// TODO: Use known_hosts-based verification for production
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         connConfig.Timeout,
	}

	// Ensure host has port
	hostWithPort := host
	if !strings.Contains(host, ":") {
		hostWithPort = host + ":22"
	}

	// Try Go SSH library (single attempt, since external ssh already failed)
	client, err := ssh.Dial("tcp4", hostWithPort, config)
	if err == nil {
		if connConfig.Verbose {
			color.Green("  Connected via Go SSH library")
		}
		return &SSHClient{
			client:   client,
			config:   config,
			host:     hostWithPort,
			user:     user,
			password: password,
			useShell: false,
		}, nil
	}

	return nil, fmt.Errorf("failed to connect to %s.\nError: %v\n\nTroubleshooting:\n"+
		"  1. Verify SSH is running: ssh %s@%s\n"+
		"  2. Check SSH key: ssh-add -l\n"+
		"  3. Set INFRA_PASSWORD if using password auth\n"+
		"  4. Install sshpass for password auth: brew install hudochenkov/sshpass/sshpass",
		host, err, user, hostOnly)
}

// isHostReachable checks if a host is reachable via TCP on port 22
func isHostReachable(host string, timeout time.Duration) bool {
	// First try Go's net.DialTimeout with tcp4
	conn, err := net.DialTimeout("tcp4", host+":22", timeout)
	if err == nil {
		conn.Close()
		return true
	}

	// Fallback to generic tcp
	conn, err = net.DialTimeout("tcp", host+":22", timeout)
	if err == nil {
		conn.Close()
		return true
	}

	// Go's network stack can fail in some network configurations where
	// system tools work fine. Try nc/netcat as final fallback.
	timeoutSec := int(timeout.Seconds())
	if timeoutSec < 1 {
		timeoutSec = 5
	}

	// Try nc with timeout
	ncCmd := exec.Command("nc", "-z", "-w", fmt.Sprintf("%d", timeoutSec), host, "22")
	if ncCmd.Run() == nil {
		return true
	}

	return false
}

// isRecoverableError determines if an SSH error is worth retrying
func isRecoverableError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()

	// These errors are worth retrying
	recoverablePatterns := []string{
		"connection reset",
		"connection refused",
		"no route to host",
		"timeout",
		"temporary failure",
		"network is unreachable",
		"i/o timeout",
	}

	for _, pattern := range recoverablePatterns {
		if strings.Contains(strings.ToLower(errStr), pattern) {
			return true
		}
	}

	// Authentication errors are not recoverable
	if strings.Contains(errStr, "authentication") || strings.Contains(errStr, "permission denied") {
		return false
	}

	return true
}

// summarizeError provides a user-friendly error summary
func summarizeError(err error) string {
	if err == nil {
		return ""
	}
	errStr := err.Error()

	if strings.Contains(errStr, "no route to host") {
		return "no route to host (network issue)"
	}
	if strings.Contains(errStr, "connection refused") {
		return "connection refused (SSH not running?)"
	}
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "i/o timeout") {
		return "connection timeout"
	}
	if strings.Contains(errStr, "authentication") {
		return "authentication failed"
	}

	// Truncate long errors
	if len(errStr) > 80 {
		return errStr[:77] + "..."
	}
	return errStr
}

// Close closes the SSH connection
func (c *SSHClient) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// Run executes a command on the remote host
func (c *SSHClient) Run(cmd string) (string, error) {
	// Use external ssh command if in shell mode
	if c.useShell {
		return c.runWithExternalSSH(cmd)
	}

	session, err := c.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	if err := session.Run(cmd); err != nil {
		return "", fmt.Errorf("command failed: %w\nstderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// runWithExternalSSH runs a command using the external ssh binary
func (c *SSHClient) runWithExternalSSH(cmd string) (string, error) {
	sshCmd := exec.Command("ssh",
		"-o", "ConnectTimeout=30",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "BatchMode=yes",
		fmt.Sprintf("%s@%s", c.user, c.host),
		cmd)

	var stdout, stderr bytes.Buffer
	sshCmd.Stdout = &stdout
	sshCmd.Stderr = &stderr

	if err := sshCmd.Run(); err != nil {
		return "", fmt.Errorf("command failed: %w\nstderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// RunWithOutput executes a command and streams output
func (c *SSHClient) RunWithOutput(cmd string) error {
	// Use external ssh command if in shell mode
	if c.useShell {
		sshCmd := exec.Command("ssh",
			"-o", "ConnectTimeout=30",
			"-o", "StrictHostKeyChecking=accept-new",
			"-o", "BatchMode=yes",
			fmt.Sprintf("%s@%s", c.user, c.host),
			cmd)
		sshCmd.Stdout = os.Stdout
		sshCmd.Stderr = os.Stderr
		return sshCmd.Run()
	}

	session, err := c.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	return session.Run(cmd)
}

// CopyFile copies a local file to the remote host via SCP
func (c *SSHClient) CopyFile(localPath, remotePath string) error {
	// Open local file
	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer f.Close()

	// Get file info
	stat, err := f.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat local file: %w", err)
	}

	// Create session
	session, err := c.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Start the SCP command on remote
	go func() {
		w, _ := session.StdinPipe()
		defer w.Close()

		// Send file header
		fmt.Fprintf(w, "C0644 %d %s\n", stat.Size(), filepath.Base(remotePath))

		// Send file content
		io.Copy(w, f)

		// Send end marker
		fmt.Fprint(w, "\x00")
	}()

	// Run scp command
	remoteDir := filepath.Dir(remotePath)
	if err := session.Run(fmt.Sprintf("scp -t %s", remoteDir)); err != nil {
		return fmt.Errorf("scp failed: %w", err)
	}

	return nil
}

// CopyFileSimple copies a file using cat and shell redirection (more reliable)
func (c *SSHClient) CopyFileSimple(localPath, remotePath string) error {
	// Read local file
	content, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("failed to read local file: %w", err)
	}

	// Create session
	session, err := c.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Write content via stdin
	session.Stdin = bytes.NewReader(content)

	// Run cat to write file
	cmd := fmt.Sprintf("cat > %s", remotePath)
	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("failed to write remote file: %w", err)
	}

	return nil
}

// getSSHKeyAuth tries to load SSH private key from default locations
func getSSHKeyAuth() ssh.Signer {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	// Try common key locations
	keyPaths := []string{
		filepath.Join(home, ".ssh", "id_ed25519"),
		filepath.Join(home, ".ssh", "id_rsa"),
		filepath.Join(home, ".ssh", "id_ecdsa"),
	}

	for _, keyPath := range keyPaths {
		key, err := os.ReadFile(keyPath)
		if err != nil {
			continue
		}

		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			// Key might be encrypted - skip it
			continue
		}

		return signer
	}

	return nil
}

// ConnectToInfraVM creates an SSH client to the infra VM
func ConnectToInfraVM() (*SSHClient, error) {
	cfg := GetConfig()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return NewSSHClient(cfg.InfraVM.Host, cfg.InfraVM.User, cfg.InfraVM.Password)
}

// RunOnInfraVM executes a command on the infra VM
func RunOnInfraVM(cmd string) (string, error) {
	client, err := ConnectToInfraVM()
	if err != nil {
		return "", err
	}
	defer client.Close()
	return client.Run(cmd)
}

// RunOnInfraVMWithOutput executes a command on the infra VM with output streaming
func RunOnInfraVMWithOutput(cmd string) error {
	client, err := ConnectToInfraVM()
	if err != nil {
		return err
	}
	defer client.Close()
	return client.RunWithOutput(cmd)
}
