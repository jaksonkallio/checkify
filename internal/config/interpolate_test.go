package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInterpolator_Interpolate_SinglePlaceholder(t *testing.T) {
	interp := NewInterpolator("env")
	interp.Set("API_KEY", "secret123")

	result := interp.Interpolate(InterpolatedString("Bearer ${env.API_KEY}"))

	assert.Equal(t, "Bearer secret123", result)
}

func TestInterpolator_Interpolate_MultiplePlaceholders(t *testing.T) {
	interp := NewInterpolator("env")
	interp.Set("HOST", "api.example.com")
	interp.Set("PORT", "8080")
	interp.Set("PROTOCOL", "https")

	result := interp.Interpolate(InterpolatedString("${env.PROTOCOL}://${env.HOST}:${env.PORT}/api"))

	assert.Equal(t, "https://api.example.com:8080/api", result)
}

func TestInterpolator_Interpolate_MissingVariable(t *testing.T) {
	interp := NewInterpolator("env")

	result := interp.Interpolate(InterpolatedString("Value: ${env.MISSING}"))

	// Should leave placeholder unchanged when variable is missing
	assert.Equal(t, "Value: ${env.MISSING}", result)
}

func TestInterpolator_Interpolate_NoPlaceholders(t *testing.T) {
	interp := NewInterpolator("env")
	interp.Set("KEY", "value")

	result := interp.Interpolate(InterpolatedString("plain text without placeholders"))

	assert.Equal(t, "plain text without placeholders", result)
}

func TestInterpolator_Interpolate_EmptyString(t *testing.T) {
	interp := NewInterpolator("env")
	interp.Set("KEY", "value")

	result := interp.Interpolate(InterpolatedString(""))

	assert.Equal(t, "", result)
}

func TestInterpolator_Interpolate_SpecialCharacters(t *testing.T) {
	interp := NewInterpolator("env")
	interp.Set("SPECIAL", "value with $pecial #chars & symbols!")

	result := interp.Interpolate(InterpolatedString("Config: ${env.SPECIAL}"))

	assert.Equal(t, "Config: value with $pecial #chars & symbols!", result)
}

func TestInterpolator_Interpolate_SamePlaceholderMultipleTimes(t *testing.T) {
	interp := NewInterpolator("env")
	interp.Set("DOMAIN", "example.com")

	result := interp.Interpolate(InterpolatedString("${env.DOMAIN}/api and ${env.DOMAIN}/metrics"))

	assert.Equal(t, "example.com/api and example.com/metrics", result)
}

func TestInterpolator_Interpolate_DifferentPrefix(t *testing.T) {
	interp := NewInterpolator("var")
	interp.Set("endpoint", "api.example.com")

	result := interp.Interpolate(InterpolatedString("https://${var.endpoint}/v1"))

	assert.Equal(t, "https://api.example.com/v1", result)
}

func TestInterpolator_ContainsPlaceholder_True(t *testing.T) {
	interp := NewInterpolator("env")

	result := interp.ContainsPlaceholder("Value with ${env.PLACEHOLDER}")

	assert.True(t, result)
}

func TestInterpolator_ContainsPlaceholder_False(t *testing.T) {
	interp := NewInterpolator("env")

	result := interp.ContainsPlaceholder("Value without any placeholder")

	assert.False(t, result)
}

func TestInterpolator_ContainsPlaceholder_WrongPrefix(t *testing.T) {
	interp := NewInterpolator("env")

	// Should not detect placeholders with different prefix
	result := interp.ContainsPlaceholder("Value with ${var.SOMETHING}")

	assert.False(t, result)
}

func TestInterpolator_Set_OverwriteValue(t *testing.T) {
	interp := NewInterpolator("env")
	interp.Set("KEY", "first_value")
	interp.Set("KEY", "second_value")

	result := interp.Interpolate(InterpolatedString("${env.KEY}"))

	assert.Equal(t, "second_value", result)
}

func TestInterpolator_Clear_RemovesAllValues(t *testing.T) {
	interp := NewInterpolator("env")
	interp.Set("KEY1", "value1")
	interp.Set("KEY2", "value2")

	interp.Clear()

	result1 := interp.Interpolate(InterpolatedString("${env.KEY1}"))
	result2 := interp.Interpolate(InterpolatedString("${env.KEY2}"))

	assert.Equal(t, "${env.KEY1}", result1)
	assert.Equal(t, "${env.KEY2}", result2)
}

func TestInterpolatorSet_InterpolateMany_TwoPass(t *testing.T) {
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
}

func TestInterpolatorSet_InterpolateMany_Order(t *testing.T) {
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
}

func TestInterpolatorSet_InterpolateMany_EmptySet(t *testing.T) {
	set := NewInterpolatorSet()

	result := set.InterpolateMany(InterpolatedString("${env.VALUE}"))

	assert.Equal(t, "${env.VALUE}", result)
}

func TestInterpolatorSet_ContainsPlaceholder_MultipleInterpolators(t *testing.T) {
	envInterp := NewInterpolator("env")
	varInterp := NewInterpolator("var")

	set := NewInterpolatorSet(envInterp, varInterp)

	hasEnv := set.ContainsPlaceholder("${env.VALUE}")
	hasVar := set.ContainsPlaceholder("${var.VALUE}")
	hasNone := set.ContainsPlaceholder("no placeholder")

	assert.True(t, hasEnv)
	assert.True(t, hasVar)
	assert.False(t, hasNone)
}

func TestInterpolatorSet_ContainsPlaceholder_DetectsAnyPrefix(t *testing.T) {
	envInterp := NewInterpolator("env")
	varInterp := NewInterpolator("var")

	set := NewInterpolatorSet(envInterp, varInterp)

	result := set.ContainsPlaceholder("Mix of ${env.A} and ${var.B}")

	assert.True(t, result)
}

func TestInterpolator_Interpolate_EdgeCaseEmptyValue(t *testing.T) {
	interp := NewInterpolator("env")
	interp.Set("EMPTY", "")

	result := interp.Interpolate(InterpolatedString("Prefix${env.EMPTY}Suffix"))

	assert.Equal(t, "PrefixSuffix", result)
}

func TestInterpolator_Interpolate_NestedBraces(t *testing.T) {
	interp := NewInterpolator("env")
	interp.Set("KEY", "value")

	// Should only replace the properly formatted placeholder
	result := interp.Interpolate(InterpolatedString("${env.KEY} but not ${{env.KEY}}"))

	assert.Equal(t, "value but not ${{env.KEY}}", result)
}
