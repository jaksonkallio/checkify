package main

import (
	"fmt"
	"os"
)

// EnvProvider interface allows mocking environment variables for testing
type EnvProvider interface {
	Environ() []string
}

// OSEnvProvider provides access to real OS environment variables
type OSEnvProvider struct{}

// Environ returns all OS environment variables
func (o OSEnvProvider) Environ() []string {
	return os.Environ()
}

// MockEnvProvider is a mock implementation of EnvProvider interface
type MockEnvProvider struct {
	Vars map[string]string
}

// Environ returns mocked environment variables
func (m MockEnvProvider) Environ() []string {
	result := make([]string, 0, len(m.Vars))
	for k, v := range m.Vars {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	return result
}
