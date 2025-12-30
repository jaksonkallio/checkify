package evaluator

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	checkPassLatencyMs = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "check_pass_latency_ms",
		Help:    "Histogram of passing check latencies, in milliseconds.",
		Buckets: prometheus.ExponentialBucketsRange(10, 120000, 25),
	}, []string{
		"name",
		"environment",
		"tags",
	})

	checkEvaluationsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "check_evaluations_total",
		Help: "Total number of check evaluations.",
	}, []string{
		"name",
		"environment",
		"tags",
		"pass",
	})

	checkPassGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "check_pass",
		Help: "Current state of the check, passing (1) or failing (0).",
	}, []string{
		"name",
		"environment",
		"tags",
	})

	checkLastEvaluatedGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "check_last_evaluated_timestamp_seconds",
		Help: "Timestamp of the last evaluation of the check, in unix epoch seconds.",
	}, []string{
		"name",
		"environment",
		"tags",
	})
)
