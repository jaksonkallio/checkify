package evaluator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// MockPinger implements the Pinger interface for testing
type MockPinger struct {
	RunFunc func(ctx context.Context) (PingStatistics, error)
}

func (m *MockPinger) Run(ctx context.Context) (PingStatistics, error) {
	if m.RunFunc != nil {
		return m.RunFunc(ctx)
	}
	// Default successful ping
	return PingStatistics{
		PacketsSent:           1,
		PacketsRecv:           1,
		PacketsRecvDuplicates: 0,
		PacketLoss:            0,
		AvgRtt:                10 * time.Millisecond,
		MinRtt:                10 * time.Millisecond,
		MaxRtt:                10 * time.Millisecond,
	}, nil
}

func TestNewPingCheck(t *testing.T) {
	check, err := NewPingCheck(
		"test-ping",
		"8.8.8.8",
		10*time.Second,
		1,
		5*time.Second,
		"production",
		[]string{"critical", "dns"},
		IPStackAny,
		"",
	)

	assert.NoError(t, err)
	assert.NotNil(t, check)
	assert.Equal(t, "test-ping", check.Name)
	assert.Equal(t, "8.8.8.8", check.Host)
	assert.Equal(t, 1, check.Count)
	assert.Equal(t, 10*time.Second, check.Interval)
	assert.Equal(t, 5*time.Second, check.Timeout)
	assert.Equal(t, "production", check.Environment)
	assert.Equal(t, []string{"critical", "dns"}, check.Tags)
	assert.Equal(t, IPStackAny, check.IPStack)
}

func TestNewPingCheck_WithAssertion(t *testing.T) {
	check, err := NewPingCheck(
		"test-ping",
		"8.8.8.8",
		10*time.Second,
		1,
		5*time.Second,
		"production",
		[]string{},
		IPStackAny,
		"packet_loss_pct == 0.0 && avg_rtt_ms < 100",
	)

	assert.NoError(t, err)
	assert.NotNil(t, check)
	assert.NotNil(t, check.assertionProgram)
}

func TestNewPingCheck_InvalidAssertion(t *testing.T) {
	_, err := NewPingCheck(
		"test-ping",
		"8.8.8.8",
		10*time.Second,
		1,
		5*time.Second,
		"production",
		[]string{},
		IPStackAny,
		"invalid syntax here!",
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "compile CEL expression")
}

func TestNewPingCheck_InvalidIPStack(t *testing.T) {
	_, err := NewPingCheck(
		"test-ping",
		"8.8.8.8",
		10*time.Second,
		1,
		5*time.Second,
		"production",
		[]string{},
		"invalid",
		"",
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported IP stack")
}

func TestPingCheck_EvaluateSuccess(t *testing.T) {
	check, err := NewPingCheck(
		"test-ping",
		"8.8.8.8",
		10*time.Second,
		1,
		5*time.Second,
		"production",
		[]string{"critical"},
		IPStackAny,
		"",
	)
	assert.NoError(t, err)

	// Use mock pinger
	check.pinger = &MockPinger{
		RunFunc: func(ctx context.Context) (PingStatistics, error) {
			return PingStatistics{
				PacketsSent:           1,
				PacketsRecv:           1,
				PacketsRecvDuplicates: 0,
				PacketLoss:            0,
				AvgRtt:                25 * time.Millisecond,
				MinRtt:                25 * time.Millisecond,
				MaxRtt:                25 * time.Millisecond,
			}, nil
		},
	}

	ctx := context.Background()
	result := check.Evaluate(ctx)

	assert.NoError(t, result.Err)
	assert.True(t, result.Pass)
	assert.Equal(t, 25*time.Millisecond, result.Latency)
}

func TestPingCheck_EvaluatePacketLoss(t *testing.T) {
	check, err := NewPingCheck(
		"test-ping",
		"8.8.8.8",
		10*time.Second,
		3,
		5*time.Second,
		"production",
		[]string{},
		IPStackAny,
		"",
	)
	assert.NoError(t, err)

	// Mock pinger with packet loss
	check.pinger = &MockPinger{
		RunFunc: func(ctx context.Context) (PingStatistics, error) {
			return PingStatistics{
				PacketsSent:           3,
				PacketsRecv:           2,
				PacketsRecvDuplicates: 0,
				PacketLoss:            33.33,
				AvgRtt:                30 * time.Millisecond,
				MinRtt:                25 * time.Millisecond,
				MaxRtt:                35 * time.Millisecond,
			}, nil
		},
	}

	ctx := context.Background()
	result := check.Evaluate(ctx)

	// No assertion, so it should pass as long as ping succeeded
	assert.NoError(t, result.Err)
	assert.True(t, result.Pass)
}

func TestPingCheck_EvaluateWithAssertionPass(t *testing.T) {
	check, err := NewPingCheck(
		"test-ping",
		"8.8.8.8",
		10*time.Second,
		1,
		5*time.Second,
		"production",
		[]string{},
		IPStackAny,
		"packet_loss_pct == 0.0 && avg_rtt_ms < 100",
	)
	assert.NoError(t, err)

	check.pinger = &MockPinger{
		RunFunc: func(ctx context.Context) (PingStatistics, error) {
			return PingStatistics{
				PacketsSent:           1,
				PacketsRecv:           1,
				PacketsRecvDuplicates: 0,
				PacketLoss:            0,
				AvgRtt:                50 * time.Millisecond,
				MinRtt:                50 * time.Millisecond,
				MaxRtt:                50 * time.Millisecond,
			}, nil
		},
	}

	ctx := context.Background()
	result := check.Evaluate(ctx)

	assert.NoError(t, result.Err)
	assert.True(t, result.Pass)
	assert.Equal(t, 50*time.Millisecond, result.Latency)
}

func TestPingCheck_EvaluateWithAssertionFail(t *testing.T) {
	check, err := NewPingCheck(
		"test-ping",
		"8.8.8.8",
		10*time.Second,
		1,
		5*time.Second,
		"production",
		[]string{},
		IPStackAny,
		"avg_rtt_ms < 10", // Will fail because RTT is 50ms
	)
	assert.NoError(t, err)

	check.pinger = &MockPinger{
		RunFunc: func(ctx context.Context) (PingStatistics, error) {
			return PingStatistics{
				PacketsSent:           1,
				PacketsRecv:           1,
				PacketsRecvDuplicates: 0,
				PacketLoss:            0,
				AvgRtt:                50 * time.Millisecond,
				MinRtt:                50 * time.Millisecond,
				MaxRtt:                50 * time.Millisecond,
			}, nil
		},
	}

	ctx := context.Background()
	result := check.Evaluate(ctx)

	assert.NoError(t, result.Err)
	assert.False(t, result.Pass)
	assert.Equal(t, 50*time.Millisecond, result.Latency)
}

func TestPingCheck_EvaluateTimeout(t *testing.T) {
	check, err := NewPingCheck(
		"test-ping-timeout",
		"8.8.8.8",
		10*time.Second,
		1,
		0, // Zero timeout
		"production",
		[]string{},
		IPStackAny,
		"",
	)
	assert.NoError(t, err)

	ctx := context.Background()
	result := check.Evaluate(ctx)

	assert.Error(t, result.Err)
	assert.IsType(t, &ErrCheckTimeout{}, result.Err)
	assert.False(t, result.Pass)
}

func TestPingCheck_EvaluatePingError(t *testing.T) {
	check, err := NewPingCheck(
		"test-ping",
		"invalid-host",
		10*time.Second,
		1,
		2*time.Second,
		"production",
		[]string{},
		IPStackAny,
		"",
	)
	assert.NoError(t, err)

	// Mock pinger that returns an error
	check.pinger = &MockPinger{
		RunFunc: func(ctx context.Context) (PingStatistics, error) {
			return PingStatistics{}, errors.New("lookup failed")
		},
	}

	ctx := context.Background()
	result := check.Evaluate(ctx)

	assert.Error(t, result.Err)
	assert.False(t, result.Pass)
	assert.Contains(t, result.Err.Error(), "lookup failed")
}

func TestPingCheck_EvaluateContextCancellation(t *testing.T) {
	check, err := NewPingCheck(
		"test-ping-cancel",
		"8.8.8.8",
		10*time.Second,
		1,
		10*time.Second,
		"production",
		[]string{},
		IPStackAny,
		"",
	)
	assert.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	// Mock pinger that respects context cancellation
	check.pinger = &MockPinger{
		RunFunc: func(ctx context.Context) (PingStatistics, error) {
			return PingStatistics{}, ctx.Err()
		},
	}

	cancel() // Cancel immediately

	result := check.Evaluate(ctx)

	assert.Error(t, result.Err)
	assert.False(t, result.Pass)
}

func TestPingCheck_EvaluateComplexAssertion(t *testing.T) {
	check, err := NewPingCheck(
		"test-ping",
		"8.8.8.8",
		10*time.Second,
		3,
		5*time.Second,
		"production",
		[]string{},
		IPStackAny,
		"packet_loss_pct < 50.0 && avg_rtt_ms < 100 && packets_received >= 1",
	)
	assert.NoError(t, err)

	check.pinger = &MockPinger{
		RunFunc: func(ctx context.Context) (PingStatistics, error) {
			return PingStatistics{
				PacketsSent:           3,
				PacketsRecv:           2,
				PacketsRecvDuplicates: 0,
				PacketLoss:            33.33,
				AvgRtt:                75 * time.Millisecond,
				MinRtt:                70 * time.Millisecond,
				MaxRtt:                80 * time.Millisecond,
			}, nil
		},
	}

	ctx := context.Background()
	result := check.Evaluate(ctx)

	assert.NoError(t, result.Err)
	assert.True(t, result.Pass)
}

func TestValidatePingCheckSet(t *testing.T) {
	t.Run("valid check set", func(t *testing.T) {
		check1, _ := NewPingCheck("check1", "8.8.8.8", 10*time.Second, 1, 5*time.Second, "production", []string{}, IPStackAny, "")
		check2, _ := NewPingCheck("check2", "1.1.1.1", 10*time.Second, 1, 5*time.Second, "production", []string{}, IPStackAny, "")
		check3, _ := NewPingCheck("check1", "8.8.8.8", 10*time.Second, 1, 5*time.Second, "staging", []string{}, IPStackAny, "")

		checks := []PingCheck{*check1, *check2, *check3}
		err := ValidatePingCheckSet(checks)
		assert.NoError(t, err)
	})

	t.Run("duplicate check", func(t *testing.T) {
		check1, _ := NewPingCheck("check1", "8.8.8.8", 10*time.Second, 1, 5*time.Second, "production", []string{}, IPStackAny, "")
		check2, _ := NewPingCheck("check1", "1.1.1.1", 10*time.Second, 1, 5*time.Second, "production", []string{}, IPStackAny, "")

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

func TestDefaultPinger_IPv4(t *testing.T) {
	// This test requires network access and will actually ping
	t.Skip("Skipping integration test - requires network access")

	pinger := &DefaultPinger{
		Host:    "8.8.8.8",
		Count:   1,
		Timeout: 5 * time.Second,
		IPStack: IPStackIPv4,
	}

	ctx := context.Background()
	stats, err := pinger.Run(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, 1, stats.PacketsSent)
	assert.Greater(t, stats.PacketsRecv, 0)
}

func TestDefaultPinger_IPv6(t *testing.T) {
	// This test requires network access and IPv6 support
	t.Skip("Skipping integration test - requires network access and IPv6")

	pinger := &DefaultPinger{
		Host:    "2001:4860:4860::8888",
		Count:   1,
		Timeout: 5 * time.Second,
		IPStack: IPStackIPv6,
	}

	ctx := context.Background()
	stats, err := pinger.Run(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, stats)
}
