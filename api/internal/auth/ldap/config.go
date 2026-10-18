// Package ldap provides FreeIPA/LDAP authentication for the Kootenai
// platform. It implements the search+bind pattern: a service account searches
// for the user, then the user's own credentials are used to bind (verify).
//
// TLS is required by default (fail-closed). Group membership is extracted
// and mapped to platform roles via configurable mappings.
package ldap

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Config holds LDAP connection and authentication settings.
// Loaded from environment variables via config.LoadFromEnv().
type Config struct {
	// Connection
	Host          string `yaml:"host"`            // LDAP server hostname (required)
	Port          int    `yaml:"port"`            // Default: 636 (LDAPS) or 389 (StartTLS)
	UseTLS        bool   `yaml:"use_tls"`         // Require TLS (default: true)
	StartTLS      bool   `yaml:"start_tls"`       // Use StartTLS instead of LDAPS
	TLSSkipVerify bool   `yaml:"tls_skip_verify"` // Skip TLS cert verification (dev only)
	TLSCACertPath string `yaml:"tls_ca_cert"`     // Path to custom CA certificate

	// Service account for searches
	BindDN       string `yaml:"bind_dn"`       // Service account DN (required)
	BindPassword string `yaml:"bind_password"` // Service account password (required)

	// User search
	BaseDN           string           `yaml:"base_dn"`            // Base DN for user searches (required)
	UserSearchFilter string           `yaml:"user_search_filter"` // LDAP filter; %s = username (default: "(uid=%s)")
	UserAttrMap      UserAttributeMap `yaml:"user_attr_map"`

	// Group/role mapping
	GroupBaseDN      string            `yaml:"group_base_dn"`      // Base DN for group searches (optional)
	GroupRoleMapping map[string]string `yaml:"group_role_mapping"` // LDAP group DN → platform role
	DefaultRole      string            `yaml:"default_role"`       // Fallback role (default: "student")

	// Routing
	DefaultDomain string `yaml:"default_domain"` // Email domain for LDAP routing (e.g., "example.com")
	AuthFirst     bool   `yaml:"auth_first"`     // Try LDAP before local auth

	// Pool
	MaxIdleConns    int           `yaml:"max_idle_conns"`    // Connection pool size (default: 5)
	ConnIdleTimeout time.Duration `yaml:"conn_idle_timeout"` // Idle connection timeout (default: 60s)
	SearchTimeout   time.Duration `yaml:"search_timeout"`    // LDAP search timeout (default: 10s)
}

// UserAttributeMap maps LDAP attributes to user model fields.
type UserAttributeMap struct {
	UID         string `yaml:"uid"`          // default: "uid"
	Email       string `yaml:"email"`        // default: "mail"
	DisplayName string `yaml:"display_name"` // default: "displayName"
	FirstName   string `yaml:"first_name"`   // default: "givenName"
	LastName    string `yaml:"last_name"`    // default: "sn"
	MemberOf    string `yaml:"member_of"`    // default: "memberOf"
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Port:             636,
		UseTLS:           true,
		UserSearchFilter: "(uid=%s)",
		DefaultRole:      "student",
		MaxIdleConns:     5,
		ConnIdleTimeout:  60 * time.Second,
		SearchTimeout:    10 * time.Second,
		UserAttrMap:      DefaultUserAttributeMap(),
	}
}

// DefaultUserAttributeMap returns the standard FreeIPA/OpenLDAP attribute mapping.
func DefaultUserAttributeMap() UserAttributeMap {
	return UserAttributeMap{
		UID:         "uid",
		Email:       "mail",
		DisplayName: "displayName",
		FirstName:   "givenName",
		LastName:    "sn",
		MemberOf:    "memberOf",
	}
}

// Validate checks that all required fields are set and TLS is enabled.
func (c *Config) Validate() error {
	if c.Host == "" {
		return errors.New("ldap: host is required")
	}
	if !c.UseTLS && !c.StartTLS {
		return errors.New("ldap: TLS or StartTLS is required for secure connections")
	}
	if c.BindDN == "" {
		return errors.New("ldap: bind_dn (service account) is required")
	}
	if c.BindPassword == "" {
		return errors.New("ldap: bind_password is required")
	}
	if c.BaseDN == "" {
		return errors.New("ldap: base_dn is required")
	}
	return nil
}

// Addr returns the host:port address string.
func (c *Config) Addr() string {
	port := c.Port
	if port == 0 {
		if c.UseTLS && !c.StartTLS {
			port = 636
		} else {
			port = 389
		}
	}
	return fmt.Sprintf("%s:%d", c.Host, port)
}

// SearchFilter returns the user search filter with the username substituted.
func (c *Config) SearchFilter(username string) string {
	filter := c.UserSearchFilter
	if filter == "" {
		filter = "(uid=%s)"
	}
	return fmt.Sprintf(filter, ldapEscape(username))
}

// SearchAttributes returns the list of LDAP attributes to request.
func (c *Config) SearchAttributes() []string {
	m := c.UserAttrMap
	attrs := []string{"dn", m.UID, m.Email, m.DisplayName, m.FirstName, m.LastName}
	if m.MemberOf != "" {
		attrs = append(attrs, m.MemberOf)
	}
	return attrs
}

// MapGroupsToRoles maps LDAP group DNs to platform roles using GroupRoleMapping.
// Returns the highest-priority matching role, or DefaultRole if no match.
func (c *Config) MapGroupsToRoles(groups []string) string {
	if len(c.GroupRoleMapping) == 0 {
		return c.effectiveDefaultRole()
	}

	// Role priority: admin > instructor > student > member
	priority := map[string]int{"admin": 4, "instructor": 3, "student": 2, "member": 1}
	bestRole := ""
	bestPriority := 0

	for _, group := range groups {
		groupLower := strings.ToLower(group)
		for pattern, role := range c.GroupRoleMapping {
			if strings.ToLower(pattern) == groupLower {
				if p := priority[role]; p > bestPriority {
					bestRole = role
					bestPriority = p
				}
			}
		}
	}

	if bestRole == "" {
		return c.effectiveDefaultRole()
	}
	return bestRole
}

func (c *Config) effectiveDefaultRole() string {
	if c.DefaultRole != "" {
		return c.DefaultRole
	}
	return "student"
}

// ldapEscape escapes special characters in LDAP search filter values per RFC 4515.
func ldapEscape(s string) string {
	var b strings.Builder
	for _, c := range s {
		switch c {
		case '\\', '*', '(', ')', '\x00':
			fmt.Fprintf(&b, "\\%02x", c)
		default:
			b.WriteRune(c)
		}
	}
	return b.String()
}
