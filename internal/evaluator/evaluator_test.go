package evaluator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEvaluator(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		checks := []Check{
			{Name: "check1", Environment: "prod", Interval: time.Second},
			{Name: "check2", Environment: "prod", Interval: time.Second},
			{Name: "ping1", Environment: "prod", Interval: time.Second},
		}

		evaluator, err := NewEvaluator(checks, RealClock{})

		require.NoError(t, err)
		assert.NotNil(t, evaluator)
		assert.Len(t, evaluator.checks, 3)
	})

	t.Run("ValidatesCheckSet", func(t *testing.T) {
		checks := []Check{
			{Name: "duplicate", Environment: "prod", Interval: time.Second},
			{Name: "duplicate", Environment: "prod", Interval: time.Second}, // Duplicate
		}

		_, err := NewEvaluator(checks, RealClock{})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "validate check set")
	})

	t.Run("DuplicateCheckError", func(t *testing.T) {
		checks := []Check{
			{Name: "api-check", Environment: "prod", Interval: time.Second},
			{Name: "api-check", Environment: "prod", Interval: time.Second}, // Exact duplicate
		}

		_, err := NewEvaluator(checks, RealClock{})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate check key")
	})

	t.Run("EmptyCheckSet", func(t *testing.T) {
		checks := []Check{}

		evaluator, err := NewEvaluator(checks, RealClock{})

		require.NoError(t, err)
		assert.NotNil(t, evaluator)
		assert.Len(t, evaluator.checks, 0)
	})
}

func TestEvaluator_EvaluateChecksByName(t *testing.T) {
	t.Run("SingleHTTPCheck", func(t *testing.T) {
		// Create a simple check with no HTTP client (won't actually execute)
		httpEval := &HTTPCheckEvaluator{
			Timeout: -1 * time.Second, // Negative timeout = immediate error
		}
		check := Check{
			Name:        "test-check",
			Environment: "test",
			Interval:    time.Second,
			Evaluator:   httpEval,
		}

		evaluator, err := NewEvaluator([]Check{check}, RealClock{})
		require.NoError(t, err)

		// This should not panic, it will just execute the check
		evaluator.EvaluateChecksByName(context.Background(), "test-check")

		// Test completes successfully if no panic
	})

	t.Run("SinglePingCheck", func(t *testing.T) {
		pingEval := &PingCheckEvaluator{
			Timeout: -1 * time.Second, // Negative timeout = immediate error
		}
		check := Check{
			Name:        "test-ping",
			Environment: "test",
			Interval:    time.Second,
			Evaluator:   pingEval,
		}

		evaluator, err := NewEvaluator([]Check{check}, RealClock{})
		require.NoError(t, err)

		// This should not panic, it will just execute the check
		evaluator.EvaluateChecksByName(context.Background(), "test-ping")

		// Test completes successfully if no panic
	})

	t.Run("MultipleChecks", func(t *testing.T) {
		checks := []Check{
			{Name: "check1", Environment: "prod", Interval: time.Second, Evaluator: &HTTPCheckEvaluator{Timeout: -1 * time.Second}},
			{Name: "check1", Environment: "stage", Interval: time.Second, Evaluator: &HTTPCheckEvaluator{Timeout: -1 * time.Second}},
			{Name: "check2", Environment: "prod", Interval: time.Second, Evaluator: &HTTPCheckEvaluator{Timeout: -1 * time.Second}},
			{Name: "check1", Environment: "test", Interval: time.Second, Evaluator: &PingCheckEvaluator{Timeout: -1 * time.Second}},
		}

		evaluator, err := NewEvaluator(checks, RealClock{})
		require.NoError(t, err)

		// Should evaluate all checks with name "check1" (HTTP in prod, HTTP in stage, Ping in test)
		evaluator.EvaluateChecksByName(context.Background(), "check1")

		// Test completes successfully if no panic
	})

	t.Run("CheckNotFound", func(t *testing.T) {
		checks := []Check{
			{Name: "existing-check", Environment: "prod", Interval: time.Second, Evaluator: &HTTPCheckEvaluator{Timeout: -1 * time.Second}},
		}

		evaluator, err := NewEvaluator(checks, RealClock{})
		require.NoError(t, err)

		// Should not panic even if check doesn't exist
		evaluator.EvaluateChecksByName(context.Background(), "nonexistent")

		// Test completes successfully if no panic
	})
}

// MockClock for testing scheduling behavior
type MockClock struct {
	JitterDuration time.Duration
}

func (mc MockClock) Now() time.Time {
	return time.Now()
}

func (mc MockClock) NewTimer(d time.Duration) *time.Timer {
	return time.NewTimer(d)
}

func (mc MockClock) Jitter(max time.Duration) time.Duration {
	return mc.JitterDuration
}

func TestRealClock(t *testing.T) {
	t.Run("Jitter", func(t *testing.T) {
		t.Run("ZeroMax", func(t *testing.T) {
			clock := RealClock{}

			result := clock.Jitter(0)

			assert.Equal(t, time.Duration(0), result)
		})

		t.Run("NegativeMax", func(t *testing.T) {
			clock := RealClock{}

			result := clock.Jitter(-10 * time.Second)

			assert.Equal(t, time.Duration(0), result)
		})

		t.Run("PositiveMax", func(t *testing.T) {
			clock := RealClock{}

			result := clock.Jitter(10 * time.Second)

			// Should be between 0 and max
			assert.GreaterOrEqual(t, result, time.Duration(0))
			assert.LessOrEqual(t, result, 10*time.Second)
		})
	})

	t.Run("Now", func(t *testing.T) {
		clock := RealClock{}

		before := time.Now()
		result := clock.Now()
		after := time.Now()

		assert.True(t, result.After(before) || result.Equal(before))
		assert.True(t, result.Before(after) || result.Equal(after))
	})

	t.Run("NewTimer", func(t *testing.T) {
		clock := RealClock{}

		timer := clock.NewTimer(10 * time.Millisecond)

		assert.NotNil(t, timer)
		timer.Stop() // Clean up
	})
}

// logAttr looks up a single log attribute on a check result by key.
func logAttr(t *testing.T, result CheckResult, key string) slog.Value {
	t.Helper()

	for _, attr := range result.LogAttrs {
		if attr.Key == key {
			return attr.Value
		}
	}

	t.Fatalf("no log attribute with key %q in %v", key, result.LogAttrs)
	return slog.Value{}
}

// hasLogAttr reports whether a check result carries a log attribute with the given key.
func hasLogAttr(result CheckResult, key string) bool {
	for _, attr := range result.LogAttrs {
		if attr.Key == key {
			return true
		}
	}

	return false
}

func TestCheckResultLogArgs(t *testing.T) {
	t.Run("IncludesCommonAttributes", func(t *testing.T) {
		check := Check{Name: "check1", Environment: "prod", Tags: []string{"tag1"}}
		result := CheckResult{Pass: true, Latency: 250 * time.Millisecond}

		args := checkResultLogArgs(check, result)

		assert.Equal(t, []any{
			"evaluatorType", "<nil>",
			"name", "check1",
			"environment", "prod",
			"tags", []string{"tag1"},
			"pass", true,
			"latency", int64(250),
			"error", error(nil),
		}, args)
	})

	t.Run("AppendsCheckSpecificAttributes", func(t *testing.T) {
		check := Check{Name: "check1", Environment: "prod"}
		result := CheckResult{
			Pass: false,
			LogAttrs: []slog.Attr{
				slog.Int("httpStatusCode", 503),
				slog.String("httpBodyPrefix", "retry later"),
			},
		}

		args := checkResultLogArgs(check, result)

		require.Len(t, args, 16)
		assert.Equal(t, slog.Int("httpStatusCode", 503), args[14])
		assert.Equal(t, slog.String("httpBodyPrefix", "retry later"), args[15])
	})
}

func TestTruncateForLog(t *testing.T) {
	t.Run("ShortPayloadKeptWhole", func(t *testing.T) {
		assert.Equal(t, "retry later", truncateForLog("retry later"))
	})

	t.Run("SurroundingWhitespaceTrimmed", func(t *testing.T) {
		assert.Equal(t, "retry later", truncateForLog("\n  retry later\n"))
	})

	t.Run("LongPayloadTruncated", func(t *testing.T) {
		payload := strings.Repeat("a", MaxLoggedPayloadLength+50)

		truncated := truncateForLog(payload)

		assert.Len(t, truncated, MaxLoggedPayloadLength)
	})

	t.Run("MultiByteCharactersNotSplit", func(t *testing.T) {
		// Each rune is 3 bytes, so a byte-wise cut would leave an invalid character at the end.
		payload := strings.Repeat("あ", MaxLoggedPayloadLength+10)

		truncated := truncateForLog(payload)

		assert.True(t, utf8.ValidString(truncated))
		assert.Equal(t, MaxLoggedPayloadLength, utf8.RuneCountInString(truncated))
	})

	t.Run("EmptyPayload", func(t *testing.T) {
		assert.Equal(t, "", truncateForLog(""))
	})
}

func TestTruncateListForLog(t *testing.T) {
	t.Run("ShortListKeptWhole", func(t *testing.T) {
		values := []string{"1.2.3.4", "5.6.7.8"}

		assert.Equal(t, values, truncateListForLog(values))
	})

	t.Run("LongListCapped", func(t *testing.T) {
		values := make([]string, 0, MaxLoggedListItems+20)
		for i := range MaxLoggedListItems + 20 {
			values = append(values, fmt.Sprintf("record-%d", i))
		}

		truncated := truncateListForLog(values)

		require.Len(t, truncated, MaxLoggedListItems)
		// The retained items are the first ones, unmodified.
		assert.Equal(t, values[:MaxLoggedListItems], truncated)
	})

	t.Run("LongValuesTruncated", func(t *testing.T) {
		token := strings.Repeat("t", MaxLoggedPayloadLength+200)

		truncated := truncateListForLog([]string{"short", token})

		require.Len(t, truncated, 2)
		assert.Equal(t, "short", truncated[0])
		assert.Len(t, truncated[1], MaxLoggedPayloadLength)
	})

	t.Run("BothCapsAppliedTogether", func(t *testing.T) {
		values := make([]string, 0, MaxLoggedListItems+5)
		for range MaxLoggedListItems + 5 {
			values = append(values, strings.Repeat("t", MaxLoggedPayloadLength+200))
		}

		truncated := truncateListForLog(values)

		require.Len(t, truncated, MaxLoggedListItems)
		for _, value := range truncated {
			assert.Len(t, value, MaxLoggedPayloadLength)
		}
	})

	t.Run("EmptyList", func(t *testing.T) {
		assert.Nil(t, truncateListForLog(nil))
		assert.Nil(t, truncateListForLog([]string{}))
	})

	t.Run("DoesNotMutateInput", func(t *testing.T) {
		values := []string{strings.Repeat("t", MaxLoggedPayloadLength+10)}

		truncateListForLog(values)

		assert.Len(t, values[0], MaxLoggedPayloadLength+10)
	})
}
