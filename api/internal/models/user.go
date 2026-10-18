// Package models contains shared data types for the lab platform
package models

import (
	"encoding/json"
	"time"
)

// User represents a user entity for database operations
type User struct {
	ID                 string            `json:"id"`
	ExternalID         string            `json:"externalId"` // FreeIPA uid
	Username           string            `json:"username"`
	Email              string            `json:"email,omitempty"`
	DisplayName        string            `json:"displayName,omitempty"`
	Role               string            `json:"role"` // student, instructor, admin
	IsActive           bool              `json:"isActive"`
	PasswordHash       string            `json:"-"`                            // Never serialize - bcrypt hash
	MustChangePassword bool              `json:"mustChangePassword,omitempty"` // Force password change on next login
	PasswordUpdatedAt  *time.Time        `json:"passwordUpdatedAt,omitempty"`  // Last password change timestamp
	PreferredLocale    *string           `json:"preferredLocale,omitempty"`    // BCP-47 tag (e.g. "en", "es"); NULL falls back to Accept-Language
	CreatedAt          time.Time         `json:"createdAt"`
	UpdatedAt          time.Time         `json:"updatedAt"`
	LastLoginAt        *time.Time        `json:"lastLoginAt,omitempty"`
	Metadata           map[string]string `json:"metadata,omitempty"`
}

// PasswordResetToken represents a token for password reset
type PasswordResetToken struct {
	ID        string     `json:"id"`
	UserID    string     `json:"userId"`
	TokenHash string     `json:"-"` // Never expose - stores SHA256 hash of token
	ExpiresAt time.Time  `json:"expiresAt"`
	UsedAt    *time.Time `json:"usedAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}

// AuditEntry represents an audit log entry
type AuditEntry struct {
	ID           int64           `json:"id"`
	Timestamp    time.Time       `json:"timestamp"`
	ActorID      string          `json:"actorId"`
	ActorType    string          `json:"actorType"` // user, system, api
	Action       string          `json:"action"`
	ResourceType string          `json:"resourceType"`
	ResourceID   string          `json:"resourceId,omitempty"`
	Details      json.RawMessage `json:"details,omitempty" swaggertype:"object"`
	IPAddress    string          `json:"ipAddress,omitempty"`
	UserAgent    string          `json:"userAgent,omitempty"`
}
