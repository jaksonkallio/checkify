package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

// NewOSTerminatedContext creates a context that is canceled when the OS sends some termination signal.
func NewOSTerminatedContext(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
		sig := <-sigChan
		slog.Info("received termination signal", "signal", sig)
		cancel()
	}()
	return ctx, cancel
}
