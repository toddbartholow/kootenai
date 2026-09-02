package models

import "time"

// -----------------------------------------------------------------------------
// Achievement System Types
// -----------------------------------------------------------------------------

// AchievementType represents the category of achievement
type AchievementType string

const (
	AchievementTypeLabCompletion   AchievementType = "lab_completion"     // Complete a specific lab
	AchievementTypePerfectScore    AchievementType = "perfect_score"      // 100% on a lab
	AchievementTypeStreak          AchievementType = "streak"             // Consecutive completions
	AchievementTypeSpeed           AchievementType = "speed"              // Complete quickly
	AchievementTypeCategory        AchievementType = "category"           // Complete all labs in category
	AchievementTypeMilestone       AchievementType = "milestone"          // X labs completed
	AchievementTypeSpecial         AchievementType = "special"            // Special/rare achievements
	AchievementTypePathway         AchievementType = "pathway"            // Pathway-related achievements
	AchievementTypePathwayComplete AchievementType = "pathway_completion" // Complete entire pathway
)

// AchievementTier represents the rarity/difficulty level
type AchievementTier string

const (
	AchievementTierBronze   AchievementTier = "bronze"
	AchievementTierSilver   AchievementTier = "silver"
	AchievementTierGold     AchievementTier = "gold"
	AchievementTierPlatinum AchievementTier = "platinum"
	AchievementTierDiamond  AchievementTier = "diamond"
)

// Achievement defines an achievement that can be earned
type Achievement struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Type        AchievementType     `json:"type"`
	Tier        AchievementTier     `json:"tier"`
	IconURL     string              `json:"iconUrl,omitempty"`
	Points      int                 `json:"points"`   // Achievement points (for leaderboards)
	IsSecret    bool                `json:"isSecret"` // Hidden until earned
	IsActive    bool                `json:"isActive"` // Can be earned
	Criteria    AchievementCriteria `json:"criteria"` // What's needed to earn it
	CreatedAt   time.Time           `json:"createdAt"`
	UpdatedAt   time.Time           `json:"updatedAt"`
}

// AchievementCriteria defines the requirements to earn an achievement
type AchievementCriteria struct {
	// Lab completion criteria
	LabTemplateID string   `json:"labTemplateId,omitempty"` // Specific lab to complete
	LabCategory   string   `json:"labCategory,omitempty"`   // Complete all in category
	LabTags       []string `json:"labTags,omitempty"`       // Labs with specific tags

	// Score criteria
	MinScore       int  `json:"minScore,omitempty"`       // Minimum percentage (0-100)
	RequirePerfect bool `json:"requirePerfect,omitempty"` // Must be 100%

	// Time criteria
	MaxDurationMins int `json:"maxDurationMins,omitempty"` // Complete within X minutes

	// Streak criteria
	StreakCount int    `json:"streakCount,omitempty"` // Consecutive completions
	StreakType  string `json:"streakType,omitempty"`  // daily, weekly, any

	// Milestone criteria
	TotalLabs   int `json:"totalLabs,omitempty"`   // Total labs completed
	TotalPoints int `json:"totalPoints,omitempty"` // Total points earned

	// Difficulty criteria
	MinDifficulty string `json:"minDifficulty,omitempty"` // beginner, intermediate, advanced, expert

	// Pathway criteria
	PathwayID       string `json:"pathwayId,omitempty"`       // Specific pathway to complete
	PathwaySlug     string `json:"pathwaySlug,omitempty"`     // Pathway slug (alternative to ID)
	ModulesRequired int    `json:"modulesRequired,omitempty"` // Number of modules to complete
	Type            string `json:"type,omitempty"`            // Criteria type (pathway_modules, pathway_complete, lab_perfect_score)

	// Special criteria
	CustomRule string `json:"customRule,omitempty"` // Custom rule identifier
}

// UserAchievement represents an achievement earned by a user
type UserAchievement struct {
	ID            string       `json:"id"`
	UserID        string       `json:"userId"`
	AchievementID string       `json:"achievementId"`
	Achievement   *Achievement `json:"achievement,omitempty"` // Full achievement details (for API responses)
	EarnedAt      time.Time    `json:"earnedAt"`
	SessionID     string       `json:"sessionId,omitempty"` // Which session earned it
	Progress      float64      `json:"progress,omitempty"`  // Progress towards earning (0-100)
	Notified      bool         `json:"notified"`            // User has been notified
	CreatedAt     time.Time    `json:"createdAt"`
}

// AchievementProgress tracks user progress towards achievements
type AchievementProgress struct {
	UserID        string         `json:"userId"`
	AchievementID string         `json:"achievementId"`
	Progress      float64        `json:"progress"` // 0-100
	Current       int            `json:"current"`  // Current count/value
	Required      int            `json:"required"` // Required count/value
	Earned        bool           `json:"earned"`
	EarnedAt      *time.Time     `json:"earnedAt,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"` // Additional tracking data
	UpdatedAt     time.Time      `json:"updatedAt"`
}

// UserAchievementSummary provides achievement statistics for a user
type UserAchievementSummary struct {
	UserID             string                  `json:"userId"`
	TotalEarned        int                     `json:"totalEarned"`
	TotalPoints        int                     `json:"totalPoints"`
	TotalAvailable     int                     `json:"totalAvailable"`
	CompletionPercent  float64                 `json:"completionPercent"`
	RecentAchievements []*UserAchievement      `json:"recentAchievements,omitempty"`
	RarityBreakdown    map[AchievementTier]int `json:"rarityBreakdown"` // Count per tier
	TypeBreakdown      map[AchievementType]int `json:"typeBreakdown"`   // Count per type
}

// AchievementWithProgress combines achievement definition with user progress
type AchievementWithProgress struct {
	Achievement
	Progress *AchievementProgress `json:"progress,omitempty"`
	Earned   bool                 `json:"earned"`
	EarnedAt *time.Time           `json:"earnedAt,omitempty"`
}

// AchievementNotification represents a notification to send to user
type AchievementNotification struct {
	UserID      string      `json:"userId"`
	Achievement Achievement `json:"achievement"`
	EarnedAt    time.Time   `json:"earnedAt"`
	SessionID   string      `json:"sessionId,omitempty"`
}

// LeaderboardEntry represents a user's position on the leaderboard
type LeaderboardEntry struct {
	Rank              int       `json:"rank"`
	UserID            string    `json:"userId"`
	Username          string    `json:"username"`
	DisplayName       string    `json:"displayName,omitempty"`
	TotalPoints       int       `json:"totalPoints"`
	TotalAchievements int       `json:"totalAchievements"`
	BronzeCount       int       `json:"bronzeCount"`
	SilverCount       int       `json:"silverCount"`
	GoldCount         int       `json:"goldCount"`
	PlatinumCount     int       `json:"platinumCount"`
	DiamondCount      int       `json:"diamondCount"`
	LastEarnedAt      time.Time `json:"lastEarnedAt,omitempty"`
}

// Leaderboard represents the full leaderboard response
type Leaderboard struct {
	Entries    []*LeaderboardEntry `json:"entries"`
	TotalUsers int                 `json:"totalUsers"`
	UpdatedAt  time.Time           `json:"updatedAt"`
	TimeRange  string              `json:"timeRange,omitempty"` // "all", "week", "month"
}
