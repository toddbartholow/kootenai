// Package models contains assessment data structures for Packet Tracer-style grading
package models

import "time"

// AssessmentStatus represents the status of an individual check
type AssessmentStatus string

const (
	AssessmentStatusCorrect    AssessmentStatus = "correct"
	AssessmentStatusIncorrect  AssessmentStatus = "incorrect"
	AssessmentStatusIncomplete AssessmentStatus = "incomplete"
	AssessmentStatusPending    AssessmentStatus = "pending"
	AssessmentStatusError      AssessmentStatus = "error"
)

// -----------------------------------------------------------------------------
// Assessment Template Schema (defined in lab YAML)
// -----------------------------------------------------------------------------

// AssessmentTemplate defines the hierarchical assessment structure
type AssessmentTemplate struct {
	Components []AssessmentComponent `yaml:"components" json:"components"`
	Devices    []DeviceAssessment    `yaml:"devices" json:"devices"`
}

// AssessmentComponent groups related checks (e.g., "VLSM Addressing", "Gateway Config")
type AssessmentComponent struct {
	ID          string `yaml:"id" json:"id"`
	Description string `yaml:"description" json:"description"`
	Weight      int    `yaml:"weight" json:"weight"` // Max points for this component
}

// DeviceAssessment defines checks for a single device
type DeviceAssessment struct {
	Name       string                `yaml:"name" json:"name"`
	Type       string                `yaml:"type" json:"type"` // router, switch, host, firewall
	Checks     []AssessmentCheck     `yaml:"checks,omitempty" json:"checks,omitempty"`
	Interfaces []InterfaceAssessment `yaml:"interfaces,omitempty" json:"interfaces,omitempty"`
}

// InterfaceAssessment defines checks for a device interface
type InterfaceAssessment struct {
	Name   string            `yaml:"name" json:"name"`
	Checks []AssessmentCheck `yaml:"checks" json:"checks"`
}

// AssessmentCheck defines a single verifiable item
type AssessmentCheck struct {
	ID          string       `yaml:"id" json:"id"`
	Description string       `yaml:"description" json:"description"`
	Component   string       `yaml:"component" json:"component"` // References AssessmentComponent.ID
	Points      int          `yaml:"points" json:"points"`
	Verify      VerifyConfig `yaml:"verify" json:"verify"`
}

// VerifyConfig defines how to verify a check
type VerifyConfig struct {
	Type     VerifyType `yaml:"type" json:"type"`
	Command  string     `yaml:"command,omitempty" json:"command,omitempty"`   // Show command to run
	Path     string     `yaml:"path,omitempty" json:"path,omitempty"`         // Config path (e.g., "interface Vlan1 → ip address")
	Field    string     `yaml:"field,omitempty" json:"field,omitempty"`       // Field to extract from parsed output
	Expected string     `yaml:"expected" json:"expected"`                     // Expected value
	Regex    string     `yaml:"regex,omitempty" json:"regex,omitempty"`       // Regex pattern for complex matching
	Operator string     `yaml:"operator,omitempty" json:"operator,omitempty"` // eq, ne, gt, lt, contains, matches
}

// VerifyType defines the type of verification
type VerifyType string

const (
	VerifyTypeConfigValue     VerifyType = "config_value"     // Check a specific config value
	VerifyTypeInterfaceStatus VerifyType = "interface_status" // Check interface up/down status
	VerifyTypeInterfaceIP     VerifyType = "interface_ip"     // Check interface IP address
	VerifyTypeSubnetMask      VerifyType = "subnet_mask"      // Check subnet mask
	VerifyTypeDefaultGateway  VerifyType = "default_gateway"  // Check default gateway
	VerifyTypeRouteExists     VerifyType = "route_exists"     // Check if route exists in routing table
	VerifyTypeACLRule         VerifyType = "acl_rule"         // Check ACL configuration
	VerifyTypeServiceRunning  VerifyType = "service_running"  // Check if service is running (Linux)
	VerifyTypeFileContent     VerifyType = "file_content"     // Check file content
	VerifyTypeConnectivity    VerifyType = "connectivity"     // Ping/TCP connectivity test
	VerifyTypeCommand         VerifyType = "command"          // Run command, check exit code or output
)

// -----------------------------------------------------------------------------
// Assessment Results (runtime state)
// -----------------------------------------------------------------------------

// AssessmentResult represents the complete assessment state for a session
type AssessmentResult struct {
	SessionID   string            `json:"sessionId"`
	Score       int               `json:"score"`
	MaxScore    int               `json:"maxScore"`
	Percentage  float64           `json:"percentage"`
	ItemCount   int               `json:"itemCount"`
	PassedCount int               `json:"passedCount"`
	Status      string            `json:"status"` // in_progress, completed, graded
	StartedAt   time.Time         `json:"startedAt"`
	LastChecked time.Time         `json:"lastChecked"`
	Components  []ComponentResult `json:"components"`
	Devices     []DeviceResult    `json:"devices"`
	TimeElapsed string            `json:"timeElapsed"`
}

// ComponentResult represents aggregated results for a component
type ComponentResult struct {
	ID           string  `json:"id"`
	Description  string  `json:"description"`
	TotalItems   int     `json:"totalItems"`
	PassedItems  int     `json:"passedItems"`
	MaxPoints    int     `json:"maxPoints"`
	EarnedPoints int     `json:"earnedPoints"`
	Percentage   float64 `json:"percentage"`
}

// DeviceResult represents assessment results for a device
type DeviceResult struct {
	Name         string            `json:"name"`
	Type         string            `json:"type"`
	Status       AssessmentStatus  `json:"status"` // Aggregate status
	Checks       []CheckResult     `json:"checks,omitempty"`
	Interfaces   []InterfaceResult `json:"interfaces,omitempty"`
	TotalItems   int               `json:"totalItems"`
	PassedItems  int               `json:"passedItems"`
	EarnedPoints int               `json:"earnedPoints"`
	MaxPoints    int               `json:"maxPoints"`
}

// InterfaceResult represents assessment results for an interface
type InterfaceResult struct {
	Name         string           `json:"name"`
	Status       AssessmentStatus `json:"status"`
	Checks       []CheckResult    `json:"checks"`
	TotalItems   int              `json:"totalItems"`
	PassedItems  int              `json:"passedItems"`
	EarnedPoints int              `json:"earnedPoints"`
	MaxPoints    int              `json:"maxPoints"`
}

// CheckResult represents the result of a single assessment check
type CheckResult struct {
	ID           string           `json:"id"`
	Description  string           `json:"description"`
	Component    string           `json:"component"`
	Status       AssessmentStatus `json:"status"`
	Expected     string           `json:"expected"`
	Actual       string           `json:"actual"`
	Points       int              `json:"points"`
	EarnedPoints int              `json:"earnedPoints"`
	Feedback     string           `json:"feedback"`
	CheckedAt    *time.Time       `json:"checkedAt,omitempty"`
	Error        string           `json:"error,omitempty"`
}

// -----------------------------------------------------------------------------
// Assessment Events (for WebSocket broadcast)
// -----------------------------------------------------------------------------

// AssessmentUpdate represents a real-time update to assessment state
type AssessmentUpdate struct {
	Type          string           `json:"type"` // check_update, device_update, component_update, complete
	SessionID     string           `json:"sessionId"`
	DeviceName    string           `json:"deviceName,omitempty"`
	InterfaceName string           `json:"interfaceName,omitempty"`
	CheckID       string           `json:"checkId,omitempty"`
	Status        AssessmentStatus `json:"status"`
	Expected      string           `json:"expected,omitempty"`
	Actual        string           `json:"actual,omitempty"`
	Points        int              `json:"points,omitempty"`
	EarnedPoints  int              `json:"earnedPoints,omitempty"`
	Feedback      string           `json:"feedback,omitempty"`
	Timestamp     time.Time        `json:"timestamp"`

	// Aggregates (for component/device updates)
	TotalScore  int     `json:"totalScore,omitempty"`
	MaxScore    int     `json:"maxScore,omitempty"`
	PassedItems int     `json:"passedItems,omitempty"`
	TotalItems  int     `json:"totalItems,omitempty"`
	Percentage  float64 `json:"percentage,omitempty"`
}

// -----------------------------------------------------------------------------
// Network Device Configuration (parsed structures)
// -----------------------------------------------------------------------------

// InterfaceConfig represents parsed interface configuration
type InterfaceConfig struct {
	Name           string `json:"name"`
	IPAddress      string `json:"ipAddress"`
	SubnetMask     string `json:"subnetMask"`
	CIDR           int    `json:"cidr"`
	Status         string `json:"status"`   // up/down
	Protocol       string `json:"protocol"` // up/down
	Description    string `json:"description"`
	Speed          string `json:"speed"`
	Duplex         string `json:"duplex"`
	MTU            int    `json:"mtu"`
	MACAddress     string `json:"macAddress"`
	VlanID         int    `json:"vlanId,omitempty"`
	Switchport     bool   `json:"switchport"`
	SwitchportMode string `json:"switchportMode"` // access, trunk
}

// Route represents a routing table entry
type Route struct {
	Network   string `json:"network"`
	Mask      string `json:"mask"`
	NextHop   string `json:"nextHop"`
	Interface string `json:"interface"`
	Protocol  string `json:"protocol"` // connected, static, ospf, eigrp, bgp
	Metric    int    `json:"metric"`
	AdminDist int    `json:"adminDistance"`
	IsDefault bool   `json:"isDefault"`
}

// DeviceConfig represents parsed device configuration
type DeviceConfig struct {
	Hostname       string                     `json:"hostname"`
	DefaultGateway string                     `json:"defaultGateway"`
	DNSServers     []string                   `json:"dnsServers"`
	Interfaces     map[string]InterfaceConfig `json:"interfaces"`
	Routes         []Route                    `json:"routes"`
	VLANs          map[int]string             `json:"vlans"` // VLAN ID -> Name
	ACLs           map[string][]ACLRule       `json:"acls"`
	Services       map[string]bool            `json:"services"` // service name -> enabled
}

// ACLRule represents an access control list rule
type ACLRule struct {
	Sequence    int    `json:"sequence"`
	Action      string `json:"action"` // permit, deny
	Protocol    string `json:"protocol"`
	Source      string `json:"source"`
	SourcePort  string `json:"sourcePort,omitempty"`
	Destination string `json:"destination"`
	DestPort    string `json:"destPort,omitempty"`
	Log         bool   `json:"log"`
}
