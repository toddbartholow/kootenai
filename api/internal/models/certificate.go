// Package models provides domain models for the Kootenai platform.
package models

import "time"

// Certificate represents a pathway completion certificate stored in the database
type Certificate struct {
	ID               string     `json:"id"`
	EnrollmentID     string     `json:"enrollmentId"`
	UserID           string     `json:"userId"`
	PathwayID        string     `json:"pathwayId"`
	VerificationCode string     `json:"verificationCode"`
	IssuedAt         time.Time  `json:"issuedAt"`
	ExpiresAt        *time.Time `json:"expiresAt,omitempty"`
	RevokedAt        *time.Time `json:"revokedAt,omitempty"`
	RevocationReason string     `json:"revocationReason,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
}

// IsValid returns true if the certificate is not revoked and not expired
func (c *Certificate) IsValid() bool {
	if c.RevokedAt != nil {
		return false
	}
	if c.ExpiresAt != nil && time.Now().After(*c.ExpiresAt) {
		return false
	}
	return true
}

// CertificateWithDetails includes certificate data along with user and pathway info
type CertificateWithDetails struct {
	Certificate
	PathwayName  string     `json:"pathwayName"`
	UserName     string     `json:"userName"`
	UserEmail    string     `json:"userEmail,omitempty"`
	EarnedPoints int        `json:"earnedPoints"`
	MaxPoints    int        `json:"maxPoints"`
	Percentage   float64    `json:"percentage"`
	CompletedAt  *time.Time `json:"completedAt,omitempty"`
}
