package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	cfg := loadConfig()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	js, nc, err := bootstrapNATS(cfg)
	if err != nil {
		slog.Error("bootstrap nats failed", "error", err)
		return
	}
	defer func() {
		if err := nc.Drain(); err != nil {
			slog.Error("nats drain failed", "error", err)
		}
	}()
	srv, limiter := bootstrapServer(ctx, js, cfg)
	defer limiter.Stop()
	go func() {
		slog.Info("ingest listening", "port", cfg.httpPort, "nats", cfg.natsURL)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http serve failed", "error", err)
			stop()
		}
	}()
	<-ctx.Done()
	slog.Info("shutdown signal received")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown failed", "error", err)
	}
	slog.Info("ingest stopped")
}
