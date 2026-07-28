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

// MockHTTPClient is a mock implementation of HTTPClient interface
type MockHTTPClient struct {
	mock.Mock
}

// Do mocks the HTTP request execution
func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

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
		assert.Equal(t, 30*time.Second, check.Interval)
		assert.Equal(t, "prod", check.Environment)
		assert.Len(t, check.Tags, 2)

		httpEval, ok := check.Evaluator.(*HTTPCheckEvaluator)
		require.True(t, ok, "Evaluator should be HTTPCheckEvaluator")
		assert.Equal(t, "https://api.test.com/health", httpEval.Endpoint)
		assert.Equal(t, "GET", httpEval.Method)
		assert.Equal(t, 5*time.Second, httpEval.Timeout)
		assert.Len(t, httpEval.Headers, 1)
		assert.Equal(t, "Bearer token", httpEval.Headers["Authorization"])
		assert.Equal(t, FormatJSON, httpEval.ResponseFormat)
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
		_, ok := check.Evaluator.(*HTTPCheckEvaluator)
		require.True(t, ok, "Evaluator should be HTTPCheckEvaluator")
		// Assertion is now an implementation detail, we can't test it directly
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

			httpEval := check.Evaluator.(*HTTPCheckEvaluator)
			httpEval.httpClient = mockClient

			result := httpEval.Evaluate(context.Background())

			assert.True(t, result.Pass)
			assert.Nil(t, result.Err)
			assert.Greater(t, result.Latency, time.Duration(0))
			mockClient.AssertExpectations(t)
		})

		// A JSON body does not have to be an object. Each of these decodes into a
		// different Go type via `any`, and the assertion exercises it as a CEL value.
		t.Run("JSONNonObject", func(t *testing.T) {
			tests := []struct {
				name      string
				body      string
				assertion string
			}{
				{
					// The shape that motivated this: a top-level array of objects,
					// e.g. Messari's /screener/v1/catalog/asset.
					name:      "ArrayOfObjects",
					body:      `[{"id":"a","n":1},{"id":"b","n":2}]`,
					assertion: `code == 200 && size(body) == 2 && body.all(e, e.id != "")`,
				},
				{
					name:      "ArrayOfScalars",
					body:      `[1, 2, 3]`,
					assertion: `code == 200 && size(body) == 3 && body.all(v, v > 0.0)`,
				},
				{
					// Timeseries-style points: an array of arrays.
					name:      "ArrayOfArrays",
					body:      `[[1730000001, 1.5], [1730000002, 2.5]]`,
					assertion: `code == 200 && size(body) == 2 && body.all(p, size(p) == 2 && p[1] > 1.0)`,
				},
				{
					name:      "EmptyArray",
					body:      `[]`,
					assertion: `code == 200 && size(body) == 0`,
				},
				{
					// Leading whitespace is legal JSON, so format detection must not
					// depend on the first byte of the body.
					name:      "ArrayWithLeadingWhitespace",
					body:      "  \n\t[{\"id\":\"a\"}]",
					assertion: `code == 200 && size(body) == 1`,
				},
				{
					name:      "TopLevelString",
					body:      `"pong"`,
					assertion: `code == 200 && body == "pong"`,
				},
				{
					name:      "TopLevelNumber",
					body:      `42`,
					assertion: `code == 200 && body == 42.0`,
				},
				{
					name:      "TopLevelBool",
					body:      `true`,
					assertion: `code == 200 && body`,
				},
				{
					name:      "TopLevelNull",
					body:      `null`,
					assertion: `code == 200 && body == null`,
				},
				{
					// Objects must keep working exactly as before.
					name:      "Object",
					body:      `{"status":"ok","uptime":12345}`,
					assertion: `code == 200 && body.status == "ok" && body.uptime == 12345.0`,
				},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					mockClient := new(MockHTTPClient)
					mockClient.On("Do", mock.Anything).Return(&http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(strings.NewReader(tt.body)),
						Header:     http.Header{},
					}, nil)

					check, err := NewHTTPCheck(
						"test",
						"https://api.test.com/catalog",
						"GET",
						10*time.Second,
						5*time.Second,
						"test",
						nil,
						nil,
						FormatJSON,
						tt.assertion,
					)
					require.NoError(t, err)
					httpEval := check.Evaluator.(*HTTPCheckEvaluator)
					httpEval.httpClient = mockClient

					result := httpEval.Evaluate(context.Background())

					assert.NoError(t, result.Err)
					assert.True(t, result.Pass)
					mockClient.AssertExpectations(t)
				})
			}
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
			httpEval := check.Evaluator.(*HTTPCheckEvaluator)
			httpEval.httpClient = mockClient

			result := check.Evaluator.Evaluate(context.Background())

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
			httpEval := check.Evaluator.(*HTTPCheckEvaluator)
			httpEval.httpClient = mockClient

			result := check.Evaluator.Evaluate(context.Background())

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
			httpEval := check.Evaluator.(*HTTPCheckEvaluator)
			httpEval.httpClient = mockClient

			result := check.Evaluator.Evaluate(context.Background())

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
		httpEval := check.Evaluator.(*HTTPCheckEvaluator)
		httpEval.httpClient = mockClient

		result := check.Evaluator.Evaluate(context.Background())

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
		httpEval := check.Evaluator.(*HTTPCheckEvaluator)
		httpEval.httpClient = mockClient

		result := check.Evaluator.Evaluate(context.Background())

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
		httpEval := check.Evaluator.(*HTTPCheckEvaluator)
		httpEval.httpClient = mockClient

		result := check.Evaluator.Evaluate(context.Background())

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
			httpEval := check.Evaluator.(*HTTPCheckEvaluator)
			httpEval.httpClient = mockClient

			result := check.Evaluator.Evaluate(context.Background())

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
			httpEval := check.Evaluator.(*HTTPCheckEvaluator)
			httpEval.httpClient = mockClient

			result := check.Evaluator.Evaluate(context.Background())

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
			httpEval := check.Evaluator.(*HTTPCheckEvaluator)
			httpEval.httpClient = mockClient

			result := check.Evaluator.Evaluate(context.Background())

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
		httpEval := check.Evaluator.(*HTTPCheckEvaluator)
		httpEval.httpClient = mockClient

		check.Evaluator.Evaluate(context.Background())

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
		httpEval := check.Evaluator.(*HTTPCheckEvaluator)
		httpEval.httpClient = mockClient

		check.Evaluator.Evaluate(context.Background())

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
		httpEval := check.Evaluator.(*HTTPCheckEvaluator)
		httpEval.httpClient = mockClient

		result := check.Evaluator.Evaluate(context.Background())

		// Should pass when there's no assertion to evaluate
		assert.True(t, result.Pass)
		assert.Nil(t, result.Err)
		mockClient.AssertExpectations(t)
	})
}

func TestCheck_sendCheckRequest(t *testing.T) {
	t.Run("NegativeTimeout", func(t *testing.T) {
		// NewHTTPCheckEvaluator should reject negative timeouts
		_, err := NewHTTPCheckEvaluator(
			"https://api.test.com/health",
			"GET",
			-1*time.Second,
			nil,
			FormatNone,
			"",
		)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "timeout must be greater than 0")
	})

	t.Run("ZeroTimeout", func(t *testing.T) {
		// NewHTTPCheckEvaluator should reject zero timeout
		_, err := NewHTTPCheckEvaluator(
			"https://api.test.com/health",
			"GET",
			0*time.Second,
			nil,
			FormatNone,
			"",
		)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "timeout must be greater than 0")
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

		pass, err := check.Evaluator.(*HTTPCheckEvaluator).evaluateAssertion(map[string]interface{}{}, resp)

		assert.False(t, pass)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "did not return a boolean value")
	})
}

func TestErrCheckTimeout(t *testing.T) {
	t.Run("Error", func(t *testing.T) {
		err := NewErrCheckTimeout(5 * time.Second)

		assert.Equal(t, "check timed out after 5s", err.Error())
	})
}
