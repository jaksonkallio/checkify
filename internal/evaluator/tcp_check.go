package evaluator

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"
)

const (
	DefaultTCPTimeout = 5 * time.Second
	MaxTCPTimeout     = 30 * time.Minute
)

type Dialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

type DefaultDialer struct {
	Timeout time.Duration
}

func (d *DefaultDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	dialer := &net.Dialer{
		Timeout: d.Timeout,
	}
	return dialer.DialContext(ctx, network, address)
}

type TCPCheckEvaluator struct {
	Host    string
	Port    int
	Timeout time.Duration
	IPStack IPStack
	dialer  Dialer
}

func NewTCPCheckEvaluator(
	host string,
	port int,
	timeout time.Duration,
	ipStack IPStack,
) (*TCPCheckEvaluator, error) {
	if len(host) == 0 {
		return nil, fmt.Errorf("host cannot be empty")
	}

	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("port must be between 1 and 65535")
	}

	if timeout <= 0 || timeout > MaxTCPTimeout {
		return nil, fmt.Errorf("timeout must be greater than 0 and less than %s", MaxTCPTimeout)
	}

	if ipStack == "" {
		ipStack = IPStackDefault
	}
	switch ipStack {
	case IPStackIPv4, IPStackIPv6, IPStackAny:
	default:
		return nil, fmt.Errorf("ipstack must be one of: ipv4, ipv6, any")
	}

	evaluator := &TCPCheckEvaluator{
		Host:    host,
		Port:    port,
		Timeout: timeout,
		IPStack: ipStack,
		dialer: &DefaultDialer{
			Timeout: timeout,
		},
	}

	return evaluator, nil
}

func NewTCPCheck(name string, host string, port int, interval time.Duration, timeout time.Duration, ipStack IPStack, environment string, tags []string) (Check, error) {
	evaluator, err := NewTCPCheckEvaluator(host, port, timeout, ipStack)
	if err != nil {
		return Check{}, fmt.Errorf("create TCP check evaluator: %w", err)
	}

	return NewCheck(name, environment, interval, tags, evaluator)
}

func (c *TCPCheckEvaluator) Evaluate(ctx context.Context) CheckResult {
	if c.Timeout <= 0 {
		// Negative or zero timeout means guaranteed to timeout, no need to send request.
		return CheckResult{
			Err:     NewErrCheckTimeout(c.Timeout),
			Latency: 0,
		}
	}

	var network string
	switch c.IPStack {
	case IPStackIPv4:
		network = "tcp4"
	case IPStackIPv6:
		network = "tcp6"
	default:
		network = "tcp"
	}

	address := fmt.Sprintf("%s:%d", c.Host, c.Port)

	// The network shows which stack was actually dialed, which matters when the host has both A and AAAA records.
	logAttrs := []slog.Attr{
		slog.String("tcpNetwork", network),
	}

	start := time.Now()
	conn, err := c.dialer.DialContext(ctx, network, address)
	latency := time.Since(start)
	if err != nil {
		return CheckResult{
			Pass:     false,
			Latency:  latency,
			Err:      err,
			LogAttrs: logAttrs,
		}
	}
	defer func() { _ = conn.Close() }()

	// The remote address identifies which host the name resolved to, so a single bad backend is identifiable.
	if remoteAddr := conn.RemoteAddr(); remoteAddr != nil {
		logAttrs = append(logAttrs, slog.String("tcpRemoteAddr", remoteAddr.String()))
	}

	return CheckResult{
		Pass:     true,
		Latency:  latency,
		Err:      nil,
		LogAttrs: logAttrs,
	}
}
