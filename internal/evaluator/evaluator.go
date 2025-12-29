// Package evaluator will execute checks and evaluate their assertions.
package evaluator

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Evaluator struct {
	checks []Check
}

func NewEvaluator(checks []Check) *Evaluator {
	return &Evaluator{
		checks: checks,
	}
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
		"check", check.Name,
		"environment", check.Environment,
		"interval", check.Interval,
	)

	if check.Interval <= time.Second {
		// Keep the interval sane.
		check.Interval = time.Second
	}

	// Initial phase jitter, up to check interval duration.
	delay := time.Duration(rand.Int63n(int64(check.Interval)))
	timer := time.NewTimer(delay)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		result := check.Evaluate(ctx)

		// Capture metrics about the check.
		checkDurationMetric.With(
			prometheus.Labels{
				"name":        check.Name,
				"environment": check.Environment,
				"tags":        strings.Join(check.Tags, ","),
				"pass":        fmt.Sprintf("%t", result.Pass),
			},
		).Observe(float64(result.Latency.Milliseconds()))

		// Regardless of check pass or fail, log the result as `INFO` line.
		// Do not treat check failures as actual errors, check failures exist at a higher-level.
		slog.Info(
			"finished running scheduled check evaluation",
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
