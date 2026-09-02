// Package classroom provides AI classroom simulation for virtual student orchestration.
package classroom

// PersonalityType defines AI student personality archetypes.
type PersonalityType string

const (
	PersonalityHighPerformer        PersonalityType = "high_performer"
	PersonalityStruggling           PersonalityType = "struggling"
	PersonalityIndustryProfessional PersonalityType = "industry_professional"
)

// Traits holds personality trait scores (0-10 scale).
type Traits struct {
	Motivation        int `json:"motivation"`
	Conscientiousness int `json:"conscientiousness"`
	Confidence        int `json:"confidence"`
	Anxiety           int `json:"anxiety"`
}

// TechSkills holds technical skill levels (0-10 scale).
type TechSkills struct {
	Linux      int `json:"linux"`
	Networking int `json:"networking"`
	Security   int `json:"security"`
	Scripting  int `json:"scripting"`
	CloudOps   int `json:"cloud_ops"`
}

// BehavioralConfig holds personality-driven behavioral parameters.
type BehavioralConfig struct {
	LLMTemperature    float64 `json:"llm_temperature"`
	QuizScoreMin      int     `json:"quiz_score_min"`
	QuizScoreMax      int     `json:"quiz_score_max"`
	AssignmentQuality string  `json:"assignment_quality"` // thorough|practical|incomplete
	DiscussionStyle   string  `json:"discussion_style"`   // insightful|practical|vague
	SubmitTiming      string  `json:"submit_timing"`      // early|on_time|last_minute
	ErrorRate         float64 `json:"error_rate"`         // 0.0-1.0 probability of errors
}

// PersonalityProfile is a complete profile template for an AI student personality.
type PersonalityProfile struct {
	Personality      PersonalityType  `json:"personality"`
	Traits           Traits           `json:"traits"`
	TechSkills       TechSkills       `json:"tech_skills"`
	BehavioralConfig BehavioralConfig `json:"behavioral_config"`
}

// DefaultProfiles returns the 3 personality profile templates.
func DefaultProfiles() map[PersonalityType]PersonalityProfile {
	return map[PersonalityType]PersonalityProfile{
		PersonalityHighPerformer: {
			Personality: PersonalityHighPerformer,
			Traits: Traits{
				Motivation: 9, Conscientiousness: 9, Confidence: 8, Anxiety: 3,
			},
			TechSkills: TechSkills{
				Linux: 8, Networking: 7, Security: 8, Scripting: 7, CloudOps: 6,
			},
			BehavioralConfig: BehavioralConfig{
				LLMTemperature:    0.4,
				QuizScoreMin:      85,
				QuizScoreMax:      95,
				AssignmentQuality: "thorough",
				DiscussionStyle:   "insightful",
				SubmitTiming:      "early",
				ErrorRate:         0.05,
			},
		},
		PersonalityStruggling: {
			Personality: PersonalityStruggling,
			Traits: Traits{
				Motivation: 5, Conscientiousness: 4, Confidence: 3, Anxiety: 8,
			},
			TechSkills: TechSkills{
				Linux: 3, Networking: 2, Security: 2, Scripting: 2, CloudOps: 1,
			},
			BehavioralConfig: BehavioralConfig{
				LLMTemperature:    0.8,
				QuizScoreMin:      55,
				QuizScoreMax:      70,
				AssignmentQuality: "incomplete",
				DiscussionStyle:   "vague",
				SubmitTiming:      "last_minute",
				ErrorRate:         0.18,
			},
		},
		PersonalityIndustryProfessional: {
			Personality: PersonalityIndustryProfessional,
			Traits: Traits{
				Motivation: 7, Conscientiousness: 8, Confidence: 7, Anxiety: 4,
			},
			TechSkills: TechSkills{
				Linux: 6, Networking: 7, Security: 5, Scripting: 5, CloudOps: 7,
			},
			BehavioralConfig: BehavioralConfig{
				LLMTemperature:    0.5,
				QuizScoreMin:      75,
				QuizScoreMax:      85,
				AssignmentQuality: "practical",
				DiscussionStyle:   "practical",
				SubmitTiming:      "on_time",
				ErrorRate:         0.08,
			},
		},
	}
}

// ValidPersonalities returns all valid personality type values.
func ValidPersonalities() []PersonalityType {
	return []PersonalityType{
		PersonalityHighPerformer,
		PersonalityStruggling,
		PersonalityIndustryProfessional,
	}
}
