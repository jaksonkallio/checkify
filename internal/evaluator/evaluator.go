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

const (
	MaxCheckInterval     = 7 * 24 * time.Hour
	MaxNameLength        = 50
	MaxEnvironmentLength = 20
	MaxTags              = 10
	MaxTagLength         = 20
)

// CheckEvaluator interface defines the evaluation behavior specific to each check type
type CheckEvaluator interface {
	Evaluate(ctx context.Context) CheckResult
}

// Check contains common metadata and delegates evaluation to a CheckEvaluator
type Check struct {
	Name        string
	Environment string
	Interval    time.Duration
	Tags        []string
	Evaluator   CheckEvaluator
}

func NewCheck(
	name string,
	environment string,
	interval time.Duration,
	tags []string,
	evaluator CheckEvaluator,
) (Check, error) {
	if len(name) == 0 {
		return Check{}, fmt.Errorf("check name cannot be empty")
	}

	if len(name) > MaxNameLength {
		return Check{}, fmt.Errorf("check name cannot exceed %d characters", MaxNameLength)
	}

	if len(environment) == 0 {
		return Check{}, fmt.Errorf("check environment cannot be empty")
	}

	if len(environment) > MaxEnvironmentLength {
		return Check{}, fmt.Errorf("check environment cannot exceed %d characters", MaxEnvironmentLength)
	}

	if interval < 0 || interval > MaxCheckInterval {
		return Check{}, fmt.Errorf("check interval must be non-negative and no larger than %s", MaxCheckInterval)
	}

	if len(tags) > MaxTags {
		return Check{}, fmt.Errorf("check cannot have more than %d tags", MaxTags)
	}

	return Check{
		Name:        name,
		Environment: environment,
		Interval:    interval,
		Tags:        tags,
		Evaluator:   evaluator,
	}, nil
}

type Evaluator struct {
	checks []Check
	clock  Clock
}

func NewEvaluator(checks []Check, clock Clock) (*Evaluator, error) {
	if err := ValidateCheckSet(checks); err != nil {
		return nil, fmt.Errorf("validate check set: %w", err)
	}

	return &Evaluator{
		checks: checks,
		clock:  clock,
	}, nil
}

func ValidateCheckSet(checks []Check) error {
	// Deduplicate checks by name and environment.
	type checkKey struct {
		Name        string
		Environment string
	}
	checkNames := make(map[checkKey]struct{})
	for _, check := range checks {
		key := checkKey{
			Name:        check.Name,
			Environment: check.Environment,
		}
		if _, exists := checkNames[key]; exists {
			return fmt.Errorf("duplicate check key with name %q in environment %q", check.Name, check.Environment)
		}
		checkNames[key] = struct{}{}
	}
	return nil
}

// Run will iteratively evaluate all checks based on their configured schedules until the context is cancelled.
func (e *Evaluator) Run(ctx context.Context) {
	for _, check := range e.checks {
		go e.checkEvaluateLoop(ctx, check)
	}

	<-ctx.Done()
}

// checkEvaluateLoop will continuously evaluate the given check based on its configured schedule until the context is cancelled.
// Loop uses fixed-delay scheduling with an initial phase jitter up to the check's interval.
func (e *Evaluator) checkEvaluateLoop(ctx context.Context, check Check) {
	slog.Debug(
		"starting check evaluation loop",
		"evaluatorType", fmt.Sprintf("%T", check.Evaluator),
		"check", check.Name,
		"environment", check.Environment,
		"interval", check.Interval,
	)

	interval := check.Interval
	if interval <= time.Second {
		// Keep the interval sane.
		interval = time.Second
	}

	// Initial phase jitter, up to check interval duration.
	delay := e.clock.Jitter(interval)
	timer := e.clock.NewTimer(delay)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		result := check.Evaluator.Evaluate(ctx)

		captureCheckMetrics(check, result, e.clock.Now())

		// Regardless of check pass or fail, log the result as `INFO` line.
		// Do not treat check failures as actual errors, check failures exist at a higher-level.
		slog.Info(
			"evaluated check for scheduled run",
			"evaluatorType", fmt.Sprintf("%T", check.Evaluator),
			"name", check.Name,
			"environment", check.Environment,
			"tags", check.Tags,
			"pass", result.Pass,
			"latency", result.Latency.Milliseconds(),
			"error", result.Err,
		)

		timer.Reset(interval)
	}
}

// EvaluateChecksByName will evaluate a single check by its name and log the result.
// Note that multiple checks can have the same name, distinguished by their environment, so this may evaluate multiple checks.
func (e *Evaluator) EvaluateChecksByName(ctx context.Context, checkName string) {
	for _, check := range e.checks {
		if check.Name == checkName {
			result := check.Evaluator.Evaluate(ctx)
			slog.Info(
				"evaluated check for single run",
				"evaluatorType", fmt.Sprintf("%T", check.Evaluator),
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

func captureCheckMetrics(check Check, result CheckResult, lastEvaluatedAt time.Time) {
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
			"pass":        fmt.Sprintf("%t", result.Pass),
		},
	).Inc()
}
