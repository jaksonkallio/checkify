package evaluator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/cel-go/cel"
)

type ResponseFormat string

const (
	FormatJSON   ResponseFormat = "json"
	FormatString ResponseFormat = "string"
	FormatBinary ResponseFormat = "binary"
	FormatNone   ResponseFormat = "none"
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

type Check struct {
	Name           string
	Timeout        time.Duration
	Interval       time.Duration
	Environment    string
	Endpoint       string
	Method         string
	Headers        map[string]string
	Tags           []string
	ResponseFormat ResponseFormat
	Assertion      string
	celProgram     cel.Program
}

func NewCheck(name string, endpoint string, method string, interval time.Duration, timeout time.Duration, environment string, headers map[string]string, tags []string, responseFormat ResponseFormat, assertion string) (*Check, error) {
	check := &Check{
		Name:           name,
		Endpoint:       endpoint,
		Method:         method,
		Interval:       interval,
		Timeout:        timeout,
		Environment:    environment,
		Headers:        headers,
		Tags:           tags,
		ResponseFormat: responseFormat,
		Assertion:      assertion,
	}

	// Compile CEL assertion program if provided
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

		check.celProgram = prg
	}

	return check, nil
}

// Evaluate will perform the check and evaluate its assertion, returning an non-pass if the request or assertion failed.
func (c *Check) Evaluate(ctx context.Context) CheckResult {
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
	if c.Assertion != "" {
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

func (c *Check) sendCheckRequest(ctx context.Context) (*http.Response, time.Duration, error) {
	// Create HTTP request with context for cancellation support
	req, err := http.NewRequestWithContext(ctx, c.Method, c.Endpoint, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("create HTTP request: %w", err)
	}

	// Add headers
	for key, value := range c.Headers {
		req.Header.Set(key, value)
	}

	if c.Timeout <= 0 {
		// Negative or zero timeout means guaranteed to timeout, no need to send request.
		return nil, 0, NewErrCheckTimeout(c.Timeout)
	}

	client := &http.Client{
		Timeout: c.Timeout,
	}

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)
	if err != nil {
		return nil, latency, fmt.Errorf("do HTTP request: %w", err)
	}

	return resp, latency, nil
}

// evaluateAssertion evaluates the CEL assertion against the response using the pre-compiled program
func (c *Check) evaluateAssertion(responseData interface{}, resp *http.Response) (bool, error) {
	// Convert headers to map[string]string
	headers := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	// Evaluate the pre-compiled program with the response data
	out, _, err := c.celProgram.Eval(map[string]interface{}{
		"body":    responseData,
		"code":    int64(resp.StatusCode),
		"headers": headers,
	})
	if err != nil {
		return false, fmt.Errorf("evaluate CEL expression: %w", err)
	}

	// Convert result to boolean
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

func ValidateCheckSet(checks []Check) error {
	// Deduplicate checks by name and environment.
	type checkKey struct {
		Name        string
		Environment string
	}
	checkNames := make(map[checkKey]struct{})
	for _, check := range checks {
		key := checkKey{
			Name:        check.Name,
			Environment: check.Environment,
		}
		if _, exists := checkNames[key]; exists {
			return fmt.Errorf("duplicate check key with name %q in environment %q", check.Name, check.Environment)
		}
		checkNames[key] = struct{}{}
	}
	return nil
}
