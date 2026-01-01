package main

import (
	"fmt"
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
			environmentVarsInterpolator := varInterpolators[envName]

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

			check, err := evaluator.NewHTTPCheck(
				c.Name,
				endpoint,
				c.Method,
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
			environmentVarsInterpolator := varInterpolators[envName]

			interpolatorSet := config.NewInterpolatorSet(envInterpolator, environmentVarsInterpolator)

			host := interpolatorSet.InterpolateMany(c.Host)
			if interpolatorSet.ContainsPlaceholder(host) {
				return nil, fmt.Errorf("host contains uninterpolated placeholder")
			}

			check, err := evaluator.NewPingCheck(
				c.Name,
				host,
				c.Interval,
				c.Count,
				c.Timeout,
				envName,
				c.Tags,
				evaluator.IPStack(strings.ToLower(c.IPStack)),
				c.Assertion,
			)
			if err != nil {
				return nil, fmt.Errorf("create Ping check %s for environment %s: %w", c.Name, envName, err)
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
