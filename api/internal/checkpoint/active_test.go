package checkpoint

import (
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

func TestValidateIP(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		wantErr bool
	}{
		// Valid IPv4 addresses
		{"valid IPv4", "192.168.1.1", false},
		{"valid IPv4 localhost", "127.0.0.1", false},
		{"valid IPv4 zeros", "0.0.0.0", false},
		{"valid IPv4 broadcast", "255.255.255.255", false},
		{"valid IPv4 private", "10.0.0.5", false},

		// Valid IPv6 addresses
		{"valid IPv6 localhost", "::1", false},
		{"valid IPv6 full", "2001:0db8:85a3:0000:0000:8a2e:0370:7334", false},
		{"valid IPv6 compressed", "2001:db8:85a3::8a2e:370:7334", false},

		// Invalid addresses
		{"empty string", "", true},
		{"hostname", "example.com", true},
		{"hostname with port", "example.com:80", true},
		{"IP with port", "192.168.1.1:80", true},
		{"command injection attempt", "192.168.1.1; rm -rf /", true},
		{"command injection backtick", "192.168.1.1`whoami`", true},
		{"command injection pipe", "192.168.1.1|cat /etc/passwd", true},
		{"path traversal", "../../../etc/passwd", true},
		{"invalid octets", "256.256.256.256", true},
		{"partial IP", "192.168.1", true},
		{"too many octets", "192.168.1.1.1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateIP(tt.ip)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateIP(%q) error = %v, wantErr %v", tt.ip, err, tt.wantErr)
			}
			if tt.wantErr && err != nil && !errors.Is(err, ErrInvalidIP) {
				t.Errorf("validateIP(%q) error = %v, want ErrInvalidIP", tt.ip, err)
			}
		})
	}
}

func TestValidatePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		// Valid paths
		{"simple file", "/etc/passwd", false},
		{"nested path", "/var/log/syslog", false},
		{"home directory", "/home/user/file.txt", false},
		{"relative path", "file.txt", false},
		{"path with dots", "/path/to/file.conf", false},
		{"path with dashes", "/var/log/my-app.log", false},
		{"path with underscores", "/var/log/my_app.log", false},
		{"path with numbers", "/var/log/app123.log", false},

		// Invalid paths - shell metacharacters
		{"empty string", "", true},
		{"semicolon injection", "/etc/passwd; rm -rf /", true},
		{"pipe injection", "/etc/passwd|cat", true},
		{"ampersand injection", "/etc/passwd&whoami", true},
		{"dollar injection", "/etc/$USER", true},
		{"backtick injection", "/etc/`whoami`", true},
		{"subshell injection", "/etc/$(whoami)", true},
		{"brace expansion", "/etc/{passwd,shadow}", true},
		{"redirect output", "/etc/passwd > /tmp/out", true},
		{"redirect input", "/etc/passwd < /tmp/in", true},
		{"backslash escape", "/etc/passwd\\n", true},
		{"double quote", "/etc/\"passwd\"", true},
		{"single quote", "/etc/'passwd'", true},
		{"glob asterisk", "/etc/*.conf", true},
		{"glob question", "/etc/pass??", true},
		{"bracket glob", "/etc/[a-z]*", true},
		{"exclamation history", "/etc/passwd!", true},
		{"hash comment", "/etc/passwd#comment", true},
		{"tilde expansion", "~/file", true},
		{"newline injection", "/etc/passwd\nrm -rf /", true},
		{"carriage return", "/etc/passwd\rmalicious", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
			if tt.wantErr && err != nil && !errors.Is(err, ErrInvalidPath) {
				t.Errorf("validatePath(%q) error = %v, want ErrInvalidPath", tt.path, err)
			}
		})
	}
}

func TestValidateTarget(t *testing.T) {
	tests := []struct {
		name    string
		target  string
		wantErr bool
	}{
		// Empty target is allowed (not all scripts use ${TARGET})
		{"empty string", "", false},

		// Valid targets
		{"hostname", "webserver", false},
		{"fqdn", "web.lab.local", false},
		{"ip address", "192.168.1.100", false},
		{"path-like target", "/var/log/syslog", false},
		{"alphanumeric", "target123", false},
		{"with dashes", "my-target-vm", false},
		{"with underscores", "my_target_vm", false},
		{"port number", "8080", false},
		{"host and port colon", "web.lab.local:8080", false},

		// Invalid targets - shell metacharacters
		{"semicolon injection", "target; rm -rf /", true},
		{"pipe injection", "target|cat /etc/passwd", true},
		{"ampersand injection", "target&whoami", true},
		{"dollar injection", "$HOME", true},
		{"backtick injection", "`whoami`", true},
		{"subshell injection", "$(whoami)", true},
		{"brace expansion", "{a,b}", true},
		{"redirect output", "target > /tmp/out", true},
		{"redirect input", "target < /tmp/in", true},
		{"backslash escape", "target\\n", true},
		{"double quote", "\"target\"", true},
		{"single quote", "'target'", true},
		{"glob asterisk", "target*", true},
		{"glob question", "target?", true},
		{"bracket glob", "[a-z]*", true},
		{"exclamation", "target!", true},
		{"hash comment", "target#comment", true},
		{"tilde expansion", "~/file", true},
		{"newline injection", "target\nrm -rf /", true},
		{"carriage return", "target\rmalicious", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTarget(tt.target)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTarget(%q) error = %v, wantErr %v", tt.target, err, tt.wantErr)
			}
			if tt.wantErr && err != nil && !errors.Is(err, ErrInvalidTarget) {
				t.Errorf("validateTarget(%q) error = %v, want ErrInvalidTarget", tt.target, err)
			}
		})
	}
}

func TestGetServicePort(t *testing.T) {
	tests := []struct {
		service string
		want    int
	}{
		{"ssh", 22},
		{"sshd", 22},
		{"http", 80},
		{"httpd", 80},
		{"apache2", 80},
		{"nginx", 80},
		{"https", 443},
		{"mysql", 3306},
		{"mariadb", 3306},
		{"postgres", 5432},
		{"postgresql", 5432},
		{"redis", 6379},
		{"mongodb", 27017},
		{"dns", 53},
		{"bind", 53},
		{"named", 53},
		{"smtp", 25},
		{"postfix", 25},
		{"ftp", 21},
		{"vsftpd", 21},
		{"telnet", 23},
		{"ldap", 389},
		{"ldaps", 636},
		{"ntp", 123},
		{"snmp", 161},
		{"docker", 2375},
		// Case insensitive
		{"SSH", 22},
		{"NGINX", 80},
		{"MySQL", 3306},
		// Unknown services
		{"unknown-service", 0},
		{"", 0},
		{"custom-app", 0},
	}

	for _, tt := range tests {
		t.Run(tt.service, func(t *testing.T) {
			got := getServicePort(tt.service)
			if got != tt.want {
				t.Errorf("getServicePort(%q) = %v, want %v", tt.service, got, tt.want)
			}
		})
	}
}

func TestNewActiveChecker(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	checker := NewActiveChecker(logger)

	if checker == nil {
		t.Fatal("NewActiveChecker returned nil")
	}

	if checker.httpClient == nil {
		t.Error("httpClient should be initialized")
	}

	if checker.checkTimeout == 0 {
		t.Error("checkTimeout should be non-zero")
	}
}

func TestNewActiveCheckRunner(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	eval := NewEvaluator(logger)
	runner := NewActiveCheckRunner(eval, logger)

	if runner == nil {
		t.Fatal("NewActiveCheckRunner returned nil")
	}

	if runner.checker == nil {
		t.Error("checker should be initialized")
	}

	if runner.sessions == nil {
		t.Error("sessions should be initialized")
	}

	if runner.interval == 0 {
		t.Error("interval should be non-zero")
	}
}

func TestActiveCheckRunnerRegisterUnregister(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	eval := NewEvaluator(logger)
	runner := NewActiveCheckRunner(eval, logger)

	// Register a session with active check triggers
	checkpoints := []Checkpoint{
		{
			ID: "network-check",
			Triggers: []CheckpointTrigger{
				{
					Type: TriggerTypeNetwork,
					Match: TriggerMatch{
						Port: 80,
					},
				},
			},
		},
	}

	// Convert to models.Checkpoint for the API
	modelCheckpoints := make([]models.Checkpoint, len(checkpoints))
	for i, cp := range checkpoints {
		triggers := make([]models.CheckpointTrigger, len(cp.Triggers))
		for j, t := range cp.Triggers {
			triggers[j] = models.CheckpointTrigger{
				Type:  models.TriggerType(t.Type),
				Match: models.TriggerMatch{Port: t.Match.Port},
			}
		}
		modelCheckpoints[i] = models.Checkpoint{
			ID:       cp.ID,
			Triggers: triggers,
		}
	}

	vmIPs := map[string]string{
		"webserver": "10.0.0.5",
	}

	runner.RegisterSession("session-123", "pod-456", modelCheckpoints, vmIPs, nil)

	runner.mu.RLock()
	_, exists := runner.sessions["session-123"]
	runner.mu.RUnlock()

	if !exists {
		t.Error("Session should be registered")
	}

	// Unregister
	runner.UnregisterSession("session-123")

	runner.mu.RLock()
	_, exists = runner.sessions["session-123"]
	runner.mu.RUnlock()

	if exists {
		t.Error("Session should be unregistered")
	}
}

func TestActiveCheckRunnerRegisterNoActiveChecks(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	eval := NewEvaluator(logger)
	runner := NewActiveCheckRunner(eval, logger)

	// Register a session without active check triggers
	checkpoints := []models.Checkpoint{
		{
			ID: "file-check",
			Triggers: []models.CheckpointTrigger{
				{
					Type:  models.TriggerTypeFileExists,
					Match: models.TriggerMatch{Path: "/etc/test"},
				},
			},
		},
	}

	vmIPs := map[string]string{
		"server": "10.0.0.5",
	}

	runner.RegisterSession("session-123", "pod-456", checkpoints, vmIPs, nil)

	runner.mu.RLock()
	_, exists := runner.sessions["session-123"]
	runner.mu.RUnlock()

	// With file_exists now included in active checks, this should be registered
	if !exists {
		t.Error("Session with file_exists checks should be registered")
	}
}

// Local types for test conversion
type Checkpoint struct {
	ID       string
	Triggers []CheckpointTrigger
}

type CheckpointTrigger struct {
	Type  TriggerType
	Match TriggerMatch
}

type TriggerType string

const (
	TriggerTypeNetwork    TriggerType = "network_connection"
	TriggerTypeFileExists TriggerType = "file_exists"
)

type TriggerMatch struct {
	Port int
	Path string
}
