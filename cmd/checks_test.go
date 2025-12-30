package main

import (
	"testing"
	"time"

	"github.com/messari/checkify/internal/config"
	"github.com/messari/checkify/internal/evaluator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertChecks(t *testing.T) {
	t.Run("BasicSuccess", func(t *testing.T) {
		envProvider := MockEnvProvider{
			Vars: map[string]string{},
		}

		cfg := config.Config{
			Environments: []config.Environment{
				{
					Name: "test",
					Vars: map[string]config.InterpolatedString{
						"api_url": config.InterpolatedString("https://api.test.com"),
					},
				},
			},
			Checks: []config.Check{
				{
					Name:           "health-check",
					Environments:   []string{"test"},
					Endpoint:       config.InterpolatedString("${var.api_url}/health"),
					Method:         "GET",
					Interval:       30 * time.Second,
					Timeout:        5 * time.Second,
					ResponseFormat: config.FormatJSON,
				},
			},
		}

		checks, err := convertChecks(cfg, envProvider)

		require.NoError(t, err)
		require.Len(t, checks, 1)
		assert.Equal(t, "health-check", checks[0].Name)
		assert.Equal(t, "https://api.test.com/health", checks[0].Endpoint)
		assert.Equal(t, "GET", checks[0].Method)
		assert.Equal(t, "test", checks[0].Environment)
		assert.Equal(t, evaluator.FormatJSON, checks[0].ResponseFormat)
	})

	t.Run("EnvInterpolation", func(t *testing.T) {
		envProvider := MockEnvProvider{
			Vars: map[string]string{
				"API_TOKEN":   "secret_token_456",
				"BASE_DOMAIN": "example.com",
			},
		}

		cfg := config.Config{
			Environments: []config.Environment{
				{
					Name: "prod",
					Vars: map[string]config.InterpolatedString{
						"api_url": config.InterpolatedString("https://api.${env.BASE_DOMAIN}"),
					},
				},
			},
			Checks: []config.Check{
				{
					Name:         "api-check",
					Environments: []string{"prod"},
					Endpoint:     config.InterpolatedString("${var.api_url}/status"),
					Method:       "GET",
					Interval:     30 * time.Second,
					Timeout:      5 * time.Second,
				},
			},
		}

		checks, err := convertChecks(cfg, envProvider)

		require.NoError(t, err)
		require.Len(t, checks, 1)
		assert.Equal(t, "https://api.example.com/status", checks[0].Endpoint)
	})

	t.Run("VarInterpolation", func(t *testing.T) {
		envProvider := MockEnvProvider{
			Vars: map[string]string{},
		}

		cfg := config.Config{
			Environments: []config.Environment{
				{
					Name: "test",
					Vars: map[string]config.InterpolatedString{
						"base_url": config.InterpolatedString("https://api.test.com"),
						"version":  config.InterpolatedString("v1"),
					},
				},
			},
			Checks: []config.Check{
				{
					Name:         "versioned-endpoint",
					Environments: []string{"test"},
					Endpoint:     config.InterpolatedString("${var.base_url}/${var.version}/users"),
					Method:       "GET",
					Interval:     30 * time.Second,
					Timeout:      5 * time.Second,
				},
			},
		}

		checks, err := convertChecks(cfg, envProvider)

		require.NoError(t, err)
		require.Len(t, checks, 1)
		assert.Equal(t, "https://api.test.com/v1/users", checks[0].Endpoint)
	})

	t.Run("EnvThenVarInterpolation", func(t *testing.T) {
		envProvider := MockEnvProvider{
			Vars: map[string]string{
				"PROD_TOKEN":  "prod_secret_token",
				"STAGE_TOKEN": "stage_secret_token",
			},
		}

		cfg := config.Config{
			Environments: []config.Environment{
				{
					Name: "prod",
					Vars: map[string]config.InterpolatedString{
						"api_url": config.InterpolatedString("https://api.example.com"),
						"token":   config.InterpolatedString("${env.PROD_TOKEN}"),
					},
				},
			},
			Checks: []config.Check{
				{
					Name:         "api-check",
					Environments: []string{"prod"},
					Endpoint:     config.InterpolatedString("${var.api_url}/v1/status"),
					Method:       "GET",
					Headers: map[string]config.InterpolatedString{
						"Authorization": config.InterpolatedString("Bearer ${var.token}"),
					},
					Interval: 30 * time.Second,
					Timeout:  5 * time.Second,
				},
			},
		}

		checks, err := convertChecks(cfg, envProvider)

		require.NoError(t, err)
		require.Len(t, checks, 1)
		assert.Equal(t, "https://api.example.com/v1/status", checks[0].Endpoint)
		assert.Equal(t, "Bearer prod_secret_token", checks[0].Headers["Authorization"])
	})

	t.Run("MultipleEnvironments", func(t *testing.T) {
		envProvider := MockEnvProvider{
			Vars: map[string]string{},
		}

		cfg := config.Config{
			Environments: []config.Environment{
				{
					Name: "prod",
					Vars: map[string]config.InterpolatedString{
						"api_url": config.InterpolatedString("https://api.prod.com"),
					},
				},
				{
					Name: "stage",
					Vars: map[string]config.InterpolatedString{
						"api_url": config.InterpolatedString("https://api.stage.com"),
					},
				},
			},
			Checks: []config.Check{
				{
					Name:         "health-check",
					Environments: []string{"prod", "stage"}, // Should create 2 checks
					Endpoint:     config.InterpolatedString("${var.api_url}/health"),
					Method:       "GET",
					Interval:     30 * time.Second,
					Timeout:      5 * time.Second,
				},
			},
		}

		checks, err := convertChecks(cfg, envProvider)

		require.NoError(t, err)
		require.Len(t, checks, 2)

		// Check prod environment
		assert.Equal(t, "health-check", checks[0].Name)
		assert.Equal(t, "https://api.prod.com/health", checks[0].Endpoint)
		assert.Equal(t, "prod", checks[0].Environment)

		// Check stage environment
		assert.Equal(t, "health-check", checks[1].Name)
		assert.Equal(t, "https://api.stage.com/health", checks[1].Endpoint)
		assert.Equal(t, "stage", checks[1].Environment)
	})

	t.Run("HeaderInterpolation", func(t *testing.T) {
		envProvider := MockEnvProvider{
			Vars: map[string]string{
				"API_KEY": "secret-key-123",
			},
		}

		cfg := config.Config{
			Environments: []config.Environment{
				{
					Name: "test",
					Vars: map[string]config.InterpolatedString{
						"token": config.InterpolatedString("${env.API_KEY}"),
					},
				},
			},
			Checks: []config.Check{
				{
					Name:         "secured-endpoint",
					Environments: []string{"test"},
					Endpoint:     config.InterpolatedString("https://api.test.com/secure"),
					Method:       "GET",
					Headers: map[string]config.InterpolatedString{
						"X-API-Key":    config.InterpolatedString("${var.token}"),
						"Content-Type": config.InterpolatedString("application/json"),
					},
					Interval: 30 * time.Second,
					Timeout:  5 * time.Second,
				},
			},
		}

		checks, err := convertChecks(cfg, envProvider)

		require.NoError(t, err)
		require.Len(t, checks, 1)
		assert.Equal(t, "secret-key-123", checks[0].Headers["X-API-Key"])
		assert.Equal(t, "application/json", checks[0].Headers["Content-Type"])
	})

	t.Run("UninterpolatedEnvPlaceholder", func(t *testing.T) {
		envProvider := MockEnvProvider{
			Vars: map[string]string{
				// MISSING_VAR is not provided
			},
		}

		cfg := config.Config{
			Environments: []config.Environment{
				{
					Name: "test",
					Vars: map[string]config.InterpolatedString{
						"api_url": config.InterpolatedString("https://${env.MISSING_VAR}.com"),
					},
				},
			},
			Checks: []config.Check{
				{
					Name:         "test-check",
					Environments: []string{"test"},
					Endpoint:     config.InterpolatedString("${var.api_url}/health"),
					Method:       "GET",
					Interval:     30 * time.Second,
					Timeout:      5 * time.Second,
				},
			},
		}

		_, err := convertChecks(cfg, envProvider)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "uninterpolated placeholder")
		assert.Contains(t, err.Error(), "test")
		assert.Contains(t, err.Error(), "api_url")
	})

	t.Run("UninterpolatedVarPlaceholder", func(t *testing.T) {
		envProvider := MockEnvProvider{
			Vars: map[string]string{},
		}

		cfg := config.Config{
			Environments: []config.Environment{
				{
					Name: "test",
					Vars: map[string]config.InterpolatedString{
						"api_url": config.InterpolatedString("https://api.test.com"),
					},
				},
			},
			Checks: []config.Check{
				{
					Name:         "test-check",
					Environments: []string{"test"},
					Endpoint:     config.InterpolatedString("${var.NONEXISTENT}/health"),
					Method:       "GET",
					Interval:     30 * time.Second,
					Timeout:      5 * time.Second,
				},
			},
		}

		_, err := convertChecks(cfg, envProvider)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "uninterpolated placeholder")
		assert.Contains(t, err.Error(), "endpoint")
	})

	t.Run("UninterpolatedHeaderPlaceholder", func(t *testing.T) {
		envProvider := MockEnvProvider{
			Vars: map[string]string{},
		}

		cfg := config.Config{
			Environments: []config.Environment{
				{
					Name: "test",
					Vars: map[string]config.InterpolatedString{
						"api_url": config.InterpolatedString("https://api.test.com"),
					},
				},
			},
			Checks: []config.Check{
				{
					Name:         "test-check",
					Environments: []string{"test"},
					Endpoint:     config.InterpolatedString("${var.api_url}/health"),
					Method:       "GET",
					Headers: map[string]config.InterpolatedString{
						"Authorization": config.InterpolatedString("Bearer ${var.MISSING_TOKEN}"),
					},
					Interval: 30 * time.Second,
					Timeout:  5 * time.Second,
				},
			},
		}

		_, err := convertChecks(cfg, envProvider)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "uninterpolated placeholder")
		assert.Contains(t, err.Error(), "header")
	})

	t.Run("WithTags", func(t *testing.T) {
		envProvider := MockEnvProvider{
			Vars: map[string]string{},
		}

		cfg := config.Config{
			Environments: []config.Environment{
				{
					Name: "test",
					Vars: map[string]config.InterpolatedString{
						"api_url": config.InterpolatedString("https://api.test.com"),
					},
				},
			},
			Checks: []config.Check{
				{
					Name:         "tagged-check",
					Environments: []string{"test"},
					Endpoint:     config.InterpolatedString("${var.api_url}/health"),
					Method:       "GET",
					Interval:     30 * time.Second,
					Timeout:      5 * time.Second,
					Tags:         []string{"critical", "api", "health"},
				},
			},
		}

		checks, err := convertChecks(cfg, envProvider)

		require.NoError(t, err)
		require.Len(t, checks, 1)
		assert.Equal(t, []string{"critical", "api", "health"}, checks[0].Tags)
	})

	t.Run("WithAssertion", func(t *testing.T) {
		envProvider := MockEnvProvider{
			Vars: map[string]string{},
		}

		cfg := config.Config{
			Environments: []config.Environment{
				{
					Name: "test",
					Vars: map[string]config.InterpolatedString{
						"api_url": config.InterpolatedString("https://api.test.com"),
					},
				},
			},
			Checks: []config.Check{
				{
					Name:         "check-with-assertion",
					Environments: []string{"test"},
					Endpoint:     config.InterpolatedString("${var.api_url}/health"),
					Method:       "GET",
					Interval:     30 * time.Second,
					Timeout:      5 * time.Second,
					Assertion:    `code == 200 && body.status == "healthy"`,
				},
			},
		}

		checks, err := convertChecks(cfg, envProvider)

		require.NoError(t, err)
		require.Len(t, checks, 1)
		assert.Equal(t, `code == 200 && body.status == "healthy"`, checks[0].Assertion)
	})
}

func TestConvertResponseFormat(t *testing.T) {
	tests := []struct {
		name     string
		input    config.ResponseFormat
		expected evaluator.ResponseFormat
	}{
		{"JSON", config.FormatJSON, evaluator.FormatJSON},
		{"String", config.FormatString, evaluator.FormatString},
		{"Binary", config.FormatBinary, evaluator.FormatBinary},
		{"None", config.FormatNone, evaluator.FormatNone},
		{"Unknown", config.ResponseFormat("unknown"), evaluator.FormatNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertResponseFormat(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
