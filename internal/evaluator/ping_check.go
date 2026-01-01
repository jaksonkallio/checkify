package evaluator

import (
	"context"
	"fmt"
	"time"

	"github.com/google/cel-go/cel"
	probing "github.com/prometheus-community/pro-bing"
)

const (
	IPStackIPv4    IPStack = "ipv4"
	IPStackIPv6    IPStack = "ipv6"
	IPStackAny     IPStack = "any"
	IPStackDefault IPStack = IPStackAny

	MaxPingTimeout = 1 * time.Minute
	MaxPingCount   = 100
)

type IPStack string

// PingStatistics contains the results of a ping operation
type PingStatistics struct {
	PacketsSent           int
	PacketsRecv           int
	PacketsRecvDuplicates int
	PacketLoss            float64
	AvgRtt                time.Duration
	MinRtt                time.Duration
	MaxRtt                time.Duration
}

// Pinger interface allows mocking ping operations for testing
type Pinger interface {
	Run(ctx context.Context) (PingStatistics, error)
}

// DefaultPinger wraps pro-bing to implement the Pinger interface
type DefaultPinger struct {
	Host    string
	Count   int
	Timeout time.Duration
	IPStack IPStack
}

// Run executes the ping and returns statistics
func (p *DefaultPinger) Run(ctx context.Context) (PingStatistics, error) {
	pinger, err := probing.NewPinger(p.Host)
	if err != nil {
		return PingStatistics{}, fmt.Errorf("create pinger: %w", err)
	}

	pinger.Count = p.Count
	pinger.Timeout = p.Timeout
	pinger.SetPrivileged(false)

	switch p.IPStack {
	case IPStackIPv4:
		pinger.SetNetwork("ip4")
	case IPStackIPv6:
		pinger.SetNetwork("ip6")
	default:
		pinger.SetNetwork("ip")
	}

	if err := pinger.RunWithContext(ctx); err != nil {
		return PingStatistics{}, fmt.Errorf("run ping with context: %w", err)
	}

	stats := pinger.Statistics()
	return PingStatistics{
		PacketsSent:           stats.PacketsSent,
		PacketsRecv:           stats.PacketsRecv,
		PacketsRecvDuplicates: stats.PacketsRecvDuplicates,
		PacketLoss:            stats.PacketLoss,
		AvgRtt:                stats.AvgRtt,
		MinRtt:                stats.MinRtt,
		MaxRtt:                stats.MaxRtt,
	}, nil
}

type PingCheckEvaluator struct {
	Host             string
	Count            int
	Timeout          time.Duration
	IPStack          IPStack
	assertionProgram cel.Program
	pinger           Pinger
}

func NewPingCheckEvaluator(
	host string,
	count int,
	timeout time.Duration,
	ipStack IPStack,
	assertion string,
) (*PingCheckEvaluator, error) {
	if len(host) == 0 {
		return nil, fmt.Errorf("host cannot be empty")
	}

	if count <= 0 || count > MaxPingCount {
		return nil, fmt.Errorf("count must be greater than 0 and less than %d", MaxPingCount)
	}

	if timeout <= 0 || timeout > MaxPingTimeout {
		return nil, fmt.Errorf("timeout must be greater than 0 and less than %s", MaxPingTimeout)
	}

	if ipStack == "" {
		ipStack = IPStackDefault
	}
	switch ipStack {
	case IPStackIPv4, IPStackIPv6, IPStackAny:
	default:
		return nil, fmt.Errorf("unsupported IP stack %q", ipStack)
	}

	// Compile CEL assertion program if provided
	var assertionProgram cel.Program
	if assertion != "" {
		env, err := cel.NewEnv(
			cel.Variable("avg_rtt_ms", cel.IntType),
			cel.Variable("max_rtt_ms", cel.IntType),
			cel.Variable("min_rtt_ms", cel.IntType),
			cel.Variable("packet_loss_pct", cel.DoubleType),
			cel.Variable("packets_received", cel.IntType),
			cel.Variable("packets_received_dup", cel.IntType),
			cel.Variable("packets_sent", cel.IntType),
		)
		if err != nil {
			return nil, fmt.Errorf("create CEL environment: %w", err)
		}

		ast, issues := env.Compile(assertion)
		if issues != nil && issues.Err() != nil {
			return nil, fmt.Errorf("compile CEL expression: %w", issues.Err())
		}

		prg, err := env.Program(ast)
		if err != nil {
			return nil, fmt.Errorf("create CEL program: %w", err)
		}

		assertionProgram = prg
	}

	evaluator := &PingCheckEvaluator{
		Host:             host,
		Count:            count,
		Timeout:          timeout,
		IPStack:          ipStack,
		assertionProgram: assertionProgram,
		pinger: &DefaultPinger{
			Host:    host,
			Count:   count,
			Timeout: timeout,
			IPStack: ipStack,
		},
	}

	return evaluator, nil
}

// NewPingCheck is a convenience constructor that creates both the evaluator and the Check
func NewPingCheck(name string, host string, interval time.Duration, count int, timeout time.Duration, environment string, tags []string, ipStack IPStack, assertion string) (Check, error) {
	evaluator, err := NewPingCheckEvaluator(host, count, timeout, ipStack, assertion)
	if err != nil {
		return Check{}, fmt.Errorf("create ping check evaluator: %w", err)
	}

	return NewCheck(name, environment, interval, tags, evaluator), nil
}

// Evaluate performs an ICMP ping to the configured host and returns the result.
func (c *PingCheckEvaluator) Evaluate(ctx context.Context) CheckResult {
	if c.Timeout <= 0 {
		// Negative or zero timeout means guaranteed to timeout, no need to send request.
		return CheckResult{
			Err:     NewErrCheckTimeout(c.Timeout),
			Latency: 0,
		}
	}

	stats, err := c.pinger.Run(ctx)
	if err != nil {
		return CheckResult{
			Err:     err,
			Latency: 0,
		}
	}

	// Use average RTT as the latency metric.
	latency := stats.AvgRtt

	var pass bool
	if c.assertionProgram != nil {
		assertionPasses, err := c.evaluateAssertion(stats)
		if err != nil {
			return CheckResult{
				Err:     fmt.Errorf("evaluate assertion: %w", err),
				Latency: latency,
			}
		}
		pass = assertionPasses
	} else {
		// No assertion means pass if the pinger was able to successfully run at all.
		pass = true
	}

	return CheckResult{
		Pass:    pass,
		Latency: latency,
		Err:     nil,
	}
}

// evaluateAssertion evaluates the CEL assertion against the ping statistics using the pre-compiled program.
func (c *PingCheckEvaluator) evaluateAssertion(stats PingStatistics) (bool, error) {
	out, _, err := c.assertionProgram.Eval(map[string]any{
		"avg_rtt_ms":           int(stats.AvgRtt.Milliseconds()),
		"max_rtt_ms":           int(stats.MaxRtt.Milliseconds()),
		"min_rtt_ms":           int(stats.MinRtt.Milliseconds()),
		"packet_loss_pct":      stats.PacketLoss,
		"packets_received":     stats.PacketsRecv,
		"packets_received_dup": stats.PacketsRecvDuplicates,
		"packets_sent":         stats.PacketsSent,
	})
	if err != nil {
		return false, fmt.Errorf("evaluate CEL expression: %w", err)
	}

	result, ok := out.Value().(bool)
	if !ok {
		return false, fmt.Errorf("CEL expression did not return a boolean value")
	}

	return result, nil
}
