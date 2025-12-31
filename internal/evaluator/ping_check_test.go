package evaluator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewPingCheck(t *testing.T) {
	check, err := NewPingCheck(
		"test-ping",
		"8.8.8.8",
		10*time.Second,
		5*time.Second,
		"production",
		[]string{"critical", "dns"},
	)

	assert.NoError(t, err)
	assert.NotNil(t, check)
	assert.Equal(t, "test-ping", check.Name)
	assert.Equal(t, "8.8.8.8", check.Host)
	assert.Equal(t, 10*time.Second, check.Interval)
	assert.Equal(t, 5*time.Second, check.Timeout)
	assert.Equal(t, "production", check.Environment)
	assert.Equal(t, []string{"critical", "dns"}, check.Tags)
}

func TestPingCheck_EvaluateSuccess(t *testing.T) {
	check, err := NewPingCheck(
		"test-ping",
		"8.8.8.8",
		10*time.Second,
		5*time.Second,
		"production",
		[]string{"critical"},
	)
	assert.NoError(t, err)

	ctx := context.Background()
	result := check.Evaluate(ctx)

	assert.NoError(t, result.Err)
	assert.True(t, result.Pass)
	assert.Greater(t, result.Latency, time.Duration(0))
}

func TestPingCheck_EvaluateIPv6(t *testing.T) {
	check, err := NewPingCheck(
		"test-ping-ipv6",
		"2001:4860:4860::8888", // Google DNS IPv6
		10*time.Second,
		5*time.Second,
		"production",
		[]string{"ipv6"},
	)
	assert.NoError(t, err)

	ctx := context.Background()
	result := check.Evaluate(ctx)

	// IPv6 might not be available in all environments, so we just check that it doesn't panic
	assert.NotNil(t, result)
}

func TestPingCheck_EvaluateDomain(t *testing.T) {
	check, err := NewPingCheck(
		"test-ping-domain",
		"google.com",
		10*time.Second,
		5*time.Second,
		"production",
		[]string{"domain"},
	)
	assert.NoError(t, err)

	ctx := context.Background()
	result := check.Evaluate(ctx)

	assert.NoError(t, result.Err)
	assert.True(t, result.Pass)
	assert.Greater(t, result.Latency, time.Duration(0))
}

func TestPingCheck_EvaluateTimeout(t *testing.T) {
	check, err := NewPingCheck(
		"test-ping-timeout",
		"8.8.8.8",
		10*time.Second,
		0,
		"production",
		[]string{},
	)
	assert.NoError(t, err)

	ctx := context.Background()
	result := check.Evaluate(ctx)

	assert.Error(t, result.Err)
	assert.IsType(t, &ErrCheckTimeout{}, result.Err)
}

func TestPingCheck_EvaluateInvalidHost(t *testing.T) {
	check, err := NewPingCheck(
		"test-ping-invalid",
		"invalid-host-that-definitely-does-not-exist-12345.com",
		10*time.Second,
		2*time.Second,
		"production",
		[]string{},
	)
	assert.NoError(t, err)

	ctx := context.Background()
	result := check.Evaluate(ctx)

	// Should either fail to resolve or timeout
	assert.False(t, result.Pass)
}

func TestPingCheck_EvaluateContextCancellation(t *testing.T) {
	check, err := NewPingCheck(
		"test-ping-cancel",
		"8.8.8.8",
		10*time.Second,
		10*time.Second,
		"production",
		[]string{},
	)
	assert.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result := check.Evaluate(ctx)

	assert.Error(t, result.Err)
}

func TestValidatePingCheckSet(t *testing.T) {
	t.Run("valid check set", func(t *testing.T) {
		check1, _ := NewPingCheck("check1", "8.8.8.8", 10*time.Second, 5*time.Second, "production", []string{})
		check2, _ := NewPingCheck("check2", "1.1.1.1", 10*time.Second, 5*time.Second, "production", []string{})
		check3, _ := NewPingCheck("check1", "8.8.8.8", 10*time.Second, 5*time.Second, "staging", []string{})

		checks := []PingCheck{*check1, *check2, *check3}
		err := ValidatePingCheckSet(checks)
		assert.NoError(t, err)
	})

	t.Run("duplicate check", func(t *testing.T) {
		check1, _ := NewPingCheck("check1", "8.8.8.8", 10*time.Second, 5*time.Second, "production", []string{})
		check2, _ := NewPingCheck("check1", "1.1.1.1", 10*time.Second, 5*time.Second, "production", []string{})

		checks := []PingCheck{*check1, *check2}
		err := ValidatePingCheckSet(checks)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate check key")
	})

	t.Run("empty check set", func(t *testing.T) {
		checks := []PingCheck{}
		err := ValidatePingCheckSet(checks)
		assert.NoError(t, err)
	})
}
