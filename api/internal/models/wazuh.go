// Package models contains shared data types for the lab platform
package models

import (
	"encoding/json"
	"time"
)

// WazuhEvent represents an event received from a Wazuh agent
type WazuhEvent struct {
	ID          string          `json:"id"`
	Timestamp   time.Time       `json:"timestamp"`
	AgentID     string          `json:"agent_id"`
	AgentName   string          `json:"agent_name"`
	RuleID      int             `json:"rule_id"`
	RuleLevel   int             `json:"rule_level"`
	Description string          `json:"description"`
	Groups      []string        `json:"groups"`
	Location    string          `json:"location"`
	Decoder     string          `json:"decoder,omitempty"`
	Data        json.RawMessage `json:"data"` // Flexible payload
}

// SyscheckData represents Wazuh file integrity monitoring data
type SyscheckData struct {
	Path         string    `json:"path"`
	Event        string    `json:"event"` // added, modified, deleted, attributes
	Mode         string    `json:"mode,omitempty"`
	Permissions  string    `json:"permissions,omitempty"`
	Size         int64     `json:"size,omitempty"`
	MD5          string    `json:"md5,omitempty"`
	SHA1         string    `json:"sha1,omitempty"`
	SHA256       string    `json:"sha256,omitempty"`
	UID          string    `json:"uid,omitempty"`
	GID          string    `json:"gid,omitempty"`
	Owner        string    `json:"owner,omitempty"`
	Group        string    `json:"group,omitempty"`
	Changed      []string  `json:"changed_attributes,omitempty"`
	ChangedAttrs []string  `json:"changedAttrs,omitempty"`
	OldContent   string    `json:"old_content,omitempty"`
	NewContent   string    `json:"new_content,omitempty"`
	Diff         string    `json:"diff,omitempty"`
	ModifiedTime time.Time `json:"mtime,omitempty"`
	// Permission change tracking
	OldPerm  string `json:"old_perm,omitempty"`
	NewPerm  string `json:"new_perm,omitempty"`
	OldUser  string `json:"old_uname,omitempty"`
	NewUser  string `json:"new_uname,omitempty"`
	OldGroup string `json:"old_gname,omitempty"`
	NewGroup string `json:"new_gname,omitempty"`
}

// AuditData represents Wazuh audit log data (command execution)
type AuditData struct {
	Type      string `json:"type"`
	Command   string `json:"command,omitempty"`
	Exe       string `json:"exe,omitempty"`
	Success   bool   `json:"success,omitempty"`
	UID       string `json:"uid,omitempty"`
	AUID      string `json:"auid,omitempty"`
	EUID      string `json:"euid,omitempty"`
	User      string `json:"user,omitempty"`
	CWD       string `json:"cwd,omitempty"`
	Syscall   string `json:"syscall,omitempty"`
	Arch      string `json:"arch,omitempty"`
	Key       string `json:"key,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

// AuthData represents authentication event data
type AuthData struct {
	User    string `json:"user,omitempty"`
	SrcIP   string `json:"src_ip,omitempty"`
	Action  string `json:"action,omitempty"`
	Status  string `json:"status,omitempty"`
	Service string `json:"service,omitempty"`
	FullLog string `json:"full_log,omitempty"`
}

// PackageData represents package installation/removal data
type PackageData struct {
	Package string `json:"package"`
	Version string `json:"version,omitempty"`
	Arch    string `json:"arch,omitempty"`
	Action  string `json:"action"`            // install, remove, upgrade
	Manager string `json:"manager,omitempty"` // apt, yum, dnf, etc.
}

// ServiceData represents systemd/init service state changes
type ServiceData struct {
	Unit     string `json:"unit"`
	State    string `json:"state"` // active, inactive, failed
	SubState string `json:"sub_state,omitempty"`
	PID      int    `json:"pid,omitempty"`
	ExitCode int    `json:"exit_code,omitempty"`
}

// NetworkData represents network connection/state data
type NetworkData struct {
	Protocol   string `json:"protocol,omitempty"`    // tcp, udp, icmp
	LocalAddr  string `json:"local_addr,omitempty"`  // Local IP address
	LocalPort  int    `json:"local_port,omitempty"`  // Local port number
	RemoteAddr string `json:"remote_addr,omitempty"` // Remote IP address
	RemotePort int    `json:"remote_port,omitempty"` // Remote port number
	State      string `json:"state,omitempty"`       // ESTABLISHED, LISTEN, TIME_WAIT, etc.
	Process    string `json:"process,omitempty"`     // Process name/command
	PID        int    `json:"pid,omitempty"`         // Process ID
	User       string `json:"user,omitempty"`        // User owning the connection
	Interface  string `json:"interface,omitempty"`   // Network interface
	Direction  string `json:"direction,omitempty"`   // inbound, outbound
}

// UserData represents user account changes
type UserData struct {
	Username string   `json:"username"`
	UID      int      `json:"uid,omitempty"`
	GID      int      `json:"gid,omitempty"`
	Home     string   `json:"home,omitempty"`
	Shell    string   `json:"shell,omitempty"`
	Groups   []string `json:"groups,omitempty"`
	Action   string   `json:"action"` // created, modified, deleted
}

// DiskUsageData represents disk space monitoring data
type DiskUsageData struct {
	MountPoint  string  `json:"mount_point"`
	Device      string  `json:"device,omitempty"`
	TotalBytes  int64   `json:"total_bytes"`
	UsedBytes   int64   `json:"used_bytes"`
	AvailBytes  int64   `json:"avail_bytes"`
	UsedPercent float64 `json:"used_percent"`
	InodesTotal int64   `json:"inodes_total,omitempty"`
	InodesUsed  int64   `json:"inodes_used,omitempty"`
	InodesFree  int64   `json:"inodes_free,omitempty"`
}

// CPULoadData represents CPU load monitoring data
type CPULoadData struct {
	Load1       float64 `json:"load_1"`      // 1-minute load average
	Load5       float64 `json:"load_5"`      // 5-minute load average
	Load15      float64 `json:"load_15"`     // 15-minute load average
	CPUPercent  float64 `json:"cpu_percent"` // Current CPU usage percentage
	NumCPUs     int     `json:"num_cpus"`    // Number of CPUs
	UserPercent float64 `json:"user_percent,omitempty"`
	SysPercent  float64 `json:"sys_percent,omitempty"`
	IdlePercent float64 `json:"idle_percent,omitempty"`
}

// MemoryUsageData represents memory monitoring data
type MemoryUsageData struct {
	TotalBytes     int64   `json:"total_bytes"`
	UsedBytes      int64   `json:"used_bytes"`
	FreeBytes      int64   `json:"free_bytes"`
	AvailBytes     int64   `json:"avail_bytes"`
	UsedPercent    float64 `json:"used_percent"`
	SwapTotalBytes int64   `json:"swap_total_bytes,omitempty"`
	SwapUsedBytes  int64   `json:"swap_used_bytes,omitempty"`
	SwapFreeBytes  int64   `json:"swap_free_bytes,omitempty"`
	SwapPercent    float64 `json:"swap_percent,omitempty"`
}

// ProcessData represents process state monitoring data
type ProcessData struct {
	Name        string  `json:"name"`
	PID         int     `json:"pid"`
	PPID        int     `json:"ppid,omitempty"` // Parent PID
	State       string  `json:"state"`          // running, sleeping, stopped, zombie
	User        string  `json:"user,omitempty"`
	CPUPercent  float64 `json:"cpu_percent,omitempty"`
	MemPercent  float64 `json:"mem_percent,omitempty"`
	MemRSS      int64   `json:"mem_rss,omitempty"` // Resident set size
	CommandLine string  `json:"command_line,omitempty"`
	StartTime   string  `json:"start_time,omitempty"`
}

// PortListenData represents port listening status data
type PortListenData struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"` // tcp, udp
	Address  string `json:"address"`  // 0.0.0.0, 127.0.0.1, ::, etc.
	PID      int    `json:"pid,omitempty"`
	Process  string `json:"process,omitempty"`
	State    string `json:"state"` // LISTEN, etc.
	User     string `json:"user,omitempty"`
}

// CronJobData represents cron job configuration data
type CronJobData struct {
	User     string `json:"user"`
	Schedule string `json:"schedule"` // Cron expression (e.g., "0 * * * *")
	Command  string `json:"command"`
	Status   string `json:"status"` // active, disabled
}

// FirewallRuleData represents firewall rule configuration data
type FirewallRuleData struct {
	Chain       string `json:"chain"`  // INPUT, OUTPUT, FORWARD
	Action      string `json:"action"` // ACCEPT, DROP, REJECT
	Protocol    string `json:"protocol,omitempty"`
	SourceIP    string `json:"source_ip,omitempty"`
	DestIP      string `json:"dest_ip,omitempty"`
	SourcePort  int    `json:"source_port,omitempty"`
	DestPort    int    `json:"dest_port,omitempty"`
	Interface   string `json:"interface,omitempty"`
	RuleNumber  int    `json:"rule_number,omitempty"`
	Description string `json:"description,omitempty"`
}
