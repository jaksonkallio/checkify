package evaluator

import (
	"math/rand"
	"time"
)

// Clock interface allows mocking time-related operations for testing
type Clock interface {
	Now() time.Time
	NewTimer(d time.Duration) *time.Timer
	Jitter(max time.Duration) time.Duration
}

// RealClock provides access to real time operations
type RealClock struct{}

// Now returns the current time
func (rc RealClock) Now() time.Time {
	return time.Now()
}

// NewTimer creates a new timer with the given duration
func (rc RealClock) NewTimer(d time.Duration) *time.Timer {
	return time.NewTimer(d)
}

// Jitter returns a random duration between 0 and max
func (rc RealClock) Jitter(max time.Duration) time.Duration {
	if max <= 0 {
		return 0
	}
	return time.Duration(rand.Int63n(int64(max)))
}
