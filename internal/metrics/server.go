// Package metrics provides service metrics functionality for checks.
package metrics

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func ServePrometheusMetrics(ctx context.Context, path string, port int) {
	mux := http.NewServeMux()
	mux.Handle(path, promhttp.Handler())
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	err := srv.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		slog.Error("prometheus HTTP server error", "error", err)
	}
}
