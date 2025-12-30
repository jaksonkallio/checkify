package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInterpolator(t *testing.T) {
	t.Run("Interpolate", func(t *testing.T) {
		t.Run("SinglePlaceholder", func(t *testing.T) {
			interp := NewInterpolator("env")
			interp.Set("API_KEY", "secret123")

			result := interp.Interpolate(InterpolatedString("Bearer ${env.API_KEY}"))

			assert.Equal(t, "Bearer secret123", result)
		})

		t.Run("MultiplePlaceholders", func(t *testing.T) {
			interp := NewInterpolator("env")
			interp.Set("HOST", "api.example.com")
			interp.Set("PORT", "8080")
			interp.Set("PROTOCOL", "https")

			result := interp.Interpolate(InterpolatedString("${env.PROTOCOL}://${env.HOST}:${env.PORT}/api"))

			assert.Equal(t, "https://api.example.com:8080/api", result)
		})

		t.Run("MissingVariable", func(t *testing.T) {
			interp := NewInterpolator("env")

			result := interp.Interpolate(InterpolatedString("Value: ${env.MISSING}"))

			// Should leave placeholder unchanged when variable is missing
			assert.Equal(t, "Value: ${env.MISSING}", result)
		})

		t.Run("NoPlaceholders", func(t *testing.T) {
			interp := NewInterpolator("env")
			interp.Set("KEY", "value")

			result := interp.Interpolate(InterpolatedString("plain text without placeholders"))

			assert.Equal(t, "plain text without placeholders", result)
		})

		t.Run("EmptyString", func(t *testing.T) {
			interp := NewInterpolator("env")
			interp.Set("KEY", "value")

			result := interp.Interpolate(InterpolatedString(""))

			assert.Equal(t, "", result)
		})

		t.Run("SpecialCharacters", func(t *testing.T) {
			interp := NewInterpolator("env")
			interp.Set("SPECIAL", "value with $pecial #chars & symbols!")

			result := interp.Interpolate(InterpolatedString("Config: ${env.SPECIAL}"))

			assert.Equal(t, "Config: value with $pecial #chars & symbols!", result)
		})

		t.Run("SamePlaceholderMultipleTimes", func(t *testing.T) {
			interp := NewInterpolator("env")
			interp.Set("DOMAIN", "example.com")

			result := interp.Interpolate(InterpolatedString("${env.DOMAIN}/api and ${env.DOMAIN}/metrics"))

			assert.Equal(t, "example.com/api and example.com/metrics", result)
		})

		t.Run("DifferentPrefix", func(t *testing.T) {
			interp := NewInterpolator("var")
			interp.Set("endpoint", "api.example.com")

			result := interp.Interpolate(InterpolatedString("https://${var.endpoint}/v1"))

			assert.Equal(t, "https://api.example.com/v1", result)
		})

		t.Run("EdgeCaseEmptyValue", func(t *testing.T) {
			interp := NewInterpolator("env")
			interp.Set("EMPTY", "")

			result := interp.Interpolate(InterpolatedString("Prefix${env.EMPTY}Suffix"))

			assert.Equal(t, "PrefixSuffix", result)
		})

		t.Run("NestedBraces", func(t *testing.T) {
			interp := NewInterpolator("env")
			interp.Set("KEY", "value")

			// Should only replace the properly formatted placeholder
			result := interp.Interpolate(InterpolatedString("${env.KEY} but not ${{env.KEY}}"))

			assert.Equal(t, "value but not ${{env.KEY}}", result)
		})
	})

	t.Run("ContainsPlaceholder", func(t *testing.T) {
		t.Run("True", func(t *testing.T) {
			interp := NewInterpolator("env")

			result := interp.ContainsPlaceholder("Value with ${env.PLACEHOLDER}")

			assert.True(t, result)
		})

		t.Run("False", func(t *testing.T) {
			interp := NewInterpolator("env")

			result := interp.ContainsPlaceholder("Value without any placeholder")

			assert.False(t, result)
		})

		t.Run("WrongPrefix", func(t *testing.T) {
			interp := NewInterpolator("env")

			// Should not detect placeholders with different prefix
			result := interp.ContainsPlaceholder("Value with ${var.SOMETHING}")

			assert.False(t, result)
		})
	})

	t.Run("Set", func(t *testing.T) {
		t.Run("OverwriteValue", func(t *testing.T) {
			interp := NewInterpolator("env")
			interp.Set("KEY", "first_value")
			interp.Set("KEY", "second_value")

			result := interp.Interpolate(InterpolatedString("${env.KEY}"))

			assert.Equal(t, "second_value", result)
		})
	})

	t.Run("Clear", func(t *testing.T) {
		t.Run("RemovesAllValues", func(t *testing.T) {
			interp := NewInterpolator("env")
			interp.Set("KEY1", "value1")
			interp.Set("KEY2", "value2")

			interp.Clear()

			result1 := interp.Interpolate(InterpolatedString("${env.KEY1}"))
			result2 := interp.Interpolate(InterpolatedString("${env.KEY2}"))

			assert.Equal(t, "${env.KEY1}", result1)
			assert.Equal(t, "${env.KEY2}", result2)
		})
	})
}

func TestInterpolatorSet(t *testing.T) {
	t.Run("InterpolateMany", func(t *testing.T) {
		t.Run("TwoPass", func(t *testing.T) {
			// Simulates the actual usage pattern in cmd/checks.go
			envInterp := NewInterpolator("env")
			envInterp.Set("BASE_URL", "https://api.example.com")

			varInterp := NewInterpolator("var")
			// In real usage, values are pre-interpolated before being set
			endpointValue := envInterp.Interpolate(InterpolatedString("${env.BASE_URL}/v1"))
			varInterp.Set("endpoint", endpointValue)

			set := NewInterpolatorSet(envInterp, varInterp)

			result := set.InterpolateMany(InterpolatedString("${var.endpoint}/users"))

			assert.Equal(t, "https://api.example.com/v1/users", result)
		})

		t.Run("Order", func(t *testing.T) {
			interp1 := NewInterpolator("env")
			interp1.Set("VALUE", "from_env")

			interp2 := NewInterpolator("var")
			interp2.Set("VALUE", "from_var")

			// First interpolator should be applied first
			set := NewInterpolatorSet(interp1, interp2)

			result1 := set.InterpolateMany(InterpolatedString("${env.VALUE}"))
			result2 := set.InterpolateMany(InterpolatedString("${var.VALUE}"))

			assert.Equal(t, "from_env", result1)
			assert.Equal(t, "from_var", result2)
		})

		t.Run("EmptySet", func(t *testing.T) {
			set := NewInterpolatorSet()

			result := set.InterpolateMany(InterpolatedString("${env.VALUE}"))

			assert.Equal(t, "${env.VALUE}", result)
		})
	})

	t.Run("ContainsPlaceholder", func(t *testing.T) {
		t.Run("MultipleInterpolators", func(t *testing.T) {
			envInterp := NewInterpolator("env")
			varInterp := NewInterpolator("var")

			set := NewInterpolatorSet(envInterp, varInterp)

			hasEnv := set.ContainsPlaceholder("${env.VALUE}")
			hasVar := set.ContainsPlaceholder("${var.VALUE}")
			hasNone := set.ContainsPlaceholder("no placeholder")

			assert.True(t, hasEnv)
			assert.True(t, hasVar)
			assert.False(t, hasNone)
		})

		t.Run("DetectsAnyPrefix", func(t *testing.T) {
			envInterp := NewInterpolator("env")
			varInterp := NewInterpolator("var")

			set := NewInterpolatorSet(envInterp, varInterp)

			result := set.ContainsPlaceholder("Mix of ${env.A} and ${var.B}")

			assert.True(t, result)
		})
	})
}
