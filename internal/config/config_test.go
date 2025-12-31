package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockFileReader is a mock implementation of FileReader interface
type MockFileReader struct {
	Files map[string][]byte
	Err   error
}

// ReadFile returns mocked file contents or an error
func (m MockFileReader) ReadFile(path string) ([]byte, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	data, exists := m.Files[path]
	if !exists {
		return nil, os.ErrNotExist
	}
	return data, nil
}

func TestLoad(t *testing.T) {
	t.Run("ValidBasicConfig", func(t *testing.T) {
		reader := MockFileReader{
			Files: map[string][]byte{
				"config.yaml": []byte(`
prometheus:
  port: 2112
  path: /metrics

environments:
  - name: test
    vars:
      api_url: https://api.test.com

checks:
  http:
    - name: health-check
      environments: [test]
      endpoint: https://api.test.com/health
      method: GET
      interval: 30s
      timeout: 5s
      response_format: json
`),
			},
		}

		cfg, err := LoadWithReader("config.yaml", reader)

		require.NoError(t, err)
		assert.Equal(t, 2112, cfg.Prometheus.Port)
		assert.Equal(t, "/metrics", cfg.Prometheus.Path)
		assert.Len(t, cfg.Environments, 1)
		assert.Equal(t, "test", cfg.Environments[0].Name)
		assert.Len(t, cfg.Checks.HTTP,1)
		assert.Equal(t, "health-check", cfg.Checks.HTTP[0].Name)
		assert.Equal(t, "GET", cfg.Checks.HTTP[0].Method)
		assert.Equal(t, 30*time.Second, cfg.Checks.HTTP[0].Interval)
		assert.Equal(t, 5*time.Second, cfg.Checks.HTTP[0].Timeout)
	})

	t.Run("MultipleEnvironments", func(t *testing.T) {
		reader := MockFileReader{
			Files: map[string][]byte{
				"config.yaml": []byte(`
environments:
  - name: prod
    vars:
      api_url: https://api.prod.com
  - name: stage
    vars:
      api_url: https://api.stage.com

checks:
  http:
    - name: health-check
      environments: [prod, stage]
      endpoint: https://api.test.com/health
      method: GET
      interval: 10s
      timeout: 5s
`),
			},
		}

		cfg, err := LoadWithReader("config.yaml", reader)

		require.NoError(t, err)
		assert.Len(t, cfg.Environments, 2)
		assert.Equal(t, "prod", cfg.Environments[0].Name)
		assert.Equal(t, "stage", cfg.Environments[1].Name)
	})

	t.Run("FileNotFound", func(t *testing.T) {
		reader := MockFileReader{
			Files: map[string][]byte{},
		}

		_, err := LoadWithReader("nonexistent.yaml", reader)

		assert.Error(t, err)
		assert.Equal(t, os.ErrNotExist, err)
	})

	t.Run("InvalidYAML", func(t *testing.T) {
		reader := MockFileReader{
			Files: map[string][]byte{
				"bad.yaml": []byte(`invalid: yaml: content: [unclosed`),
			},
		}

		_, err := LoadWithReader("bad.yaml", reader)

		assert.Error(t, err)
	})

	t.Run("WithHeaders", func(t *testing.T) {
		reader := MockFileReader{
			Files: map[string][]byte{
				"config.yaml": []byte(`
environments:
  - name: test

checks:
  http:
    - name: api-check
      environments: [test]
      endpoint: https://api.test.com
      method: POST
      interval: 10s
      timeout: 5s
      headers:
        Authorization: Bearer token
        Content-Type: application/json
`),
			},
		}

		cfg, err := LoadWithReader("config.yaml", reader)

		require.NoError(t, err)
		assert.Len(t, cfg.Checks.HTTP,1)
		assert.Len(t, cfg.Checks.HTTP[0].Headers, 2)
		assert.Equal(t, InterpolatedString("Bearer token"), cfg.Checks.HTTP[0].Headers["Authorization"])
		assert.Equal(t, InterpolatedString("application/json"), cfg.Checks.HTTP[0].Headers["Content-Type"])
	})

	t.Run("WithTags", func(t *testing.T) {
		reader := MockFileReader{
			Files: map[string][]byte{
				"config.yaml": []byte(`
environments:
  - name: test

checks:
  http:
    - name: critical-check
      environments: [test]
      endpoint: https://api.test.com
      method: GET
      interval: 10s
      timeout: 5s
      tags: [critical, api, health]
`),
			},
		}

		cfg, err := LoadWithReader("config.yaml", reader)

		require.NoError(t, err)
		assert.Len(t, cfg.Checks.HTTP,1)
		assert.Equal(t, []string{"critical", "api", "health"}, cfg.Checks.HTTP[0].Tags)
	})

	t.Run("ResponseFormats", func(t *testing.T) {
		tests := []struct {
			name           string
			format         string
			expectedFormat ResponseFormat
		}{
			{"JSON format", "json", FormatJSON},
			{"String format", "string", FormatString},
			{"Binary format", "binary", FormatBinary},
			{"None format", "none", FormatNone},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				reader := MockFileReader{
					Files: map[string][]byte{
						"config.yaml": []byte(`
environments:
  - name: test

checks:
  http:
    - name: test-check
      environments: [test]
      endpoint: https://api.test.com
      method: GET
      interval: 10s
      timeout: 5s
      response_format: ` + tt.format + `
`),
					},
				}

				cfg, err := LoadWithReader("config.yaml", reader)

				require.NoError(t, err)
				assert.Equal(t, tt.expectedFormat, cfg.Checks.HTTP[0].ResponseFormat)
			})
		}
	})

	t.Run("WithAssertion", func(t *testing.T) {
		reader := MockFileReader{
			Files: map[string][]byte{
				"config.yaml": []byte(`
environments:
  - name: test

checks:
  http:
    - name: assertion-check
      environments: [test]
      endpoint: https://api.test.com
      method: GET
      interval: 10s
      timeout: 5s
      response_format: json
      assertion: "code == 200 && body.status == 'ok'"
`),
			},
		}

		cfg, err := LoadWithReader("config.yaml", reader)

		require.NoError(t, err)
		assert.Equal(t, "code == 200 && body.status == 'ok'", cfg.Checks.HTTP[0].Assertion)
	})

	t.Run("EmptyConfig", func(t *testing.T) {
		reader := MockFileReader{
			Files: map[string][]byte{
				"empty.yaml": []byte(``),
			},
		}

		cfg, err := LoadWithReader("empty.yaml", reader)

		require.NoError(t, err)
		assert.Len(t, cfg.Environments, 0)
		assert.Len(t, cfg.Checks.HTTP,0)
	})
}

func TestPrometheus_Enabled(t *testing.T) {
	t.Run("True", func(t *testing.T) {
		prom := Prometheus{
			Port: 9090,
			Path: "/metrics",
		}

		assert.True(t, prom.Enabled())
	})

	t.Run("False", func(t *testing.T) {
		prom := Prometheus{
			Port: 0,
			Path: "/metrics",
		}

		assert.False(t, prom.Enabled())
	})
}
