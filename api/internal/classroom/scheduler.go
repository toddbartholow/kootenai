package classroom

import (
	"math/rand"
	"time"
)

// Scheduler computes per-student activity timing based on personality.
type Scheduler struct{}

// NewScheduler creates a new scheduler.
func NewScheduler() *Scheduler {
	return &Scheduler{}
}

// ActivityDelay returns the simulated delay before a student starts an activity.
// High performers start early, struggling students delay until last minute.
// The speedMultiplier compresses all timings (1.0 = real-time, 10.0 = 10x faster).
// #nosec G404 -- math/rand is acceptable for simulation timing jitter.
func (s *Scheduler) ActivityDelay(profile PersonalityProfile, state BehaviorState, speedMultiplier float64) time.Duration {
	if speedMultiplier <= 0 {
		speedMultiplier = 1.0
	}

	var baseDelay time.Duration

	switch profile.BehavioralConfig.SubmitTiming {
	case "early":
		// 1-4 seconds (simulating 1-2 days early)
		baseDelay = time.Duration(1000+rand.Intn(3000)) * time.Millisecond
	case "on_time":
		// 3-8 seconds (simulating on-time submission)
		baseDelay = time.Duration(3000+rand.Intn(5000)) * time.Millisecond
	case "last_minute":
		// 6-15 seconds (simulating last-minute rush)
		baseDelay = time.Duration(6000+rand.Intn(9000)) * time.Millisecond
	default:
		baseDelay = time.Duration(2000+rand.Intn(4000)) * time.Millisecond
	}

	// Low energy increases delays
	if state.Energy < 0.4 {
		baseDelay = time.Duration(float64(baseDelay) * 1.5)
	}

	// High stress can cause erratic timing (either rush or procrastinate)
	if state.Stress > 0.7 && profile.Traits.Anxiety >= 7 {
		// Anxious + stressed = procrastinate more
		baseDelay = time.Duration(float64(baseDelay) * 1.3)
	}

	// Apply speed multiplier
	return time.Duration(float64(baseDelay) / speedMultiplier)
}

// InterActivityDelay returns the delay between activities within a single module.
// #nosec G404 -- math/rand is acceptable for simulation timing jitter.
func (s *Scheduler) InterActivityDelay(speedMultiplier float64) time.Duration {
	if speedMultiplier <= 0 {
		speedMultiplier = 1.0
	}
	base := time.Duration(500+rand.Intn(2000)) * time.Millisecond
	return time.Duration(float64(base) / speedMultiplier)
}
