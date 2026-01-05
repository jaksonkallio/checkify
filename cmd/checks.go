package main

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/messari/checkify/internal/config"
	"github.com/messari/checkify/internal/evaluator"
)

func convertHTTPChecks(cfg config.Config, envProvider EnvProvider) ([]evaluator.Check, error) {
	envInterpolator, varInterpolators, err := setupInterpolators(cfg, envProvider)
	if err != nil {
		return nil, err
	}

	var checks []evaluator.Check
	for _, c := range cfg.Checks.HTTP {
		for _, envName := range c.Environments {
			environmentVarsInterpolator, ok := varInterpolators[envName]
			if !ok {
				return nil, fmt.Errorf("no interpolator found for environment %q", envName)
			}

			interpolatorSet := config.NewInterpolatorSet(envInterpolator, environmentVarsInterpolator)

			headers := make(map[string]string)
			for k, v := range c.Headers {
				headers[k] = interpolatorSet.InterpolateMany(v)
				if interpolatorSet.ContainsPlaceholder(headers[k]) {
					return nil, fmt.Errorf("header %q contains uninterpolated placeholder", k)
				}
			}

			endpoint := interpolatorSet.InterpolateMany(c.Endpoint)
			if interpolatorSet.ContainsPlaceholder(endpoint) {
				return nil, fmt.Errorf("endpoint contains uninterpolated placeholder")
			}

			var method string
			if len(c.Method) == 0 {
				method = http.MethodGet
			} else {
				method = c.Method
			}

			if c.Timeout == 0 {
				c.Timeout = evaluator.DefaultHTTPTimeout
			}

			check, err := evaluator.NewHTTPCheck(
				c.Name,
				endpoint,
				method,
				c.Interval,
				c.Timeout,
				envName,
				headers,
				c.Tags,
				convertResponseFormat(c.ResponseFormat),
				c.Assertion,
			)
			if err != nil {
				return nil, fmt.Errorf("create HTTP check %s for environment %s: %w", c.Name, envName, err)
			}

			checks = append(checks, check)
		}
	}
	return checks, nil
}

func convertPingChecks(cfg config.Config, envProvider EnvProvider) ([]evaluator.Check, error) {
	envInterpolator, varInterpolators, err := setupInterpolators(cfg, envProvider)
	if err != nil {
		return nil, err
	}

	var checks []evaluator.Check
	for _, c := range cfg.Checks.Ping {
		for _, envName := range c.Environments {
			environmentVarsInterpolator, ok := varInterpolators[envName]
			if !ok {
				return nil, fmt.Errorf("no interpolator found for environment %q", envName)
			}

			interpolatorSet := config.NewInterpolatorSet(envInterpolator, environmentVarsInterpolator)

			host := interpolatorSet.InterpolateMany(c.Host)
			if interpolatorSet.ContainsPlaceholder(host) {
				return nil, fmt.Errorf("host contains uninterpolated placeholder")
			}

			if c.Timeout == 0 {
				c.Timeout = evaluator.DefaultPingTimeout
			}

			if c.Count == 0 {
				c.Count = 1
			}

			var ipStack evaluator.IPStack
			if len(c.IPStack) == 0 {
				ipStack = evaluator.IPStackDefault
			} else {
				ipStack = evaluator.IPStack(strings.ToLower(c.IPStack))
			}

			check, err := evaluator.NewPingCheck(
				c.Name,
				host,
				c.Interval,
				c.Count,
				c.Timeout,
				envName,
				c.Tags,
				ipStack,
				c.Assertion,
			)
			if err != nil {
				return nil, fmt.Errorf("create ping check %s for environment %s: %w", c.Name, envName, err)
			}

			checks = append(checks, check)
		}
	}
	return checks, nil
}

func convertTCPChecks(cfg config.Config, envProvider EnvProvider) ([]evaluator.Check, error) {
	envInterpolator, varInterpolators, err := setupInterpolators(cfg, envProvider)
	if err != nil {
		return nil, err
	}

	var checks []evaluator.Check
	for _, c := range cfg.Checks.TCP {
		for _, envName := range c.Environments {
			environmentVarsInterpolator, ok := varInterpolators[envName]
			if !ok {
				return nil, fmt.Errorf("no interpolator found for environment %q", envName)
			}

			interpolatorSet := config.NewInterpolatorSet(envInterpolator, environmentVarsInterpolator)

			host := interpolatorSet.InterpolateMany(c.Host)
			if interpolatorSet.ContainsPlaceholder(host) {
				return nil, fmt.Errorf("host contains uninterpolated placeholder")
			}

			timeout := c.Timeout
			if timeout == 0 {
				timeout = evaluator.DefaultTCPTimeout
			}

			var ipStack evaluator.IPStack
			if len(c.IPStack) == 0 {
				ipStack = evaluator.IPStackDefault
			} else {
				ipStack = evaluator.IPStack(strings.ToLower(c.IPStack))
			}

			check, err := evaluator.NewTCPCheck(
				c.Name,
				host,
				c.Port,
				c.Interval,
				timeout,
				ipStack,
				envName,
				c.Tags,
			)
			if err != nil {
				return nil, fmt.Errorf("create TCP check %s for environment %s: %w", c.Name, envName, err)
			}

			checks = append(checks, check)
		}
	}
	return checks, nil
}

func convertDNSChecks(cfg config.Config, envProvider EnvProvider) ([]evaluator.Check, error) {
	envInterpolator, varInterpolators, err := setupInterpolators(cfg, envProvider)
	if err != nil {
		return nil, err
	}

	var checks []evaluator.Check
	for _, c := range cfg.Checks.DNS {
		for _, envName := range c.Environments {
			environmentVarsInterpolator, ok := varInterpolators[envName]
			if !ok {
				return nil, fmt.Errorf("no interpolator found for environment %q", envName)
			}

			interpolatorSet := config.NewInterpolatorSet(envInterpolator, environmentVarsInterpolator)

			query := interpolatorSet.InterpolateMany(c.Query)
			if interpolatorSet.ContainsPlaceholder(query) {
				return nil, fmt.Errorf("query contains uninterpolated placeholder")
			}

			host := interpolatorSet.InterpolateMany(c.Host)
			if interpolatorSet.ContainsPlaceholder(host) {
				return nil, fmt.Errorf("host contains uninterpolated placeholder")
			}

			timeout := c.Timeout
			if timeout == 0 {
				timeout = evaluator.DefaultDNSTimeout
			}

			port := c.Port
			if port == 0 {
				port = evaluator.DefaultDNSPort
			}

			var ipStack evaluator.IPStack
			if len(c.IPStack) == 0 {
				ipStack = evaluator.IPStackDefault
			} else {
				ipStack = evaluator.IPStack(strings.ToLower(c.IPStack))
			}

			recordType := evaluator.DNSRecordType(strings.ToUpper(c.RecordType))

			check, err := evaluator.NewDNSCheck(
				c.Name,
				query,
				recordType,
				host,
				port,
				c.Interval,
				timeout,
				ipStack,
				envName,
				c.Tags,
				c.Assertion,
			)
			if err != nil {
				return nil, fmt.Errorf("create DNS check %s for environment %s: %w", c.Name, envName, err)
			}

			checks = append(checks, check)
		}
	}
	return checks, nil
}

func setupInterpolators(cfg config.Config, envProvider EnvProvider) (*config.Interpolator, map[string]*config.Interpolator, error) {
	// Set the env interpolator with all env vars.
	envInterpolator := config.NewInterpolator("env")
	for _, value := range envProvider.Environ() {
		parts := strings.SplitN(value, "=", 2)
		envInterpolator.Set(parts[0], parts[1])
	}

	// Set up individual env interpolators for each configured environment.
	varInterpolators := make(map[string]*config.Interpolator)
	for _, environment := range cfg.Environments {
		interpolator := config.NewInterpolator("var")
		for k, v := range environment.Vars {
			// Set each var interpolated value using the env interpolator.
			value := envInterpolator.Interpolate(v)
			if envInterpolator.ContainsPlaceholder(value) {
				return nil, nil, fmt.Errorf("environment %q var %q contains uninterpolated placeholder", environment.Name, k)
			}
			interpolator.Set(k, value)
		}
		varInterpolators[environment.Name] = interpolator
	}

	return envInterpolator, varInterpolators, nil
}

func convertResponseFormat(format config.ResponseFormat) evaluator.ResponseFormat {
	switch format {
	case config.FormatJSON:
		return evaluator.FormatJSON
	case config.FormatString:
		return evaluator.FormatString
	case config.FormatBinary:
		return evaluator.FormatBinary
	case config.FormatNone:
		return evaluator.FormatNone
	default:
		return evaluator.FormatNone
	}
}
