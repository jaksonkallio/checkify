package evaluator

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	checkDurationMetric = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "check_duration",
		Help:    "Histogram of check durations.",
		Buckets: prometheus.ExponentialBucketsRange(10, 120000, 25),
	}, []string{
		"environment",
		"name",
		"pass",
		"tags",
	})
)
