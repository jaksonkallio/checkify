package evaluator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckDurationMetric_NotNil(t *testing.T) {
	// Verify the metric is initialized
	assert.NotNil(t, checkDurationMetric)
}
