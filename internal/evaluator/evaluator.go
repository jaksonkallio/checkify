// Package evaluator will execute checks and evaluate their assertions.
package evaluator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Check interface defines the common behavior for all check types
type Check interface {
	Evaluate(ctx context.Context) CheckResult
	GetName() string
	GetEnvironment() string
	GetInterval() time.Duration
	GetTags() []string
}

type Evaluator struct {
	httpChecks []HTTPCheck
	pingChecks []PingCheck
	clock      Clock
}

func NewEvaluator(httpChecks []HTTPCheck, pingChecks []PingCheck, clock Clock) (*Evaluator, error) {
	if err := ValidateHTTPCheckSet(httpChecks); err != nil {
		return nil, fmt.Errorf("validate HTTP check set: %w", err)
	}

	if err := ValidatePingCheckSet(pingChecks); err != nil {
		return nil, fmt.Errorf("validate Ping check set: %w", err)
	}

	return &Evaluator{
		httpChecks: httpChecks,
		pingChecks: pingChecks,
		clock:      clock,
	}, nil
}

// Run will iteratively evaluate all checks based on their configured schedules until the context is cancelled.
func (e *Evaluator) Run(ctx context.Context) {
	for _, check := range e.httpChecks {
		go e.httpCheckEvaluateLoop(ctx, check)
	}

	for _, check := range e.pingChecks {
		go e.pingCheckEvaluateLoop(ctx, check)
	}

	<-ctx.Done()
}

// httpCheckEvaluateLoop will continuously evaluate the given HTTP check based on its configured schedule until the context is cancelled.
// Loop uses fixed-delay scheduling with an initial phase jitter up to the check's interval.
func (e *Evaluator) httpCheckEvaluateLoop(ctx context.Context, check HTTPCheck) {
	slog.Debug(
		"starting HTTP check evaluation loop",
		"check", check.Name,
		"environment", check.Environment,
		"interval", check.Interval,
	)

	if check.Interval <= time.Second {
		// Keep the interval sane.
		check.Interval = time.Second
	}

	// Initial phase jitter, up to check interval duration.
	delay := e.clock.Jitter(check.Interval)
	timer := e.clock.NewTimer(delay)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		result := check.Evaluate(ctx)

		captureHTTPCheckMetrics(check, result, e.clock.Now())

		// Regardless of check pass or fail, log the result as `INFO` line.
		// Do not treat check failures as actual errors, check failures exist at a higher-level.
		slog.Info(
			"evaluated HTTP check for scheduled run",
			"name", check.Name,
			"environment", check.Environment,
			"tags", check.Tags,
			"pass", result.Pass,
			"latency", result.Latency.Milliseconds(),
			"error", result.Err,
		)

		timer.Reset(check.Interval)
	}
}

// pingCheckEvaluateLoop will continuously evaluate the given Ping check based on its configured schedule until the context is cancelled.
// Loop uses fixed-delay scheduling with an initial phase jitter up to the check's interval.
func (e *Evaluator) pingCheckEvaluateLoop(ctx context.Context, check PingCheck) {
	slog.Debug(
		"starting ping check evaluation loop",
		"check", check.Name,
		"environment", check.Environment,
		"interval", check.Interval,
	)

	if check.Interval <= time.Second {
		// Keep the interval sane.
		check.Interval = time.Second
	}

	// Initial phase jitter, up to check interval duration.
	delay := e.clock.Jitter(check.Interval)
	timer := e.clock.NewTimer(delay)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		result := check.Evaluate(ctx)

		capturePingCheckMetrics(check, result, e.clock.Now())

		// Regardless of check pass or fail, log the result as `INFO` line.
		// Do not treat check failures as actual errors, check failures exist at a higher-level.
		slog.Info(
			"evaluated Ping check for scheduled run",
			"name", check.Name,
			"environment", check.Environment,
			"tags", check.Tags,
			"pass", result.Pass,
			"latency", result.Latency.Milliseconds(),
			"error", result.Err,
		)

		timer.Reset(check.Interval)
	}
}

// EvaluateChecksByName will evaluate a single check by its name and log the result.
// Note that multiple checks can have the same name, distinguished by their environment, so this may evaluate multiple checks.
func (e *Evaluator) EvaluateChecksByName(ctx context.Context, checkName string) {
	for _, check := range e.httpChecks {
		if check.Name == checkName {
			result := check.Evaluate(ctx)
			slog.Info(
				"evaluated HTTP check for single run",
				"name", check.Name,
				"environment", check.Environment,
				"tags", check.Tags,
				"pass", result.Pass,
				"latency", result.Latency.Milliseconds(),
				"error", result.Err,
			)
		}
	}

	for _, check := range e.pingChecks {
		if check.Name == checkName {
			result := check.Evaluate(ctx)
			slog.Info(
				"evaluated ping check for single run",
				"name", check.Name,
				"environment", check.Environment,
				"tags", check.Tags,
				"pass", result.Pass,
				"latency", result.Latency.Milliseconds(),
				"error", result.Err,
			)
		}
	}
}

func captureHTTPCheckMetrics(check HTTPCheck, result CheckResult, lastEvaluatedAt time.Time) {
	// Build a single string with all tags.
	// Not doing `strings.Join` so as to maintain a trailing separator for easier parsing.
	var tagsStr strings.Builder
	for _, tag := range check.Tags {
		fmt.Fprintf(&tagsStr, "%s;", tag)
	}

	promLabels := prometheus.Labels{
		"name":        check.Name,
		"environment": check.Environment,
		"tags":        tagsStr.String(),
		"type":        "http",
	}

	var passGaugeValue float64
	if result.Pass {
		checkPassLatencyMs.With(promLabels).Observe(float64(result.Latency.Milliseconds()))
		passGaugeValue = 1
	}

	checkPassGauge.With(promLabels).Set(passGaugeValue)
	checkLastEvaluatedGauge.With(promLabels).Set(float64(lastEvaluatedAt.Unix()))
	checkEvaluationsTotal.With(
		// Requires additional `pass` label.
		prometheus.Labels{
			"name":        check.Name,
			"environment": check.Environment,
			"tags":        tagsStr.String(),
			"type":        "http",
			"pass":        fmt.Sprintf("%t", result.Pass),
		},
	).Inc()
}

func capturePingCheckMetrics(check PingCheck, result CheckResult, lastEvaluatedAt time.Time) {
	// Build a single string with all tags.
	// Not doing `strings.Join` so as to maintain a trailing separator for easier parsing.
	var tagsStr strings.Builder
	for _, tag := range check.Tags {
		fmt.Fprintf(&tagsStr, "%s;", tag)
	}

	promLabels := prometheus.Labels{
		"name":        check.Name,
		"environment": check.Environment,
		"tags":        tagsStr.String(),
		"type":        "ping",
	}

	var passGaugeValue float64
	if result.Pass {
		checkPassLatencyMs.With(promLabels).Observe(float64(result.Latency.Milliseconds()))
		passGaugeValue = 1
	}

	checkPassGauge.With(promLabels).Set(passGaugeValue)
	checkLastEvaluatedGauge.With(promLabels).Set(float64(lastEvaluatedAt.Unix()))
	checkEvaluationsTotal.With(
		// Requires additional `pass` label.
		prometheus.Labels{
			"name":        check.Name,
			"environment": check.Environment,
			"tags":        tagsStr.String(),
			"type":        "ping",
			"pass":        fmt.Sprintf("%t", result.Pass),
		},
	).Inc()
}
