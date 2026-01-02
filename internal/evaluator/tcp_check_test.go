package evaluator

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockDialer implements the Dialer interface for testing
type MockDialer struct {
	DialContextFunc func(ctx context.Context, network, address string) (net.Conn, error)
}

func (m *MockDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if m.DialContextFunc != nil {
		return m.DialContextFunc(ctx, network, address)
	}
	// Default successful connection (return nil for simplicity in tests)
	return &mockConn{}, nil
}

// mockConn is a simple mock implementation of net.Conn for testing
type mockConn struct{}

func (m *mockConn) Read(b []byte) (n int, err error)   { return 0, nil }
func (m *mockConn) Write(b []byte) (n int, err error)  { return 0, nil }
func (m *mockConn) Close() error                       { return nil }
func (m *mockConn) LocalAddr() net.Addr                { return nil }
func (m *mockConn) RemoteAddr() net.Addr               { return nil }
func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }

func TestNewTCPCheck(t *testing.T) {
	check, err := NewTCPCheck(
		"test-tcp",
		"example.com",
		443,
		10*time.Second,
		5*time.Second,
		IPStackAny,
		"production",
		[]string{"critical", "web"},
	)

	assert.NoError(t, err)
	assert.NotNil(t, check)
	assert.Equal(t, "test-tcp", check.Name)
	assert.Equal(t, 10*time.Second, check.Interval)
	assert.Equal(t, "production", check.Environment)
	assert.Equal(t, []string{"critical", "web"}, check.Tags)

	tcpEval, ok := check.Evaluator.(*TCPCheckEvaluator)
	require.True(t, ok, "Evaluator should be TCPCheckEvaluator")
	assert.Equal(t, "example.com", tcpEval.Host)
	assert.Equal(t, 443, tcpEval.Port)
	assert.Equal(t, 5*time.Second, tcpEval.Timeout)
	assert.Equal(t, IPStackAny, tcpEval.IPStack)
}

func TestNewTCPCheck_InvalidIPStack(t *testing.T) {
	_, err := NewTCPCheck(
		"test-tcp",
		"example.com",
		80,
		10*time.Second,
		5*time.Second,
		"invalid",
		"production",
		[]string{},
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ipstack must be one of")
}

func TestNewTCPCheckEvaluator_InvalidHost(t *testing.T) {
	_, err := NewTCPCheckEvaluator(
		"",
		80,
		5*time.Second,
		IPStackAny,
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "host cannot be empty")
}

func TestNewTCPCheckEvaluator_InvalidPort(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{"zero port", 0},
		{"negative port", -1},
		{"port too high", 65536},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewTCPCheckEvaluator(
				"example.com",
				tt.port,
				5*time.Second,
				IPStackAny,
			)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), "port must be between 1 and 65535")
		})
	}
}

func TestNewTCPCheckEvaluator_InvalidTimeout(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
	}{
		{"zero timeout", 0},
		{"negative timeout", -1 * time.Second},
		{"timeout too large", 31 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewTCPCheckEvaluator(
				"example.com",
				80,
				tt.timeout,
				IPStackAny,
			)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), "timeout must be greater than 0")
		})
	}
}

func TestTCPCheck_EvaluateSuccess(t *testing.T) {
	check, err := NewTCPCheck(
		"test-tcp",
		"example.com",
		443,
		10*time.Second,
		5*time.Second,
		IPStackAny,
		"production",
		[]string{"critical"},
	)
	assert.NoError(t, err)

	// Use mock dialer
	check.Evaluator.(*TCPCheckEvaluator).dialer = &MockDialer{
		DialContextFunc: func(ctx context.Context, network, address string) (net.Conn, error) {
			assert.Equal(t, "tcp", network)
			assert.Equal(t, "example.com:443", address)
			return &mockConn{}, nil
		},
	}

	ctx := context.Background()
	result := check.Evaluator.Evaluate(ctx)

	assert.NoError(t, result.Err)
	assert.True(t, result.Pass)
	assert.Greater(t, result.Latency, time.Duration(0))
}

func TestTCPCheck_EvaluateConnectionRefused(t *testing.T) {
	check, err := NewTCPCheck(
		"test-tcp",
		"example.com",
		80,
		10*time.Second,
		5*time.Second,
		IPStackAny,
		"production",
		[]string{},
	)
	assert.NoError(t, err)

	// Mock dialer that returns connection refused error
	check.Evaluator.(*TCPCheckEvaluator).dialer = &MockDialer{
		DialContextFunc: func(ctx context.Context, network, address string) (net.Conn, error) {
			return nil, errors.New("connection refused")
		},
	}

	ctx := context.Background()
	result := check.Evaluator.Evaluate(ctx)

	assert.Error(t, result.Err)
	assert.False(t, result.Pass)
	assert.Contains(t, result.Err.Error(), "connection refused")
}

func TestTCPCheck_EvaluateTimeout(t *testing.T) {
	// NewTCPCheckEvaluator should reject zero/negative timeout
	_, err := NewTCPCheckEvaluator(
		"example.com",
		80,
		0, // Zero timeout
		IPStackAny,
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout must be greater than 0")
}

func TestTCPCheck_EvaluateContextCancellation(t *testing.T) {
	check, err := NewTCPCheck(
		"test-tcp-cancel",
		"example.com",
		80,
		10*time.Second,
		10*time.Second,
		IPStackAny,
		"production",
		[]string{},
	)
	assert.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	// Mock dialer that respects context cancellation
	check.Evaluator.(*TCPCheckEvaluator).dialer = &MockDialer{
		DialContextFunc: func(ctx context.Context, network, address string) (net.Conn, error) {
			return nil, ctx.Err()
		},
	}

	cancel() // Cancel immediately

	result := check.Evaluator.Evaluate(ctx)

	assert.Error(t, result.Err)
	assert.False(t, result.Pass)
}

func TestTCPCheck_NetworkTypeIPv4(t *testing.T) {
	check, err := NewTCPCheck(
		"test-tcp",
		"example.com",
		80,
		10*time.Second,
		5*time.Second,
		IPStackIPv4,
		"production",
		[]string{},
	)
	assert.NoError(t, err)

	// Mock dialer to verify network type
	check.Evaluator.(*TCPCheckEvaluator).dialer = &MockDialer{
		DialContextFunc: func(ctx context.Context, network, address string) (net.Conn, error) {
			assert.Equal(t, "tcp4", network, "Should use tcp4 for IPv4")
			return &mockConn{}, nil
		},
	}

	ctx := context.Background()
	result := check.Evaluator.Evaluate(ctx)

	assert.NoError(t, result.Err)
	assert.True(t, result.Pass)
}

func TestTCPCheck_NetworkTypeIPv6(t *testing.T) {
	check, err := NewTCPCheck(
		"test-tcp",
		"example.com",
		80,
		10*time.Second,
		5*time.Second,
		IPStackIPv6,
		"production",
		[]string{},
	)
	assert.NoError(t, err)

	// Mock dialer to verify network type
	check.Evaluator.(*TCPCheckEvaluator).dialer = &MockDialer{
		DialContextFunc: func(ctx context.Context, network, address string) (net.Conn, error) {
			assert.Equal(t, "tcp6", network, "Should use tcp6 for IPv6")
			return &mockConn{}, nil
		},
	}

	ctx := context.Background()
	result := check.Evaluator.Evaluate(ctx)

	assert.NoError(t, result.Err)
	assert.True(t, result.Pass)
}
