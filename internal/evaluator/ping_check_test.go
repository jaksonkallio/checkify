package evaluator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	assert.Equal(t, 10*time.Second, check.Interval)
	assert.Equal(t, "production", check.Environment)
	assert.Equal(t, []string{"critical", "dns"}, check.Tags)

	pingEval, ok := check.Evaluator.(*PingCheckEvaluator)
	require.True(t, ok, "Evaluator should be PingCheckEvaluator")
	assert.Equal(t, "8.8.8.8", pingEval.Host)
	assert.Equal(t, 1, pingEval.Count)
	assert.Equal(t, 5*time.Second, pingEval.Timeout)
	assert.Equal(t, IPStackAny, pingEval.IPStack)
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
	_, ok := check.Evaluator.(*PingCheckEvaluator)
	require.True(t, ok, "Evaluator should be PingCheckEvaluator")
	// assertionProgram is now an implementation detail, we can't test it directly
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
	check.Evaluator.(*PingCheckEvaluator).pinger = &MockPinger{
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
	result := check.Evaluator.Evaluate(ctx)

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
	check.Evaluator.(*PingCheckEvaluator).pinger = &MockPinger{
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
	result := check.Evaluator.Evaluate(ctx)

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

	check.Evaluator.(*PingCheckEvaluator).pinger = &MockPinger{
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
	result := check.Evaluator.Evaluate(ctx)

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

	check.Evaluator.(*PingCheckEvaluator).pinger = &MockPinger{
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
	result := check.Evaluator.Evaluate(ctx)

	assert.NoError(t, result.Err)
	assert.False(t, result.Pass)
	assert.Equal(t, 50*time.Millisecond, result.Latency)
}

func TestPingCheck_EvaluateTimeout(t *testing.T) {
	// NewPingCheckEvaluator should reject zero/negative timeout
	_, err := NewPingCheckEvaluator(
		"8.8.8.8",
		1,
		0, // Zero timeout
		IPStackAny,
		"",
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout must be greater than 0")
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
	check.Evaluator.(*PingCheckEvaluator).pinger = &MockPinger{
		RunFunc: func(ctx context.Context) (PingStatistics, error) {
			return PingStatistics{}, errors.New("lookup failed")
		},
	}

	ctx := context.Background()
	result := check.Evaluator.Evaluate(ctx)

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
	check.Evaluator.(*PingCheckEvaluator).pinger = &MockPinger{
		RunFunc: func(ctx context.Context) (PingStatistics, error) {
			return PingStatistics{}, ctx.Err()
		},
	}

	cancel() // Cancel immediately

	result := check.Evaluator.Evaluate(ctx)

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

	check.Evaluator.(*PingCheckEvaluator).pinger = &MockPinger{
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
	result := check.Evaluator.Evaluate(ctx)

	assert.NoError(t, result.Err)
	assert.True(t, result.Pass)
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

func TestPingCheck_LogAttrs(t *testing.T) {
	t.Run("PacketStatisticsOnPass", func(t *testing.T) {
		check, err := NewPingCheck("test-ping", "8.8.8.8", 10*time.Second, 4, 5*time.Second, "test", nil, IPStackAny, "")
		require.NoError(t, err)
		check.Evaluator.(*PingCheckEvaluator).pinger = &MockPinger{
			RunFunc: func(ctx context.Context) (PingStatistics, error) {
				return PingStatistics{
					PacketsSent:           4,
					PacketsRecv:           3,
					PacketsRecvDuplicates: 1,
					PacketLoss:            25,
					AvgRtt:                20 * time.Millisecond,
					MinRtt:                10 * time.Millisecond,
					MaxRtt:                30 * time.Millisecond,
				}, nil
			},
		}

		result := check.Evaluator.Evaluate(context.Background())

		require.True(t, result.Pass)
		assert.Equal(t, int64(4), logAttr(t, result, "pingPacketsSent").Int64())
		assert.Equal(t, int64(3), logAttr(t, result, "pingPacketsReceived").Int64())
		assert.Equal(t, int64(1), logAttr(t, result, "pingPacketsReceivedDuplicate").Int64())
		assert.Equal(t, float64(25), logAttr(t, result, "pingPacketLossPct").Float64())
		assert.Equal(t, int64(10), logAttr(t, result, "pingMinRTTMs").Int64())
		assert.Equal(t, int64(30), logAttr(t, result, "pingMaxRTTMs").Int64())
	})

	t.Run("PacketStatisticsOnAssertionNonPass", func(t *testing.T) {
		check, err := NewPingCheck("test-ping", "8.8.8.8", 10*time.Second, 4, 5*time.Second, "test", nil, IPStackAny, "packet_loss_pct == 0.0")
		require.NoError(t, err)
		check.Evaluator.(*PingCheckEvaluator).pinger = &MockPinger{
			RunFunc: func(ctx context.Context) (PingStatistics, error) {
				return PingStatistics{PacketsSent: 4, PacketsRecv: 2, PacketLoss: 50}, nil
			},
		}

		result := check.Evaluator.Evaluate(context.Background())

		require.False(t, result.Pass)
		require.NoError(t, result.Err)
		assert.Equal(t, float64(50), logAttr(t, result, "pingPacketLossPct").Float64())
		assert.Equal(t, int64(2), logAttr(t, result, "pingPacketsReceived").Int64())
	})

	t.Run("NoAttributesWhenPingFails", func(t *testing.T) {
		check, err := NewPingCheck("test-ping", "8.8.8.8", 10*time.Second, 4, 5*time.Second, "test", nil, IPStackAny, "")
		require.NoError(t, err)
		check.Evaluator.(*PingCheckEvaluator).pinger = &MockPinger{
			RunFunc: func(ctx context.Context) (PingStatistics, error) {
				return PingStatistics{}, errors.New("no such host")
			},
		}

		result := check.Evaluator.Evaluate(context.Background())

		require.Error(t, result.Err)
		assert.Empty(t, result.LogAttrs)
	})
}
