package evaluator

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewHTTPCheck(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		check, err := NewHTTPCheck(
			"test-check",
			"https://api.test.com/health",
			"GET",
			30*time.Second,
			5*time.Second,
			"prod",
			map[string]string{"Authorization": "Bearer token"},
			[]string{"tag1", "tag2"},
			FormatJSON,
			`code == 200 && body.status == "ok"`,
		)

		require.NoError(t, err)
		assert.Equal(t, "test-check", check.Name)
		assert.Equal(t, "https://api.test.com/health", check.Endpoint)
		assert.Equal(t, "GET", check.Method)
		assert.Equal(t, 30*time.Second, check.Interval)
		assert.Equal(t, 5*time.Second, check.Timeout)
		assert.Equal(t, "prod", check.Environment)
		assert.Len(t, check.Headers, 1)
		assert.Equal(t, "Bearer token", check.Headers["Authorization"])
		assert.Len(t, check.Tags, 2)
		assert.Equal(t, FormatJSON, check.ResponseFormat)
		assert.NotNil(t, check.celProgram)
		assert.NotNil(t, check.httpClient)
	})

	t.Run("EmptyAssertion", func(t *testing.T) {
		check, err := NewHTTPCheck(
			"test-check",
			"https://api.test.com/health",
			"GET",
			30*time.Second,
			5*time.Second,
			"prod",
			nil,
			nil,
			FormatJSON,
			"", // Empty assertion is valid
		)

		require.NoError(t, err)
		assert.Equal(t, "", check.Assertion)
		assert.Nil(t, check.celProgram)
	})

	t.Run("InvalidCEL", func(t *testing.T) {
		_, err := NewHTTPCheck(
			"test-check",
			"https://api.test.com/health",
			"GET",
			30*time.Second,
			5*time.Second,
			"prod",
			nil,
			nil,
			FormatJSON,
			"invalid CEL expression {{",
		)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "compile CEL expression")
	})
}

func TestCheck_Evaluate(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		t.Run("JSON", func(t *testing.T) {
			mockClient := new(MockHTTPClient)
			mockClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
				return req.Method == "GET" && req.URL.String() == "https://api.test.com/health"
			})).Return(&http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(`{"status":"ok","uptime":12345}`)),
				Header:     http.Header{},
			}, nil)

			check, err := NewHTTPCheck(
				"test",
				"https://api.test.com/health",
				"GET",
				10*time.Second,
				5*time.Second,
				"test",
				nil,
				nil,
				FormatJSON,
				`code == 200 && body.status == "ok"`,
			)
			require.NoError(t, err)
			check.httpClient = mockClient

			result := check.Evaluate(context.Background())

			assert.True(t, result.Pass)
			assert.Nil(t, result.Err)
			assert.Greater(t, result.Latency, time.Duration(0))
			mockClient.AssertExpectations(t)
		})

		t.Run("String", func(t *testing.T) {
			mockClient := new(MockHTTPClient)
			mockClient.On("Do", mock.Anything).Return(&http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("pong")),
				Header:     http.Header{},
			}, nil)

			check, err := NewHTTPCheck(
				"test",
				"https://api.test.com/ping",
				"GET",
				10*time.Second,
				5*time.Second,
				"test",
				nil,
				nil,
				FormatString,
				`code == 200 && body == "pong"`,
			)
			require.NoError(t, err)
			check.httpClient = mockClient

			result := check.Evaluate(context.Background())

			assert.True(t, result.Pass)
			assert.Nil(t, result.Err)
			mockClient.AssertExpectations(t)
		})

		t.Run("Binary", func(t *testing.T) {
			mockClient := new(MockHTTPClient)
			mockClient.On("Do", mock.Anything).Return(&http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("binary data")),
				Header:     http.Header{},
			}, nil)

			check, err := NewHTTPCheck(
				"test",
				"https://api.test.com/file",
				"GET",
				10*time.Second,
				5*time.Second,
				"test",
				nil,
				nil,
				FormatBinary,
				`code == 200`,
			)
			require.NoError(t, err)
			check.httpClient = mockClient

			result := check.Evaluate(context.Background())

			assert.True(t, result.Pass)
			assert.Nil(t, result.Err)
			mockClient.AssertExpectations(t)
		})

		t.Run("None", func(t *testing.T) {
			mockClient := new(MockHTTPClient)
			mockClient.On("Do", mock.Anything).Return(&http.Response{
				StatusCode: 204,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{},
			}, nil)

			check, err := NewHTTPCheck(
				"test",
				"https://api.test.com/action",
				"POST",
				10*time.Second,
				5*time.Second,
				"test",
				nil,
				nil,
				FormatNone,
				`code == 204`,
			)
			require.NoError(t, err)
			check.httpClient = mockClient

			result := check.Evaluate(context.Background())

			assert.True(t, result.Pass)
			assert.Nil(t, result.Err)
			mockClient.AssertExpectations(t)
		})
	})

	t.Run("CELFails", func(t *testing.T) {
		mockClient := new(MockHTTPClient)
		mockClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 500, // Assertion expects 200
			Body:       io.NopCloser(strings.NewReader(`{"error":"internal error"}`)),
			Header:     http.Header{},
		}, nil)

		check, err := NewHTTPCheck(
			"test",
			"https://api.test.com/health",
			"GET",
			10*time.Second,
			5*time.Second,
			"test",
			nil,
			nil,
			FormatJSON,
			`code == 200`,
		)
		require.NoError(t, err)
		check.httpClient = mockClient

		result := check.Evaluate(context.Background())

		assert.False(t, result.Pass)
		assert.Nil(t, result.Err) // No error, just assertion failed
		mockClient.AssertExpectations(t)
	})

	t.Run("HTTPError", func(t *testing.T) {
		mockClient := new(MockHTTPClient)
		mockClient.On("Do", mock.Anything).Return(nil, errors.New("connection refused"))

		check, err := NewHTTPCheck(
			"test",
			"https://api.test.com/health",
			"GET",
			10*time.Second,
			5*time.Second,
			"test",
			nil,
			nil,
			FormatJSON,
			`code == 200`,
		)
		require.NoError(t, err)
		check.httpClient = mockClient

		result := check.Evaluate(context.Background())

		assert.False(t, result.Pass)
		assert.Error(t, result.Err)
		assert.Contains(t, result.Err.Error(), "do HTTP request")
		mockClient.AssertExpectations(t)
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		mockClient := new(MockHTTPClient)
		mockClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{invalid json`)),
			Header:     http.Header{},
		}, nil)

		check, err := NewHTTPCheck(
			"test",
			"https://api.test.com/health",
			"GET",
			10*time.Second,
			5*time.Second,
			"test",
			nil,
			nil,
			FormatJSON,
			`code == 200`,
		)
		require.NoError(t, err)
		check.httpClient = mockClient

		result := check.Evaluate(context.Background())

		assert.False(t, result.Pass)
		assert.Error(t, result.Err)
		assert.Contains(t, result.Err.Error(), "parse JSON response")
		mockClient.AssertExpectations(t)
	})

	t.Run("CELVariables", func(t *testing.T) {
		t.Run("Headers", func(t *testing.T) {
			mockClient := new(MockHTTPClient)
			mockClient.On("Do", mock.Anything).Return(&http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
				Header: http.Header{
					"Content-Type": []string{"application/json"},
					"X-Custom":     []string{"test-value"},
				},
			}, nil)

			check, err := NewHTTPCheck(
				"test",
				"https://api.test.com/health",
				"GET",
				10*time.Second,
				5*time.Second,
				"test",
				nil,
				nil,
				FormatJSON,
				`code == 200 && headers["Content-Type"] == "application/json" && headers["X-Custom"] == "test-value"`,
			)
			require.NoError(t, err)
			check.httpClient = mockClient

			result := check.Evaluate(context.Background())

			assert.True(t, result.Pass)
			assert.Nil(t, result.Err)
			mockClient.AssertExpectations(t)
		})

		t.Run("Code", func(t *testing.T) {
			mockClient := new(MockHTTPClient)
			mockClient.On("Do", mock.Anything).Return(&http.Response{
				StatusCode: 201,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
				Header:     http.Header{},
			}, nil)

			check, err := NewHTTPCheck(
				"test",
				"https://api.test.com/create",
				"POST",
				10*time.Second,
				5*time.Second,
				"test",
				nil,
				nil,
				FormatJSON,
				`code >= 200 && code < 300`,
			)
			require.NoError(t, err)
			check.httpClient = mockClient

			result := check.Evaluate(context.Background())

			assert.True(t, result.Pass)
			assert.Nil(t, result.Err)
			mockClient.AssertExpectations(t)
		})

		t.Run("Body", func(t *testing.T) {
			mockClient := new(MockHTTPClient)
			mockClient.On("Do", mock.Anything).Return(&http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(`{"users":[{"name":"Alice"},{"name":"Bob"}],"count":2}`)),
				Header:     http.Header{},
			}, nil)

			check, err := NewHTTPCheck(
				"test",
				"https://api.test.com/users",
				"GET",
				10*time.Second,
				5*time.Second,
				"test",
				nil,
				nil,
				FormatJSON,
				`body.count == 2 && size(body.users) == 2`,
			)
			require.NoError(t, err)
			check.httpClient = mockClient

			result := check.Evaluate(context.Background())

			assert.True(t, result.Pass)
			assert.Nil(t, result.Err)
			mockClient.AssertExpectations(t)
		})
	})

	t.Run("CustomHeaders", func(t *testing.T) {
		var capturedReq *http.Request
		mockClient := new(MockHTTPClient)
		mockClient.On("Do", mock.Anything).Run(func(args mock.Arguments) {
			capturedReq = args.Get(0).(*http.Request)
		}).Return(&http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{}`)),
			Header:     http.Header{},
		}, nil)

		check, err := NewHTTPCheck(
			"test",
			"https://api.test.com/auth",
			"GET",
			10*time.Second,
			5*time.Second,
			"test",
			map[string]string{
				"Authorization": "Bearer secret-token",
				"X-API-Key":     "api-key-123",
			},
			nil,
			FormatJSON,
			"",
		)
		require.NoError(t, err)
		check.httpClient = mockClient

		check.Evaluate(context.Background())

		assert.Equal(t, "Bearer secret-token", capturedReq.Header.Get("Authorization"))
		assert.Equal(t, "api-key-123", capturedReq.Header.Get("X-API-Key"))
		mockClient.AssertExpectations(t)
	})

	t.Run("UserAgent", func(t *testing.T) {
		var capturedReq *http.Request
		mockClient := new(MockHTTPClient)
		mockClient.On("Do", mock.Anything).Run(func(args mock.Arguments) {
			capturedReq = args.Get(0).(*http.Request)
		}).Return(&http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{}`)),
			Header:     http.Header{},
		}, nil)

		check, err := NewHTTPCheck(
			"test",
			"https://api.test.com/health",
			"GET",
			10*time.Second,
			5*time.Second,
			"test",
			nil,
			nil,
			FormatJSON,
			"",
		)
		require.NoError(t, err)
		check.httpClient = mockClient

		check.Evaluate(context.Background())

		assert.Equal(t, "Checkify/1.0", capturedReq.Header.Get("User-Agent"))
		mockClient.AssertExpectations(t)
	})

	t.Run("NoAssertion", func(t *testing.T) {
		mockClient := new(MockHTTPClient)
		mockClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{}`)),
			Header:     http.Header{},
		}, nil)

		check, err := NewHTTPCheck(
			"test",
			"https://api.test.com/health",
			"GET",
			10*time.Second,
			5*time.Second,
			"test",
			nil,
			nil,
			FormatJSON,
			"", // No assertion
		)
		require.NoError(t, err)
		check.httpClient = mockClient

		result := check.Evaluate(context.Background())

		// Should pass when there's no assertion to evaluate
		assert.True(t, result.Pass)
		assert.Nil(t, result.Err)
		mockClient.AssertExpectations(t)
	})
}

func TestCheck_sendCheckRequest(t *testing.T) {
	t.Run("NegativeTimeout", func(t *testing.T) {
		check, err := NewHTTPCheck(
			"test",
			"https://api.test.com/health",
			"GET",
			10*time.Second,
			-1*time.Second,
			"test",
			nil,
			nil,
			FormatNone,
			"",
		)
		require.NoError(t, err)

		resp, latency, err := check.sendCheckRequest(context.Background())

		assert.Nil(t, resp)
		assert.Equal(t, time.Duration(0), latency)
		assert.Error(t, err)
		assert.IsType(t, &ErrCheckTimeout{}, err)
	})

	t.Run("ZeroTimeout", func(t *testing.T) {
		check, err := NewHTTPCheck(
			"test",
			"https://api.test.com/health",
			"GET",
			10*time.Second,
			0*time.Second,
			"test",
			nil,
			nil,
			FormatNone,
			"",
		)
		require.NoError(t, err)

		resp, latency, err := check.sendCheckRequest(context.Background())

		assert.Nil(t, resp)
		assert.Equal(t, time.Duration(0), latency)
		assert.Error(t, err)
		assert.IsType(t, &ErrCheckTimeout{}, err)
	})
}

func TestCheck_evaluateAssertion(t *testing.T) {
	t.Run("NonBooleanResult", func(t *testing.T) {
		check, err := NewHTTPCheck(
			"test",
			"https://api.test.com/health",
			"GET",
			10*time.Second,
			5*time.Second,
			"test",
			nil,
			nil,
			FormatJSON,
			`"not a boolean"`, // CEL expression that returns string, not boolean
		)
		require.NoError(t, err)

		resp := &http.Response{
			StatusCode: 200,
			Header:     http.Header{},
		}

		pass, err := check.evaluateAssertion(map[string]interface{}{}, resp)

		assert.False(t, pass)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "did not return a boolean value")
	})
}

func TestValidateHTTPCheckSet(t *testing.T) {
	t.Run("NoDuplicates", func(t *testing.T) {
		checks := []HTTPCheck{
			{Name: "check1", Environment: "prod"},
			{Name: "check2", Environment: "prod"},
			{Name: "check1", Environment: "stage"}, // Same name, different env - OK
		}

		err := ValidateHTTPCheckSet(checks)

		assert.NoError(t, err)
	})

	t.Run("DuplicateNameAndEnv", func(t *testing.T) {
		checks := []HTTPCheck{
			{Name: "check1", Environment: "prod"},
			{Name: "check1", Environment: "prod"}, // Duplicate
		}

		err := ValidateHTTPCheckSet(checks)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate check key")
		assert.Contains(t, err.Error(), "check1")
		assert.Contains(t, err.Error(), "prod")
	})

	t.Run("SameNameDifferentEnv", func(t *testing.T) {
		checks := []HTTPCheck{
			{Name: "api-health", Environment: "prod"},
			{Name: "api-health", Environment: "stage"},
			{Name: "api-health", Environment: "dev"},
		}

		err := ValidateHTTPCheckSet(checks)

		assert.NoError(t, err)
	})

	t.Run("EmptySet", func(t *testing.T) {
		checks := []HTTPCheck{}

		err := ValidateHTTPCheckSet(checks)

		assert.NoError(t, err)
	})
}

func TestErrCheckTimeout(t *testing.T) {
	t.Run("Error", func(t *testing.T) {
		err := NewErrCheckTimeout(5 * time.Second)

		assert.Equal(t, "check timed out after 5s", err.Error())
	})
}
