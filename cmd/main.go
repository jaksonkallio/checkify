package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/messari/checkify/internal/config"
	"github.com/messari/checkify/internal/evaluator"
	"github.com/messari/checkify/internal/metrics"
)

var commands = map[string]func(ctx context.Context) error{
	"evaluator":     evaluatorCmd,
	"evaluate-once": evaluateOnceCmd,
	"validate":      validateCmd,
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevelFromEnv(),
	})))

	ctx, cancel := NewOSTerminatedContext(context.Background())
	defer cancel()

	if len(os.Args) < 2 {
		slog.Error("did not provide a command", "commands", commands)
		os.Exit(1)
	}

	switch cmd := os.Args[1]; cmd {
	case "evaluator":
		if err := evaluatorCmd(ctx); err != nil {
			slog.Error("evaluator failed", "error", err)
			os.Exit(1)
		}
	case "evaluate-once":
		if err := evaluateOnceCmd(ctx); err != nil {
			slog.Error("evaluate-once failed", "error", err)
			os.Exit(1)
		}
	case "validate":
		if err := validateCmd(ctx); err != nil {
			slog.Error("validate failed", "error", err)
			os.Exit(1)
		}
	default:
		slog.Error("unrecognized command", "command", cmd, "commands", commands)
		os.Exit(1)
	}
}

func evaluatorCmd(ctx context.Context) error {
	config, err := config.Load("config.yaml")
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	httpChecks, err := convertHTTPChecks(config, OSEnvProvider{})
	if err != nil {
		return fmt.Errorf("convert HTTP checks: %w", err)
	}

	pingChecks, err := convertPingChecks(config, OSEnvProvider{})
	if err != nil {
		return fmt.Errorf("convert Ping checks: %w", err)
	}

	// Combine all checks into a single slice
	checks := append(httpChecks, pingChecks...)

	if config.Prometheus.Enabled() {
		// Serve prometheus metrics.
		go metrics.ServePrometheusMetrics(ctx, config.Prometheus.Path, config.Prometheus.Port)
		slog.Info("prometheus metrics server started", "port", config.Prometheus.Port, "path", config.Prometheus.Path)
	}

	checkEvaluator, err := evaluator.NewEvaluator(checks, evaluator.RealClock{})
	if err != nil {
		return fmt.Errorf("create evaluator: %w", err)
	}
	checkEvaluator.Run(ctx)
	return nil
}

func evaluateOnceCmd(ctx context.Context) error {
	if len(os.Args) <= 2 {
		return fmt.Errorf("must provide check name as argument")
	}

	config, err := config.Load("config.yaml")
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	httpChecks, err := convertHTTPChecks(config, OSEnvProvider{})
	if err != nil {
		return fmt.Errorf("convert HTTP checks: %w", err)
	}

	pingChecks, err := convertPingChecks(config, OSEnvProvider{})
	if err != nil {
		return fmt.Errorf("convert Ping checks: %w", err)
	}

	// Combine all checks into a single slice
	checks := append(httpChecks, pingChecks...)

	// Note that when evaluating once, the Prometheus metrics server is not started as metrics should not be collected for one-off runes.

	checkEvaluator, err := evaluator.NewEvaluator(checks, evaluator.RealClock{})
	if err != nil {
		return fmt.Errorf("create evaluator: %w", err)
	}
	checkEvaluator.EvaluateChecksByName(ctx, os.Args[2])
	return nil
}

func validateCmd(ctx context.Context) error {
	config, err := config.Load("config.yaml")
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	httpChecks, err := convertHTTPChecks(config, OSEnvProvider{})
	if err != nil {
		return fmt.Errorf("convert HTTP checks: %w", err)
	}

	pingChecks, err := convertPingChecks(config, OSEnvProvider{})
	if err != nil {
		return fmt.Errorf("convert Ping checks: %w", err)
	}

	// Combine all checks into a single slice
	checks := append(httpChecks, pingChecks...)

	if err := evaluator.ValidateCheckSet(checks); err != nil {
		return fmt.Errorf("validate check set: %w", err)
	}

	return nil
}

func logLevelFromEnv() slog.Level {
	levelStr := os.Getenv("LOG_LEVEL")
	switch levelStr {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
