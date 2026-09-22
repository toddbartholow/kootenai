// Package wazuh provides types and utilities for processing Wazuh alerts
package wazuh

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/events"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// Alert represents a Wazuh alert structure
// Based on Wazuh 4.x alert format
type Alert struct {
	Timestamp           string               `json:"timestamp"`
	Rule                Rule                 `json:"rule"`
	Agent               Agent                `json:"agent"`
	Manager             Manager              `json:"manager"`
	ID                  string               `json:"id"`
	FullLog             string               `json:"full_log,omitempty"`
	Decoder             Decoder              `json:"decoder,omitempty"`
	Data                AlertData            `json:"data,omitempty"`
	Location            string               `json:"location,omitempty"`
	Syscheck            *Syscheck            `json:"syscheck,omitempty"`
	SyscollectorPackage *SyscollectorPackage `json:"syscollector,omitempty"`
}

// Rule contains information about the triggered rule
type Rule struct {
	Level       int      `json:"level"`
	Description string   `json:"description"`
	ID          string   `json:"id"`
	MITRE       *MITRE   `json:"mitre,omitempty"`
	Groups      []string `json:"groups,omitempty"`
	GDPR        []string `json:"gdpr,omitempty"`
	PCI_DSS     []string `json:"pci_dss,omitempty"`
	Firedtimes  int      `json:"firedtimes,omitempty"`
}

// MITRE contains MITRE ATT&CK mapping
type MITRE struct {
	ID        []string `json:"id,omitempty"`
	Tactic    []string `json:"tactic,omitempty"`
	Technique []string `json:"technique,omitempty"`
}

// Agent contains information about the reporting agent
type Agent struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	IP     string `json:"ip,omitempty"`
	Labels Labels `json:"labels,omitempty"`
}

// Labels contains custom agent labels for pod/session mapping
type Labels struct {
	PodID     string `json:"pod_id,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	VMName    string `json:"vm_name,omitempty"`
}

// Manager contains Wazuh manager information
type Manager struct {
	Name string `json:"name"`
}

// Decoder contains decoder information
type Decoder struct {
	Name   string `json:"name,omitempty"`
	Parent string `json:"parent,omitempty"`
}

// AlertData contains additional alert data fields
type AlertData struct {
	// Audit data
	Audit *AuditData `json:"audit,omitempty"`

	// Vulnerability data
	Vulnerability *VulnerabilityData `json:"vulnerability,omitempty"`

	// General fields
	SrcIP    string `json:"srcip,omitempty"`
	DstIP    string `json:"dstip,omitempty"`
	SrcPort  string `json:"srcport,omitempty"`
	DstPort  string `json:"dstport,omitempty"`
	Protocol string `json:"protocol,omitempty"`
	Action   string `json:"action,omitempty"`
	Status   string `json:"status,omitempty"`
	URL      string `json:"url,omitempty"`
	User     string `json:"dstuser,omitempty"`
	UID      string `json:"uid,omitempty"`
	Command  string `json:"command,omitempty"`
}

// AuditData contains Linux audit log data
type AuditData struct {
	Type      string    `json:"type,omitempty"`
	Success   string    `json:"success,omitempty"`
	Syscall   string    `json:"syscall,omitempty"`
	UID       string    `json:"uid,omitempty"`
	AUID      string    `json:"auid,omitempty"`
	EUID      string    `json:"euid,omitempty"`
	GID       string    `json:"gid,omitempty"`
	EGID      string    `json:"egid,omitempty"`
	Exe       string    `json:"exe,omitempty"`
	Command   string    `json:"command,omitempty"`
	CWD       string    `json:"cwd,omitempty"`
	Key       string    `json:"key,omitempty"`
	Directory Directory `json:"directory,omitempty"`
	File      File      `json:"file,omitempty"`
	Execve    Execve    `json:"execve,omitempty"`
}

// Directory contains audit directory info
type Directory struct {
	Name string `json:"name,omitempty"`
}

// File contains audit file info
type File struct {
	Name string `json:"name,omitempty"`
}

// Execve contains executed command info
type Execve struct {
	A0 string `json:"a0,omitempty"`
	A1 string `json:"a1,omitempty"`
	A2 string `json:"a2,omitempty"`
	A3 string `json:"a3,omitempty"`
}

// VulnerabilityData contains vulnerability scan data
type VulnerabilityData struct {
	Package    string `json:"package,omitempty"`
	CVSS       string `json:"cvss,omitempty"`
	CVE        string `json:"cve,omitempty"`
	Title      string `json:"title,omitempty"`
	Severity   string `json:"severity,omitempty"`
	Published  string `json:"published,omitempty"`
	References string `json:"references,omitempty"`
}

// Syscheck contains file integrity monitoring data
type Syscheck struct {
	Path         string            `json:"path"`
	Mode         string            `json:"mode,omitempty"`
	Event        string            `json:"event"`                 // added, modified, deleted
	Size         string            `json:"size_after,omitempty"`  // Wazuh sends as string
	SizeBefore   string            `json:"size_before,omitempty"` // Wazuh sends as string
	Perm         string            `json:"perm_after,omitempty"`
	PermBefore   string            `json:"perm_before,omitempty"`
	UID          string            `json:"uid_after,omitempty"`
	GID          string            `json:"gid_after,omitempty"`
	MD5          string            `json:"md5_after,omitempty"`
	SHA1         string            `json:"sha1_after,omitempty"`
	SHA256       string            `json:"sha256_after,omitempty"`
	Uname        string            `json:"uname_after,omitempty"`
	Gname        string            `json:"gname_after,omitempty"`
	Mtime        string            `json:"mtime_after,omitempty"`
	Diff         string            `json:"diff,omitempty"`
	ChangedAttrs []string          `json:"changed_attributes,omitempty"`
	Tags         []string          `json:"tags,omitempty"`
	Attributes   map[string]string `json:"attributes,omitempty"`
}

// SyscollectorPackage contains package inventory data
type SyscollectorPackage struct {
	Package PackageInfo `json:"package,omitempty"`
	Hotfix  HotfixInfo  `json:"hotfix,omitempty"`
	Program ProgramInfo `json:"program,omitempty"`
}

// PackageInfo contains installed package data
type PackageInfo struct {
	Name         string `json:"name,omitempty"`
	Version      string `json:"version,omitempty"`
	Architecture string `json:"architecture,omitempty"`
	Vendor       string `json:"vendor,omitempty"`
	Description  string `json:"description,omitempty"`
	InstallTime  string `json:"install_time,omitempty"`
}

// HotfixInfo contains Windows hotfix data
type HotfixInfo struct {
	Hotfix string `json:"hotfix,omitempty"`
}

// ProgramInfo contains Windows program data
type ProgramInfo struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
	Vendor  string `json:"vendor,omitempty"`
}

// ParseAlert parses a JSON byte slice into an Alert
func ParseAlert(data []byte) (*Alert, error) {
	var alert Alert
	if err := json.Unmarshal(data, &alert); err != nil {
		return nil, fmt.Errorf("parsing alert JSON: %w", err)
	}
	return &alert, nil
}

// ToVMEvent converts a Wazuh alert to an internal VMEvent
func (a *Alert) ToVMEvent() (*events.VMEvent, error) {
	// Determine pod ID and VM name from agent labels or name
	podID := a.Agent.Labels.PodID
	vmName := a.Agent.Labels.VMName
	if vmName == "" {
		vmName = a.Agent.Name
	}

	// Parse timestamp
	timestamp, err := time.Parse(time.RFC3339, a.Timestamp)
	if err != nil {
		// Try alternative formats
		timestamp, err = time.Parse("2006-01-02T15:04:05.000-0700", a.Timestamp)
		if err != nil {
			timestamp = time.Now()
		}
	}

	// Determine event type from rule groups and alert content
	eventType := a.determineEventType()

	// Build event data based on type
	eventData, err := a.buildEventData(eventType)
	if err != nil {
		return nil, fmt.Errorf("building event data: %w", err)
	}

	event := &events.VMEvent{
		MessageHeader: events.NewMessageHeader("wazuh-webhook"),
		PodID:         podID,
		VMName:        vmName,
		EventType:     eventType,
		Timestamp:     timestamp,
		Data:          eventData,
		WazuhAlertID:  a.ID,
		WazuhRuleID:   a.Rule.ID,
		WazuhRuleDesc: a.Rule.Description,
		WazuhLevel:    a.Rule.Level,
	}

	return event, nil
}

// determineEventType maps Wazuh alert to internal event type
func (a *Alert) determineEventType() string {
	// Check rule groups first
	for _, group := range a.Rule.Groups {
		group = strings.ToLower(group)
		switch {
		case strings.Contains(group, "syscheck"):
			return events.EventTypeSyscheck
		case strings.Contains(group, "audit"):
			return events.EventTypeAudit
		case group == "sudo":
			// Sudo events contain command execution info, treat as audit
			return events.EventTypeAudit
		case strings.Contains(group, "authentication") || strings.Contains(group, "pam"):
			return events.EventTypeAuth
		case strings.Contains(group, "sshd"):
			return events.EventTypeSSH
		case strings.Contains(group, "firewall") || strings.Contains(group, "iptables"):
			return events.EventTypeFirewall
		case strings.Contains(group, "syscollector"):
			return events.EventTypePackage
		case strings.Contains(group, "vulnerability"):
			return events.EventTypeVulnerability
		}
	}

	// Check by content
	if a.Syscheck != nil {
		return events.EventTypeSyscheck
	}
	if a.Data.Audit != nil {
		return events.EventTypeAudit
	}
	// Sudo events have command data
	if a.Data.Command != "" {
		return events.EventTypeAudit
	}
	if a.SyscollectorPackage != nil {
		return events.EventTypePackage
	}

	// Check decoder
	switch a.Decoder.Name {
	case "sshd":
		return events.EventTypeSSH
	case "pam":
		return events.EventTypeAuth
	case "systemd":
		return events.EventTypeService
	case "sudo":
		return events.EventTypeAudit
	}

	return events.EventTypeGeneric
}

// buildEventData builds the appropriate data payload based on event type
func (a *Alert) buildEventData(eventType string) (json.RawMessage, error) {
	var data any

	switch eventType {
	case events.EventTypeSyscheck:
		if a.Syscheck != nil {
			// Parse size from string (Wazuh sends as string)
			var size int64
			if a.Syscheck.Size != "" {
				if parsed, err := strconv.ParseInt(a.Syscheck.Size, 10, 64); err == nil {
					size = parsed
				}
			}
			data = models.SyscheckData{
				Path:         a.Syscheck.Path,
				Event:        a.Syscheck.Event,
				Size:         size,
				Permissions:  a.Syscheck.Perm,
				UID:          a.Syscheck.UID,
				GID:          a.Syscheck.GID,
				MD5:          a.Syscheck.MD5,
				SHA1:         a.Syscheck.SHA1,
				SHA256:       a.Syscheck.SHA256,
				Diff:         a.Syscheck.Diff,
				ChangedAttrs: a.Syscheck.ChangedAttrs,
			}
		}

	case events.EventTypeAudit:
		if a.Data.Audit != nil {
			// Build full command from execve args
			cmd := a.Data.Audit.Command
			if cmd == "" && a.Data.Audit.Execve.A0 != "" {
				parts := []string{a.Data.Audit.Execve.A0}
				if a.Data.Audit.Execve.A1 != "" {
					parts = append(parts, a.Data.Audit.Execve.A1)
				}
				if a.Data.Audit.Execve.A2 != "" {
					parts = append(parts, a.Data.Audit.Execve.A2)
				}
				if a.Data.Audit.Execve.A3 != "" {
					parts = append(parts, a.Data.Audit.Execve.A3)
				}
				cmd = strings.Join(parts, " ")
			}

			data = models.AuditData{
				Type:    a.Data.Audit.Type,
				Syscall: a.Data.Audit.Syscall,
				Success: a.Data.Audit.Success == "yes",
				UID:     a.Data.Audit.UID,
				AUID:    a.Data.Audit.AUID,
				EUID:    a.Data.Audit.EUID,
				Exe:     a.Data.Audit.Exe,
				Command: cmd,
				CWD:     a.Data.Audit.CWD,
				Key:     a.Data.Audit.Key,
			}
		} else if a.Data.Command != "" {
			// Sudo events have command in data.command field
			// Extract the actual command (last part after COMMAND=)
			cmd := a.Data.Command
			data = models.AuditData{
				Type:    "sudo",
				Success: true,
				User:    a.Data.User,
				Command: cmd,
				Exe:     cmd, // Also set exe for pattern matching
			}
		}

	case events.EventTypePackage:
		if a.SyscollectorPackage != nil {
			data = models.PackageData{
				Package: a.SyscollectorPackage.Package.Name,
				Version: a.SyscollectorPackage.Package.Version,
				Arch:    a.SyscollectorPackage.Package.Architecture,
				Action:  "installed", // syscollector reports installed packages
			}
		}

	case events.EventTypeAuth, events.EventTypeSSH:
		data = models.AuthData{
			User:    a.Data.User,
			SrcIP:   a.Data.SrcIP,
			Action:  a.Data.Action,
			Status:  a.Data.Status,
			FullLog: a.FullLog,
		}

	default:
		// Generic data - include full log and rule info
		data = map[string]any{
			"full_log":    a.FullLog,
			"rule_id":     a.Rule.ID,
			"rule_desc":   a.Rule.Description,
			"rule_level":  a.Rule.Level,
			"rule_groups": a.Rule.Groups,
		}
	}

	return json.Marshal(data)
}

// GetSessionID returns the session ID from agent labels
func (a *Alert) GetSessionID() string {
	return a.Agent.Labels.SessionID
}

// GetPodID returns the pod ID from agent labels
func (a *Alert) GetPodID() string {
	return a.Agent.Labels.PodID
}
