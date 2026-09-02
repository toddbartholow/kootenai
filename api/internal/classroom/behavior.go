package classroom

import (
	"encoding/json"
	"math"
	"math/rand"
)

// BehaviorState tracks per-student runtime state during a simulation.
type BehaviorState struct {
	Energy float64 `json:"energy"` // 0.0–1.0, decreases per activity, resets daily
	Stress float64 `json:"stress"` // 0.0–1.0, increases near deadlines
}

// BehaviorEngine computes quality modifiers and timing decisions based on
// student personality and runtime state.
type BehaviorEngine struct{}

// NewBehaviorEngine creates a new behavior engine.
func NewBehaviorEngine() *BehaviorEngine {
	return &BehaviorEngine{}
}

// InitialState returns the starting state for a student.
func (b *BehaviorEngine) InitialState(personality PersonalityType) BehaviorState {
	switch personality {
	case PersonalityHighPerformer:
		return BehaviorState{Energy: 1.0, Stress: 0.1}
	case PersonalityStruggling:
		return BehaviorState{Energy: 0.7, Stress: 0.4}
	case PersonalityIndustryProfessional:
		return BehaviorState{Energy: 0.9, Stress: 0.2}
	default:
		return BehaviorState{Energy: 0.8, Stress: 0.3}
	}
}

// QualityModifier computes a quality multiplier (0.0–1.0) based on traits and state.
// Formula: base_quality * (1 - stress*0.2) * energy
func (b *BehaviorEngine) QualityModifier(traits Traits, state BehaviorState) float64 {
	baseQuality := float64(traits.Conscientiousness) / 10.0
	modifier := baseQuality * (1 - state.Stress*0.2) * state.Energy
	return clamp(modifier, 0.1, 1.0)
}

// ShouldSkipActivity returns true if the student should skip an activity due to low energy.
// Skip chance = 30% when energy < 0.3 and conscientiousness < 5.
// #nosec G404 -- math/rand is acceptable for simulation randomness.
func (b *BehaviorEngine) ShouldSkipActivity(traits Traits, state BehaviorState) bool {
	if state.Energy >= 0.3 {
		return false
	}
	if traits.Conscientiousness >= 5 {
		return false
	}
	return rand.Float64() < 0.30
}

// UpdateAfterActivity returns an updated state after completing an activity.
func (b *BehaviorEngine) UpdateAfterActivity(state BehaviorState, traits Traits) BehaviorState {
	// Energy decreases per activity, less for high-motivation students
	energyCost := 0.08
	if traits.Motivation >= 7 {
		energyCost = 0.05
	}
	state.Energy = clamp(state.Energy-energyCost, 0.0, 1.0)

	// Stress decreases after completing something
	stressRelief := 0.05
	if traits.Anxiety >= 7 {
		stressRelief = 0.02 // anxious students don't relax as much
	}
	state.Stress = clamp(state.Stress-stressRelief, 0.0, 1.0)

	return state
}

// IncreaseStress raises stress when a deadline approaches.
func (b *BehaviorEngine) IncreaseStress(state BehaviorState, traits Traits, deadlineProximity float64) BehaviorState {
	stressIncrease := deadlineProximity * 0.3
	if traits.Anxiety >= 7 {
		stressIncrease *= 1.5
	}
	state.Stress = clamp(state.Stress+stressIncrease, 0.0, 1.0)
	return state
}

// ResetDaily resets energy for a new simulated day.
func (b *BehaviorEngine) ResetDaily(state BehaviorState) BehaviorState {
	state.Energy = clamp(state.Energy+0.6, 0.0, 1.0)
	return state
}

// ScoreForQuiz returns a simulated quiz score based on behavioral config and quality modifier.
// #nosec G404 -- math/rand is acceptable for simulation randomness.
func (b *BehaviorEngine) ScoreForQuiz(cfg BehavioralConfig, quality float64) int {
	baseMin := float64(cfg.QuizScoreMin)
	baseMax := float64(cfg.QuizScoreMax)
	score := baseMin + (baseMax-baseMin)*quality
	// Add some noise
	noise := (rand.Float64() - 0.5) * 10
	return int(clamp(score+noise, 0, 100))
}

// StateToJSON marshals the behavior state.
func StateToJSON(state BehaviorState) json.RawMessage {
	data, _ := json.Marshal(state)
	return data
}

// StateFromJSON unmarshals behavior state from JSON.
func StateFromJSON(raw json.RawMessage) BehaviorState {
	var state BehaviorState
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &state)
	}
	return state
}

func clamp(v, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, v))
}
