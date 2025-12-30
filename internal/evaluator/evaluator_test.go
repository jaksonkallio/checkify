package evaluator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEvaluator_Success(t *testing.T) {
	checks := []Check{
		{Name: "check1", Environment: "prod"},
		{Name: "check2", Environment: "prod"},
	}

	evaluator, err := NewEvaluator(checks, RealClock{})

	require.NoError(t, err)
	assert.NotNil(t, evaluator)
	assert.Len(t, evaluator.checks, 2)
}

func TestNewEvaluator_ValidatesCheckSet(t *testing.T) {
	checks := []Check{
		{Name: "duplicate", Environment: "prod"},
		{Name: "duplicate", Environment: "prod"}, // Duplicate
	}

	_, err := NewEvaluator(checks, RealClock{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validate check set")
}

func TestNewEvaluator_DuplicateCheckError(t *testing.T) {
	checks := []Check{
		{Name: "api-check", Environment: "prod"},
		{Name: "api-check", Environment: "prod"}, // Exact duplicate
	}

	_, err := NewEvaluator(checks, RealClock{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate check key")
}

func TestNewEvaluator_EmptyCheckSet(t *testing.T) {
	checks := []Check{}

	evaluator, err := NewEvaluator(checks, RealClock{})

	require.NoError(t, err)
	assert.NotNil(t, evaluator)
	assert.Len(t, evaluator.checks, 0)
}

func TestEvaluator_EvaluateChecksByName_SingleCheck(t *testing.T) {
	// Create a simple check with no HTTP client (won't actually execute)
	check := Check{
		Name:        "test-check",
		Environment: "test",
		Timeout:     -1 * time.Second, // Negative timeout = immediate error
	}

	evaluator, err := NewEvaluator([]Check{check}, RealClock{})
	require.NoError(t, err)

	// This should not panic, it will just execute the check
	evaluator.EvaluateChecksByName(context.Background(), "test-check")

	// Test completes successfully if no panic
}

func TestEvaluator_EvaluateChecksByName_MultipleChecks(t *testing.T) {
	checks := []Check{
		{Name: "check1", Environment: "prod", Timeout: -1 * time.Second},
		{Name: "check1", Environment: "stage", Timeout: -1 * time.Second},
		{Name: "check2", Environment: "prod", Timeout: -1 * time.Second},
	}

	evaluator, err := NewEvaluator(checks, RealClock{})
	require.NoError(t, err)

	// Should evaluate both checks with name "check1"
	evaluator.EvaluateChecksByName(context.Background(), "check1")

	// Test completes successfully if no panic
}

func TestEvaluator_EvaluateChecksByName_CheckNotFound(t *testing.T) {
	checks := []Check{
		{Name: "existing-check", Environment: "prod", Timeout: -1 * time.Second},
	}

	evaluator, err := NewEvaluator(checks, RealClock{})
	require.NoError(t, err)

	// Should not panic even if check doesn't exist
	evaluator.EvaluateChecksByName(context.Background(), "nonexistent")

	// Test completes successfully if no panic
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

func TestRealClock_Jitter_ZeroMax(t *testing.T) {
	clock := RealClock{}

	result := clock.Jitter(0)

	assert.Equal(t, time.Duration(0), result)
}

func TestRealClock_Jitter_NegativeMax(t *testing.T) {
	clock := RealClock{}

	result := clock.Jitter(-10 * time.Second)

	assert.Equal(t, time.Duration(0), result)
}

func TestRealClock_Jitter_PositiveMax(t *testing.T) {
	clock := RealClock{}

	result := clock.Jitter(10 * time.Second)

	// Should be between 0 and max
	assert.GreaterOrEqual(t, result, time.Duration(0))
	assert.LessOrEqual(t, result, 10*time.Second)
}

func TestRealClock_Now(t *testing.T) {
	clock := RealClock{}

	before := time.Now()
	result := clock.Now()
	after := time.Now()

	assert.True(t, result.After(before) || result.Equal(before))
	assert.True(t, result.Before(after) || result.Equal(after))
}

func TestRealClock_NewTimer(t *testing.T) {
	clock := RealClock{}

	timer := clock.NewTimer(10 * time.Millisecond)

	assert.NotNil(t, timer)
	timer.Stop() // Clean up
}
