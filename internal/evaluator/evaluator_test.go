package evaluator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEvaluator(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		httpChecks := []HTTPCheck{
			{Name: "check1", Environment: "prod"},
			{Name: "check2", Environment: "prod"},
		}
		pingChecks := []PingCheck{
			{Name: "ping1", Environment: "prod"},
		}

		evaluator, err := NewEvaluator(httpChecks, pingChecks, RealClock{})

		require.NoError(t, err)
		assert.NotNil(t, evaluator)
		assert.Len(t, evaluator.httpChecks, 2)
		assert.Len(t, evaluator.pingChecks, 1)
	})

	t.Run("ValidatesHTTPCheckSet", func(t *testing.T) {
		httpChecks := []HTTPCheck{
			{Name: "duplicate", Environment: "prod"},
			{Name: "duplicate", Environment: "prod"}, // Duplicate
		}

		_, err := NewEvaluator(httpChecks, []PingCheck{}, RealClock{})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "validate HTTP check set")
	})

	t.Run("ValidatesPingCheckSet", func(t *testing.T) {
		pingChecks := []PingCheck{
			{Name: "duplicate", Environment: "prod"},
			{Name: "duplicate", Environment: "prod"}, // Duplicate
		}

		_, err := NewEvaluator([]HTTPCheck{}, pingChecks, RealClock{})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "validate Ping check set")
	})

	t.Run("DuplicateCheckError", func(t *testing.T) {
		httpChecks := []HTTPCheck{
			{Name: "api-check", Environment: "prod"},
			{Name: "api-check", Environment: "prod"}, // Exact duplicate
		}

		_, err := NewEvaluator(httpChecks, []PingCheck{}, RealClock{})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate check key")
	})

	t.Run("EmptyCheckSet", func(t *testing.T) {
		httpChecks := []HTTPCheck{}
		pingChecks := []PingCheck{}

		evaluator, err := NewEvaluator(httpChecks, pingChecks, RealClock{})

		require.NoError(t, err)
		assert.NotNil(t, evaluator)
		assert.Len(t, evaluator.httpChecks, 0)
		assert.Len(t, evaluator.pingChecks, 0)
	})
}

func TestEvaluator_EvaluateChecksByName(t *testing.T) {
	t.Run("SingleHTTPCheck", func(t *testing.T) {
		// Create a simple check with no HTTP client (won't actually execute)
		check := HTTPCheck{
			Name:        "test-check",
			Environment: "test",
			Timeout:     -1 * time.Second, // Negative timeout = immediate error
		}

		evaluator, err := NewEvaluator([]HTTPCheck{check}, []PingCheck{}, RealClock{})
		require.NoError(t, err)

		// This should not panic, it will just execute the check
		evaluator.EvaluateChecksByName(context.Background(), "test-check")

		// Test completes successfully if no panic
	})

	t.Run("SinglePingCheck", func(t *testing.T) {
		check := PingCheck{
			Name:        "test-ping",
			Environment: "test",
			Timeout:     -1 * time.Second, // Negative timeout = immediate error
		}

		evaluator, err := NewEvaluator([]HTTPCheck{}, []PingCheck{check}, RealClock{})
		require.NoError(t, err)

		// This should not panic, it will just execute the check
		evaluator.EvaluateChecksByName(context.Background(), "test-ping")

		// Test completes successfully if no panic
	})

	t.Run("MultipleChecks", func(t *testing.T) {
		httpChecks := []HTTPCheck{
			{Name: "check1", Environment: "prod", Timeout: -1 * time.Second},
			{Name: "check1", Environment: "stage", Timeout: -1 * time.Second},
			{Name: "check2", Environment: "prod", Timeout: -1 * time.Second},
		}
		pingChecks := []PingCheck{
			{Name: "check1", Environment: "prod", Timeout: -1 * time.Second},
		}

		evaluator, err := NewEvaluator(httpChecks, pingChecks, RealClock{})
		require.NoError(t, err)

		// Should evaluate all checks with name "check1" (both HTTP and Ping)
		evaluator.EvaluateChecksByName(context.Background(), "check1")

		// Test completes successfully if no panic
	})

	t.Run("CheckNotFound", func(t *testing.T) {
		httpChecks := []HTTPCheck{
			{Name: "existing-check", Environment: "prod", Timeout: -1 * time.Second},
		}

		evaluator, err := NewEvaluator(httpChecks, []PingCheck{}, RealClock{})
		require.NoError(t, err)

		// Should not panic even if check doesn't exist
		evaluator.EvaluateChecksByName(context.Background(), "nonexistent")

		// Test completes successfully if no panic
	})
}

// MockClock for testing scheduling behavior
type MockClock struct {
	JitterDuration time.Duration
}

func (mc MockClock) Now() time.Time {
	return time.Now()
}

func (mc MockClock) NewTimer(d time.Duration) *time.Timer {
	return time.NewTimer(d)
}

func (mc MockClock) Jitter(max time.Duration) time.Duration {
	return mc.JitterDuration
}

func TestRealClock(t *testing.T) {
	t.Run("Jitter", func(t *testing.T) {
		t.Run("ZeroMax", func(t *testing.T) {
			clock := RealClock{}

			result := clock.Jitter(0)

			assert.Equal(t, time.Duration(0), result)
		})

		t.Run("NegativeMax", func(t *testing.T) {
			clock := RealClock{}

			result := clock.Jitter(-10 * time.Second)

			assert.Equal(t, time.Duration(0), result)
		})

		t.Run("PositiveMax", func(t *testing.T) {
			clock := RealClock{}

			result := clock.Jitter(10 * time.Second)

			// Should be between 0 and max
			assert.GreaterOrEqual(t, result, time.Duration(0))
			assert.LessOrEqual(t, result, 10*time.Second)
		})
	})

	t.Run("Now", func(t *testing.T) {
		clock := RealClock{}

		before := time.Now()
		result := clock.Now()
		after := time.Now()

		assert.True(t, result.After(before) || result.Equal(before))
		assert.True(t, result.Before(after) || result.Equal(after))
	})

	t.Run("NewTimer", func(t *testing.T) {
		clock := RealClock{}

		timer := clock.NewTimer(10 * time.Millisecond)

		assert.NotNil(t, timer)
		timer.Stop() // Clean up
	})
}
