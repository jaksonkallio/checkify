package evaluator

import (
	"net/http"
	"time"

	"github.com/stretchr/testify/mock"
)

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
