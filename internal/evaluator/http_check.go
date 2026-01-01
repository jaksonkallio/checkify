package evaluator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/stretchr/testify/mock"
)

type ResponseFormat string

const (
	FormatJSON   ResponseFormat = "json"
	FormatString ResponseFormat = "string"
	FormatBinary ResponseFormat = "binary"
	FormatNone   ResponseFormat = "none"

	MaxHTTPTimeout = 30 * time.Minute
)

type ErrCheckTimeout struct {
	Timeout time.Duration
}

func NewErrCheckTimeout(timeout time.Duration) *ErrCheckTimeout {
	return &ErrCheckTimeout{Timeout: timeout}
}

func (e *ErrCheckTimeout) Error() string {
	return fmt.Sprintf("check timed out after %s", e.Timeout)
}

type HTTPCheckEvaluator struct {
	Method           string
	Endpoint         string
	Headers          map[string]string
	Timeout          time.Duration
	ResponseFormat   ResponseFormat
	assertionProgram cel.Program
	httpClient       HTTPClient
}

func NewHTTPCheckEvaluator(endpoint string, method string, timeout time.Duration, headers map[string]string, responseFormat ResponseFormat, assertion string) (*HTTPCheckEvaluator, error) {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch, http.MethodOptions:
	default:
		return nil, fmt.Errorf("unsupported HTTP method %q", method)
	}

	if len(endpoint) == 0 {
		return nil, fmt.Errorf("endpoint cannot be empty")
	}

	if timeout <= 0 || timeout > MaxHTTPTimeout {
		return nil, fmt.Errorf("timeout must be greater than 0 and less than %s", MaxHTTPTimeout)
	}

	switch responseFormat {
	case FormatJSON, FormatString, FormatBinary, FormatNone:
	default:
		return nil, fmt.Errorf("unsupported response format %q", responseFormat)
	}

	// Compile CEL assertion program if provided
	var assertionProgram cel.Program
	if assertion != "" {
		env, err := cel.NewEnv(
			cel.Variable("body", cel.DynType),
			cel.Variable("code", cel.IntType),
			cel.Variable("headers", cel.MapType(cel.StringType, cel.StringType)),
		)
		if err != nil {
			return nil, fmt.Errorf("create CEL environment: %w", err)
		}

		ast, issues := env.Compile(assertion)
		if issues != nil && issues.Err() != nil {
			return nil, fmt.Errorf("compile CEL expression: %w", issues.Err())
		}

		prg, err := env.Program(ast)
		if err != nil {
			return nil, fmt.Errorf("create CEL program: %w", err)
		}

		assertionProgram = prg
	}

	return &HTTPCheckEvaluator{
		Endpoint:         endpoint,
		Method:           method,
		Timeout:          timeout,
		Headers:          headers,
		ResponseFormat:   responseFormat,
		assertionProgram: assertionProgram,
		httpClient:       &DefaultHTTPClient{Timeout: timeout},
	}, nil
}

// NewHTTPCheck is a convenience constructor that creates both the evaluator and the Check
func NewHTTPCheck(name string, endpoint string, method string, interval time.Duration, timeout time.Duration, environment string, headers map[string]string, tags []string, responseFormat ResponseFormat, assertion string) (Check, error) {
	evaluator, err := NewHTTPCheckEvaluator(endpoint, method, timeout, headers, responseFormat, assertion)
	if err != nil {
		return Check{}, fmt.Errorf("create HTTP check evaluator: %w", err)
	}

	return NewCheck(name, environment, interval, tags, evaluator)
}

// Evaluate will perform the check and evaluate its assertion, returning an non-pass if the request or assertion failed.
func (c *HTTPCheckEvaluator) Evaluate(ctx context.Context) CheckResult {
	resp, latency, err := c.sendCheckRequest(ctx)
	if err != nil {
		return CheckResult{
			Err:     err,
			Latency: latency,
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return CheckResult{
			Err:     fmt.Errorf("read response body: %w", err),
			Latency: latency,
		}
	}

	// Parse response based on format
	var responseData any
	switch c.ResponseFormat {
	case FormatJSON:
		var jsonData map[string]any
		if err := json.Unmarshal(body, &jsonData); err != nil {
			return CheckResult{
				Err:     fmt.Errorf("parse JSON response: %w", err),
				Latency: latency,
			}
		}
		responseData = jsonData
	case FormatString:
		responseData = string(body)
	case FormatBinary:
		responseData = body
	case FormatNone:
		responseData = nil
	default:
		responseData = string(body)
	}

	var pass bool
	if c.assertionProgram != nil {
		assertionPasses, err := c.evaluateAssertion(responseData, resp)
		if err != nil {
			return CheckResult{
				Err:     fmt.Errorf("evaluate assertion: %w", err),
				Latency: latency,
			}
		}
		pass = assertionPasses
	} else {
		pass = true
	}

	return CheckResult{
		Pass:    pass,
		Latency: latency,
		Err:     nil,
	}
}

func (c *HTTPCheckEvaluator) sendCheckRequest(ctx context.Context) (*http.Response, time.Duration, error) {
	// Create HTTP request with context for cancellation support
	req, err := http.NewRequestWithContext(ctx, c.Method, c.Endpoint, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("create HTTP request: %w", err)
	}

	// Set default User-Agent (user config may override User-Agent)
	req.Header.Set("User-Agent", "Checkify/1.0")

	// Add headers
	for key, value := range c.Headers {
		req.Header.Set(key, value)
	}

	if c.Timeout <= 0 {
		// Negative or zero timeout means guaranteed to timeout, no need to send request.
		return nil, 0, NewErrCheckTimeout(c.Timeout)
	}

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	latency := time.Since(start)
	if err != nil {
		return nil, latency, fmt.Errorf("do HTTP request: %w", err)
	}

	return resp, latency, nil
}

// evaluateAssertion evaluates the CEL assertion against the response using the pre-compiled program
func (c *HTTPCheckEvaluator) evaluateAssertion(responseData interface{}, resp *http.Response) (bool, error) {
	// Convert headers to map[string]string
	headers := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	out, _, err := c.assertionProgram.Eval(map[string]interface{}{
		"body":    responseData,
		"code":    int64(resp.StatusCode),
		"headers": headers,
	})
	if err != nil {
		return false, fmt.Errorf("evaluate CEL expression: %w", err)
	}

	result, ok := out.Value().(bool)
	if !ok {
		return false, fmt.Errorf("CEL expression did not return a boolean value")
	}

	return result, nil
}

type CheckResult struct {
	Pass    bool
	Latency time.Duration
	Err     error
}

// HTTPClient interface allows mocking HTTP requests for testing
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// DefaultHTTPClient wraps http.Client to implement HTTPClient interface
type DefaultHTTPClient struct {
	Timeout time.Duration
}

// Do executes an HTTP request using the configured timeout
func (c *DefaultHTTPClient) Do(req *http.Request) (*http.Response, error) {
	client := &http.Client{
		Timeout: c.Timeout,
	}
	return client.Do(req)
}

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
