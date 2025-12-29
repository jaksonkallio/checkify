package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/messari/checkify/internal/evaluator"
	"github.com/messari/checkify/internal/metrics"
)

var commands = map[string]func(ctx context.Context) error{
	"evaluator": evaluatorCmd,
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
	default:
		slog.Error("unrecognized command", "command", cmd, "commands", commands)
		os.Exit(1)
	}
}

func evaluatorCmd(ctx context.Context) error {
	config, err := loadConfig("config.yaml")
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	checks, err := convertChecks(config)
	if err != nil {
		return fmt.Errorf("convert checks: %w", err)
	}

	if config.Prometheus.Enabled() {
		// Serve prometheus metrics.
		go metrics.ServePrometheusMetrics(ctx, config.Prometheus.Path, config.Prometheus.Port)
		slog.Info("prometheus metrics server started", "port", config.Prometheus.Port, "path", config.Prometheus.Path)
	}

	evaluator.NewEvaluator(checks).Run(ctx)
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
